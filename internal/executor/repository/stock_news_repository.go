package repository

import (
	"context"
	"fmt"
	"log"
	"strings"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockNewsRepository defines the interface for interacting with stock news data.
type StockNewsRepository interface {
	Create(ctx context.Context, stockNews *entity.StockNews) error
	CreateIgnoreConflict(ctx context.Context, stockNews *entity.StockNews) error
	FindRankedNews(ctx context.Context, stockCode string, maxNews int, maxNewsAgeInDays int, priorityDomains []string) ([]entity.StockNews, error)
	GetStocksToSummarize(ctx context.Context, maxNewsAgeInDays int, minimumNews int, minimumConfidenceScore float64) ([]string, error)
}

// NewStockNewsRepository creates a new instance of StockNewsRepository.
func NewStockNewsRepository(db *gorm.DB) StockNewsRepository {
	return &stockNewsRepository{
		db: db,
	}
}

type stockNewsRepository struct {
	db *gorm.DB
}

// Create saves a new stock news article and its associated stock mentions to the database.
func (r *stockNewsRepository) Create(ctx context.Context, stockNews *entity.StockNews) error {
	return r.db.WithContext(ctx).Create(stockNews).Error
}

func (r *stockNewsRepository) FindRankedNews(ctx context.Context, stockCode string, maxNews int, maxNewsAgeInDays int, priorityDomains []string) ([]entity.StockNews, error) {
	var (
		qBuilder strings.Builder
		qParam   = []interface{}{}
	)

	qBuilder.WriteString(fmt.Sprintf(`
	SELECT
		sn.id,
		sn.title,
		sn.link,
		sn.published_at,
		sn.raw_content,
		sn.summary,
		sn.hash_identifier,
		sn.source,
		sn.google_rss_link,
		sn.impact_score,
		sn.key_issue,
		sn.created_at,
		sn.updated_at,
		sm.reason,
		sm.stock_code,
		sm.sentiment,
		sm.impact,
		sm.confidence_score,
		(0.5 * sm.confidence_score + 0.3 * sn.impact_score + 0.2 * GREATEST(0, 1 - (EXTRACT(EPOCH FROM (NOW() - sn.published_at)) / 86400) / %d)) AS final_score
	FROM stock_news AS sn
	JOIN stock_mentions AS sm ON sm.stock_news_id = sn.id
	WHERE sm.stock_code = ?
	AND sn.published_at >= NOW() - INTERVAL '%d days'
`, maxNewsAgeInDays, maxNewsAgeInDays))

	qParam = append(qParam, stockCode)
	if len(priorityDomains) > 0 {
		qBuilder.WriteString(" ORDER BY CASE WHEN sn.source IN ? THEN 0 ELSE 1 END, final_score DESC")
		qParam = append(qParam, priorityDomains)
	} else {
		qBuilder.WriteString(" ORDER BY final_score DESC")
	}

	qBuilder.WriteString(" LIMIT ?")
	qParam = append(qParam, maxNews)

	type scanResult struct {
		entity.StockNews
		StockCode       string  `gorm:"column:stock_code"`
		Sentiment       string  `gorm:"column:sentiment"`
		Impact          string  `gorm:"column:impact"`
		ConfidenceScore float64 `gorm:"column:confidence_score"`
		FinalScore      float64 `gorm:"column:final_score"`
		Reason          string  `gorm:"column:reason"`
	}

	var results []scanResult
	err := r.db.Debug().WithContext(ctx).Raw(qBuilder.String(), qParam...).Scan(&results).Error
	if err != nil {
		log.Fatal("Query error: ", err)
	}

	news := make([]entity.StockNews, len(results))
	for i, r := range results {
		news[i] = r.StockNews
		news[i].StockCode = r.StockCode
		news[i].Sentiment = r.Sentiment
		news[i].Impact = r.Impact
		news[i].ConfidenceScore = r.ConfidenceScore
		news[i].FinalScore = r.FinalScore
		news[i].Reason = r.Reason
	}

	return news, nil
}

func (r *stockNewsRepository) CreateIgnoreConflict(ctx context.Context, stockNews *entity.StockNews) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		stockMentions := stockNews.StockMentions
		stockNews.StockMentions = nil
		txInner := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "hash_identifier"}}, // kolom unik
			DoNothing: true,
		}).Create(stockNews)

		if txInner.Error != nil {
			return txInner.Error
		}

		if txInner.RowsAffected == 0 {
			return nil
		}

		if len(stockMentions) == 0 {
			return nil
		}
		for i := range stockMentions {
			stockMentions[i].StockNewsID = stockNews.ID
		}

		if err := tx.Create(&stockMentions).Error; err != nil {
			return fmt.Errorf("insert stock_mentions error: %w", err)
		}

		return nil
	})

}

func (r *stockNewsRepository) GetStocksToSummarize(ctx context.Context, maxNewsAgeInDays int, minimumNews int, minimumConfidenceScore float64) ([]string, error) {
	var stocks []string
	err := r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT sm.stock_code
		FROM stock_mentions sm
		JOIN stock_news sn ON sm.stock_news_id = sn.id
		WHERE sn.published_at >= NOW() - INTERVAL '%d days'
		GROUP BY sm.stock_code
		HAVING COUNT(*) >= %d 
		   AND MAX(sm.confidence_score) > %f
	`, maxNewsAgeInDays, minimumNews, minimumConfidenceScore)).Scan(&stocks).Error
	if err != nil {
		return nil, err
	}
	return stocks, nil
}

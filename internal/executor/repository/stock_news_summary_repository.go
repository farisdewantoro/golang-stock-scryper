package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"

	"gorm.io/gorm"
)

// StockNewsSummaryRepository defines the interface for interacting with stock news summary data.
type StockNewsSummaryRepository interface {
	Create(ctx context.Context, summary *entity.StockNewsSummary) error
	GetLast(ctx context.Context, before time.Time, stockCode string) (*entity.StockNewsSummary, error)
	Get(ctx context.Context, param *dto.GetStockSummaryParam) ([]entity.StockNewsSummary, error)
}

// NewStockNewsSummaryRepository creates a new instance of StockNewsSummaryRepository.
func NewStockNewsSummaryRepository(db *gorm.DB) StockNewsSummaryRepository {
	return &stockNewsSummaryRepository{
		db: db,
	}
}

type stockNewsSummaryRepository struct {
	db *gorm.DB
}

// Create saves a new stock news summary to the database.
func (r *stockNewsSummaryRepository) Create(ctx context.Context, summary *entity.StockNewsSummary) error {
	return r.db.WithContext(ctx).Create(summary).Error
}

func (r *stockNewsSummaryRepository) GetLast(ctx context.Context, before time.Time, stockCode string) (*entity.StockNewsSummary, error) {
	var summary entity.StockNewsSummary
	result := r.db.WithContext(ctx).Where("created_at >= ? AND stock_code = ?", before, stockCode).Order("created_at desc").First(&summary)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, result.Error
	}
	return &summary, nil
}

func (r *stockNewsSummaryRepository) Get(ctx context.Context, param *dto.GetStockSummaryParam) ([]entity.StockNewsSummary, error) {
	var summary []entity.StockNewsSummary

	qFilter := []string{}
	qParam := []interface{}{}

	if param.HashIdentifier != "" {
		qFilter = append(qFilter, "hash_identifier = ?")
		qParam = append(qParam, param.HashIdentifier)
	}

	if len(qFilter) == 0 {
		return nil, fmt.Errorf("no filter provided")
	}

	if err := r.db.WithContext(ctx).Where(strings.Join(qFilter, " AND "), qParam...).Find(&summary).Error; err != nil {
		return nil, err
	}

	return summary, nil
}

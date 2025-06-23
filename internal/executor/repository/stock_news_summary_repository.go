package repository

import (
	"context"
	"time"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

// StockNewsSummaryRepository defines the interface for interacting with stock news summary data.
type StockNewsSummaryRepository interface {
	Create(ctx context.Context, summary *entity.StockNewsSummary) error
	GetLast(ctx context.Context, before time.Time, stockCode string) (*entity.StockNewsSummary, error)
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

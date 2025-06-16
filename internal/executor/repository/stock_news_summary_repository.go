package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

// StockNewsSummaryRepository defines the interface for interacting with stock news summary data.
type StockNewsSummaryRepository interface {
	Create(ctx context.Context, summary *entity.StockNewsSummary) error
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

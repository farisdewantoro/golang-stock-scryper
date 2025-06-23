package repository

import (
	"context"
	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

type StockSignalRepository interface {
	Create(ctx context.Context, stockSignal *entity.StockSignal) error
}

type stockSignalRepository struct {
	db *gorm.DB
}

func NewStockSignalRepository(db *gorm.DB) StockSignalRepository {
	return &stockSignalRepository{db: db}
}

func (s *stockSignalRepository) Create(ctx context.Context, stockSignal *entity.StockSignal) error {
	return s.db.Create(stockSignal).Error
}

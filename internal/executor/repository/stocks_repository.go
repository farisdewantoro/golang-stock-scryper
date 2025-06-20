package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

type StocksRepository interface {
	GetStocks(ctx context.Context) ([]entity.Stock, error)
}

type stocksRepository struct {
	db *gorm.DB
}

func NewStocksRepository(db *gorm.DB) StocksRepository {
	return &stocksRepository{db: db}
}

func (s *stocksRepository) GetStocks(ctx context.Context) ([]entity.Stock, error) {
	var stocks []entity.Stock
	if err := s.db.WithContext(ctx).Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

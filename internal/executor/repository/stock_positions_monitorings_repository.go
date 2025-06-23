package repository

import (
	"context"
	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

type StockPositionsMonitoringsRepository interface {
	Create(ctx context.Context, stockPositionMonitoring *entity.StockPositionMonitoring) error
}

type stockPositionsMonitoringsRepository struct {
	db *gorm.DB
}

func NewStockPositionsMonitoringsRepository(db *gorm.DB) StockPositionsMonitoringsRepository {
	return &stockPositionsMonitoringsRepository{db: db}
}

func (s *stockPositionsMonitoringsRepository) Create(ctx context.Context, stockPositionMonitoring *entity.StockPositionMonitoring) error {
	return s.db.Create(stockPositionMonitoring).Error
}

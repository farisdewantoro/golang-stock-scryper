package repository

import (
	"context"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"strings"

	"gorm.io/gorm"
)

type StockPositionsRepository interface {
	Get(ctx context.Context, param dto.GetStockPositionsParam) ([]entity.StockPosition, error)
	Update(ctx context.Context, stockPosition entity.StockPosition) error
}

type stockPositionsRepository struct {
	db *gorm.DB
}

func NewStockPositionsRepository(db *gorm.DB) StockPositionsRepository {
	return &stockPositionsRepository{
		db: db,
	}
}

func (r *stockPositionsRepository) Get(ctx context.Context, param dto.GetStockPositionsParam) ([]entity.StockPosition, error) {
	var stockPositions []entity.StockPosition

	qFilter := []string{}
	qFilterParam := []interface{}{}
	if param.IsAlertTriggered != nil {
		qFilter = append(qFilter, "is_alert_triggered = ?")
		qFilterParam = append(qFilterParam, *param.IsAlertTriggered)
	}

	if len(qFilter) == 0 {
		return nil, fmt.Errorf("no filter provided")
	}

	if len(param.StockCodes) > 0 {
		qFilter = append(qFilter, "stock_code IN (?)")
		qFilterParam = append(qFilterParam, param.StockCodes)
	}

	if err := r.db.WithContext(ctx).Where(strings.Join(qFilter, " AND "), qFilterParam...).Find(&stockPositions).Error; err != nil {
		return nil, err
	}

	return stockPositions, nil
}

func (r *stockPositionsRepository) Update(ctx context.Context, stockPosition entity.StockPosition) error {
	return r.db.WithContext(ctx).Updates(&stockPosition).Error
}

package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/pkg/logger"

	"gorm.io/gorm"
)

// StockMentionRepository defines the interface for interacting with stock mention data.
type StockMentionRepository interface {
	SaveAll(ctx context.Context, mentions []entity.StockMention) error
}

type stockMentionRepository struct {
	DB     *gorm.DB
	logger *logger.Logger
}

// NewStockMentionRepository creates a new instance of StockMentionRepository.
func NewStockMentionRepository(db *gorm.DB, logger *logger.Logger) StockMentionRepository {
	return &stockMentionRepository{
		DB:     db,
		logger: logger,
	}
}

// SaveAll saves multiple stock mention records to the database.
func (r *stockMentionRepository) SaveAll(ctx context.Context, mentions []entity.StockMention) error {
	if len(mentions) == 0 {
		return nil
	}

	if err := r.DB.WithContext(ctx).Create(&mentions).Error; err != nil {
		r.logger.Error("Failed to save stock mentions", logger.ErrorField(err))
		return err
	}

	r.logger.Info("Successfully saved stock mentions", logger.IntField("count", len(mentions)))
	return nil
}

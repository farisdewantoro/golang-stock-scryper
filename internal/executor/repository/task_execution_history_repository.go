package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

// TaskExecutionHistoryRepository defines the interface for task execution history data operations.
type TaskExecutionHistoryRepository interface {
	FindByID(ctx context.Context, id uint) (*entity.TaskExecutionHistory, error)
	Update(ctx context.Context, history *entity.TaskExecutionHistory) error
}

// NewTaskExecutionHistoryRepository creates a new GORM-based task execution history repository.
func NewTaskExecutionHistoryRepository(db *gorm.DB) TaskExecutionHistoryRepository {
	return &taskExecutionHistoryRepository{db: db}
}

type taskExecutionHistoryRepository struct {
	db *gorm.DB
}

// FindByID retrieves a task execution history by its ID.
func (r *taskExecutionHistoryRepository) FindByID(ctx context.Context, id uint) (*entity.TaskExecutionHistory, error) {
	var history entity.TaskExecutionHistory
	if err := r.db.WithContext(ctx).First(&history, id).Error; err != nil {
		return nil, err
	}
	return &history, nil
}

// Update updates an existing task execution history record.
func (r *taskExecutionHistoryRepository) Update(ctx context.Context, history *entity.TaskExecutionHistory) error {
	return r.db.WithContext(ctx).Save(history).Error
}

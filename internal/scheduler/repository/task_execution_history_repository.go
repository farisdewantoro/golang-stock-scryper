package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

// TaskExecutionHistoryRepository defines the interface for task execution history data operations.
type TaskExecutionHistoryRepository interface {
	Create(ctx context.Context, history *entity.TaskExecutionHistory) error
	FindByID(ctx context.Context, id uint) (*entity.TaskExecutionHistory, error)
	FindAll(ctx context.Context) ([]entity.TaskExecutionHistory, error)
	FindAllByJobID(ctx context.Context, jobID uint) ([]entity.TaskExecutionHistory, error)
	Update(ctx context.Context, history *entity.TaskExecutionHistory) error
}

// NewTaskExecutionHistoryRepository creates a new GORM-based task execution history repository.
func NewTaskExecutionHistoryRepository(db *gorm.DB) TaskExecutionHistoryRepository {
	return &taskExecutionHistoryRepository{db: db}
}

type taskExecutionHistoryRepository struct {
	db *gorm.DB
}

// Create creates a new task execution history record.
func (r *taskExecutionHistoryRepository) Create(ctx context.Context, history *entity.TaskExecutionHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

// FindByID retrieves a task execution history record by its ID.
func (r *taskExecutionHistoryRepository) FindByID(ctx context.Context, id uint) (*entity.TaskExecutionHistory, error) {
	var history entity.TaskExecutionHistory
	if err := r.db.WithContext(ctx).First(&history, id).Error; err != nil {
		return nil, err
	}
	return &history, nil
}

// FindAll retrieves all task execution history records.
func (r *taskExecutionHistoryRepository) FindAll(ctx context.Context) ([]entity.TaskExecutionHistory, error) {
	var histories []entity.TaskExecutionHistory
	if err := r.db.WithContext(ctx).Order("executed_at desc").Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// FindAllByJobID retrieves all task execution history records for a specific job.
func (r *taskExecutionHistoryRepository) FindAllByJobID(ctx context.Context, jobID uint) ([]entity.TaskExecutionHistory, error) {
	var histories []entity.TaskExecutionHistory
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Order("executed_at desc").Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// Update update task execution history record
func (r *taskExecutionHistoryRepository) Update(ctx context.Context, history *entity.TaskExecutionHistory) error {
	return r.db.WithContext(ctx).Updates(history).Error
}

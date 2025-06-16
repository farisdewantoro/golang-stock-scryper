package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

// TaskScheduleRepository defines the interface for task schedule data operations.
type TaskScheduleRepository interface {
	Create(ctx context.Context, schedule *entity.TaskSchedule) error
	FindByID(ctx context.Context, id uint) (*entity.TaskSchedule, error)
	FindAll(ctx context.Context) ([]entity.TaskSchedule, error)
	Update(ctx context.Context, schedule *entity.TaskSchedule) error
	Delete(ctx context.Context, id uint) error
}

// NewTaskScheduleRepository creates a new GORM-based task schedule repository.
func NewTaskScheduleRepository(db *gorm.DB) TaskScheduleRepository {
	return &taskScheduleRepository{db: db}
}

type taskScheduleRepository struct {
	db *gorm.DB
}

// Create creates a new task schedule.
func (r *taskScheduleRepository) Create(ctx context.Context, schedule *entity.TaskSchedule) error {
	return r.db.WithContext(ctx).Create(schedule).Error
}

// FindByID retrieves a task schedule by its ID.
func (r *taskScheduleRepository) FindByID(ctx context.Context, id uint) (*entity.TaskSchedule, error) {
	var schedule entity.TaskSchedule
	if err := r.db.WithContext(ctx).First(&schedule, id).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

// FindAll retrieves all task schedules.
func (r *taskScheduleRepository) FindAll(ctx context.Context) ([]entity.TaskSchedule, error) {
	var schedules []entity.TaskSchedule
	if err := r.db.WithContext(ctx).Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

// Update updates a task schedule.
func (r *taskScheduleRepository) Update(ctx context.Context, schedule *entity.TaskSchedule) error {
	return r.db.WithContext(ctx).Save(schedule).Error
}

// Delete removes a task schedule by its ID.
func (r *taskScheduleRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.TaskSchedule{}, id).Error
}

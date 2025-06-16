package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

// JobRepository defines the interface for job data operations.
type JobRepository interface {
	FindByID(ctx context.Context, id uint) (*entity.Job, error)
}

// NewJobRepository creates a new GORM-based job repository.
func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

type jobRepository struct {
	db *gorm.DB
}

// FindByID retrieves a job by its ID.
func (r *jobRepository) FindByID(ctx context.Context, id uint) (*entity.Job, error) {
	var job entity.Job
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

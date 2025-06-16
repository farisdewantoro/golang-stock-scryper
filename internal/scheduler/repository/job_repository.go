package repository

import (
	"context"
	"time"

	"golang-stock-scryper/internal/entity"

	"gorm.io/gorm"
)

// JobRepository defines the interface for job data operations.
type JobRepository interface {
	Create(ctx context.Context, job *entity.Job) error
	FindByID(ctx context.Context, id uint) (*entity.Job, error)
	FindAll(ctx context.Context) ([]entity.Job, error)
	Update(ctx context.Context, job *entity.Job) error
	FindJobsToSchedule(ctx context.Context) ([]entity.Job, error)
	Delete(ctx context.Context, id uint) error
}

// NewJobRepository creates a new GORM-based job repository.
func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

type jobRepository struct {
	db *gorm.DB
}

// Create creates a new job in the database.
func (r *jobRepository) Create(ctx context.Context, job *entity.Job) error {
	return r.db.WithContext(ctx).Create(job).Error
}

// FindByID retrieves a job by its ID.
func (r *jobRepository) FindByID(ctx context.Context, id uint) (*entity.Job, error) {
	var job entity.Job
	if err := r.db.WithContext(ctx).Preload("Schedules").First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// FindAll retrieves all jobs.
func (r *jobRepository) FindAll(ctx context.Context) ([]entity.Job, error) {
	var jobs []entity.Job
	if err := r.db.WithContext(ctx).Preload("Schedules").Find(&jobs).Error; err != nil {
		return nil, err
	}
	return jobs, nil
}

// Update updates an existing job and its associated schedules within a transaction.
func (r *jobRepository) Update(ctx context.Context, job *entity.Job) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First, remove all existing schedules for this job to ensure a clean update.
		if err := tx.Where("job_id = ?", job.ID).Delete(&entity.TaskSchedule{}).Error; err != nil {
			return err
		}

		// Now, save the job. GORM's Save method will update the job record
		// and create the new schedule records from the job.Schedules slice.
		return tx.Save(job).Error
	})
}

// FindJobsToSchedule finds all active jobs with schedules that need to be run.
func (r *jobRepository) FindJobsToSchedule(ctx context.Context) ([]entity.Job, error) {
	var jobs []entity.Job
	now := time.Now()
	// Find jobs with active schedules that are due
	err := r.db.WithContext(ctx).
		Preload("Schedules", "is_active = ?", true).
		Joins("JOIN task_schedules ts ON ts.job_id = jobs.id").
		Where("ts.is_active = ? AND (ts.next_execution IS NULL OR ts.next_execution <= ?)", true, now).
		Group("jobs.id").
		Find(&jobs).Error
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// Delete removes a job and its associated schedules and history.
func (r *jobRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", id).Delete(&entity.TaskExecutionHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("job_id = ?", id).Delete(&entity.TaskSchedule{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&entity.Job{}, id).Error; err != nil {
			return err
		}
		return nil
	})
}

package service

import (
	"context"
	"encoding/json"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/scheduler/dto"
	"golang-stock-scryper/internal/scheduler/repository"
	"golang-stock-scryper/pkg/logger"

	"gorm.io/datatypes"
)

// JobService defines the interface for managing jobs.
type JobService interface {
	CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*dto.JobResponse, error)
	GetJobByID(ctx context.Context, id uint) (*dto.JobResponse, error)
	GetAllJobs(ctx context.Context) ([]*dto.JobResponse, error)
	UpdateJob(ctx context.Context, id uint, req *dto.UpdateJobRequest) (*dto.JobResponse, error)
	DeleteJob(ctx context.Context, id uint) error
}

// NewJobService creates a new job service.
func NewJobService(jobRepo repository.JobRepository, logger *logger.Logger) JobService {
	return &jobService{
		jobRepo: jobRepo,
		logger:  logger,
	}
}

type jobService struct {
	jobRepo repository.JobRepository
	logger  *logger.Logger
}

// CreateJob handles the business logic for creating a new job.
func (s *jobService) CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*dto.JobResponse, error) {
	retryPolicyBytes, err := json.Marshal(req.RetryPolicy)
	if err != nil {
		return nil, err
	}

	job := &entity.Job{
		Name:        req.Name,
		Description: req.Description,
		Type:        entity.JobType(req.Type),
		Payload:     datatypes.JSON(req.Payload),
		RetryPolicy: datatypes.JSON(retryPolicyBytes),
		Timeout:     req.Timeout,
	}

	for _, sDto := range req.Schedules {
		job.Schedules = append(job.Schedules, entity.TaskSchedule{
			CronExpression: sDto.CronExpression,
			IsActive:       sDto.IsActive,
		})
	}

	if err := s.jobRepo.Create(ctx, job); err != nil {
		return nil, err
	}

	return s.mapToJobResponse(job), nil
}

// GetJobByID retrieves a job by its ID.
func (s *jobService) GetJobByID(ctx context.Context, id uint) (*dto.JobResponse, error) {
	job, err := s.jobRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.mapToJobResponse(job), nil
}

// GetAllJobs retrieves all jobs.
func (s *jobService) GetAllJobs(ctx context.Context) ([]*dto.JobResponse, error) {
	jobs, err := s.jobRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	var jobResponses []*dto.JobResponse
	for _, job := range jobs {
		jobResponses = append(jobResponses, s.mapToJobResponse(&job))
	}

	return jobResponses, nil
}

// DeleteJob deletes a job by its ID.
func (s *jobService) DeleteJob(ctx context.Context, id uint) error {
	err := s.jobRepo.Delete(ctx, id)
	if err != nil {
		s.logger.Error("Failed to delete job", logger.ErrorField(err), logger.Field("job_id", id))
		return err
	}
	s.logger.Info("Job deleted successfully", logger.Field("job_id", id))
	return nil
}

// UpdateJob handles the business logic for updating an existing job.
func (s *jobService) UpdateJob(ctx context.Context, id uint, req *dto.UpdateJobRequest) (*dto.JobResponse, error) {
	// First, find the existing job.
	job, err := s.jobRepo.FindByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to find job for update", logger.ErrorField(err), logger.Field("job_id", id))
		return nil, err // Or a specific not found error
	}

	// Update the job fields from the request.
	retryPolicyBytes, err := json.Marshal(req.RetryPolicy)
	if err != nil {
		s.logger.Error("Failed to marshal retry policy", logger.ErrorField(err))
		return nil, err
	}

	job.Name = req.Name
	job.Description = req.Description
	job.Type = entity.JobType(req.Type)
	job.Payload = datatypes.JSON(req.Payload)
	job.RetryPolicy = datatypes.JSON(retryPolicyBytes)
	job.Timeout = req.Timeout

	// Replace existing schedules with new ones from the request.
	job.Schedules = []entity.TaskSchedule{} // The repository update will handle deletion
	for _, sDto := range req.Schedules {
		job.Schedules = append(job.Schedules, entity.TaskSchedule{
			CronExpression: sDto.CronExpression,
			IsActive:       sDto.IsActive,
			JobID:          job.ID,
		})
	}

	// Persist the updated job. The repository's Update method handles the transaction.
	if err := s.jobRepo.Update(ctx, job); err != nil {
		s.logger.Error("Failed to update job", logger.ErrorField(err), logger.Field("job_id", id))
		return nil, err
	}

	s.logger.Info("Job updated successfully", logger.Field("job_id", id))
	return s.mapToJobResponse(job), nil
}

// mapToJobResponse maps an entity.Job to a dto.JobResponse.
func (s *jobService) mapToJobResponse(job *entity.Job) *dto.JobResponse {
	var retryPolicy dto.RetryPolicyDTO
	_ = json.Unmarshal(job.RetryPolicy, &retryPolicy)

	var schedules []dto.ScheduleResponseDTO
	for _, schedule := range job.Schedules {
		schedules = append(schedules, dto.ScheduleResponseDTO{
			ID:             schedule.ID,
			CronExpression: schedule.CronExpression,
			IsActive:       schedule.IsActive,
			NextExecution:  schedule.NextExecution,
			LastExecution:  schedule.LastExecution,
		})
	}

	return &dto.JobResponse{
		ID:          job.ID,
		Name:        job.Name,
		Description: job.Description,
		Type:        string(job.Type),
		Payload:     json.RawMessage(job.Payload),
		RetryPolicy: retryPolicy,
		Timeout:     job.Timeout,
		Schedules:   schedules,
		CreatedAt:   job.CreatedAt,
		UpdatedAt:   job.UpdatedAt,
	}
}

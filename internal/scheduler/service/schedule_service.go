package service

import (
	"context"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/scheduler/dto"
	"golang-stock-scryper/internal/scheduler/repository"
	"golang-stock-scryper/pkg/logger"
)

// ScheduleService defines the interface for managing schedules.
type ScheduleService interface {
	CreateSchedule(ctx context.Context, req *dto.CreateScheduleRequest) (*dto.ScheduleResponse, error)
	GetScheduleByID(ctx context.Context, id uint) (*dto.ScheduleResponse, error)
	GetAllSchedules(ctx context.Context) ([]*dto.ScheduleResponse, error)
	UpdateSchedule(ctx context.Context, id uint, req *dto.UpdateScheduleRequest) (*dto.ScheduleResponse, error)
	DeleteSchedule(ctx context.Context, id uint) error
}

// NewScheduleService creates a new schedule service.
func NewScheduleService(scheduleRepo repository.TaskScheduleRepository, logger *logger.Logger) ScheduleService {
	return &scheduleService{
		scheduleRepo: scheduleRepo,
		logger:       logger,
	}
}

type scheduleService struct {
	scheduleRepo repository.TaskScheduleRepository
	logger       *logger.Logger
}

// CreateSchedule handles the business logic for creating a new schedule.
func (s *scheduleService) CreateSchedule(ctx context.Context, req *dto.CreateScheduleRequest) (*dto.ScheduleResponse, error) {
	schedule := &entity.TaskSchedule{
		JobID:          req.JobID,
		CronExpression: req.CronExpression,
		IsActive:       req.IsActive,
	}

	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		s.logger.Error("Failed to create schedule", logger.ErrorField(err))
		return nil, err
	}

	s.logger.Info("Schedule created successfully", logger.Field("schedule_id", schedule.ID))
	return s.mapToScheduleResponse(schedule), nil
}

// GetScheduleByID retrieves a schedule by its ID.
func (s *scheduleService) GetScheduleByID(ctx context.Context, id uint) (*dto.ScheduleResponse, error) {
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to find schedule", logger.ErrorField(err), logger.Field("schedule_id", id))
		return nil, err
	}
	return s.mapToScheduleResponse(schedule), nil
}

// GetAllSchedules retrieves all schedules.
func (s *scheduleService) GetAllSchedules(ctx context.Context) ([]*dto.ScheduleResponse, error) {
	schedules, err := s.scheduleRepo.FindAll(ctx)
	if err != nil {
		s.logger.Error("Failed to get all schedules", logger.ErrorField(err))
		return nil, err
	}

	var scheduleResponses []*dto.ScheduleResponse
	for _, schedule := range schedules {
		scheduleResponses = append(scheduleResponses, s.mapToScheduleResponse(&schedule))
	}

	return scheduleResponses, nil
}

// UpdateSchedule handles the business logic for updating an existing schedule.
func (s *scheduleService) UpdateSchedule(ctx context.Context, id uint, req *dto.UpdateScheduleRequest) (*dto.ScheduleResponse, error) {
	schedule, err := s.scheduleRepo.FindByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to find schedule for update", logger.ErrorField(err), logger.Field("schedule_id", id))
		return nil, err
	}

	schedule.CronExpression = req.CronExpression
	schedule.IsActive = req.IsActive

	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		s.logger.Error("Failed to update schedule", logger.ErrorField(err), logger.Field("schedule_id", id))
		return nil, err
	}

	s.logger.Info("Schedule updated successfully", logger.Field("schedule_id", id))
	return s.mapToScheduleResponse(schedule), nil
}

// DeleteSchedule deletes a schedule by its ID.
func (s *scheduleService) DeleteSchedule(ctx context.Context, id uint) error {
	err := s.scheduleRepo.Delete(ctx, id)
	if err != nil {
		s.logger.Error("Failed to delete schedule", logger.ErrorField(err), logger.Field("schedule_id", id))
		return err
	}
	s.logger.Info("Schedule deleted successfully", logger.Field("schedule_id", id))
	return nil
}

// mapToScheduleResponse maps an entity.TaskSchedule to a dto.ScheduleResponse.
func (s *scheduleService) mapToScheduleResponse(schedule *entity.TaskSchedule) *dto.ScheduleResponse {
	return &dto.ScheduleResponse{
		ID:             schedule.ID,
		JobID:          schedule.JobID,
		CronExpression: schedule.CronExpression,
		IsActive:       schedule.IsActive,
		NextExecution:  schedule.NextExecution,
		LastExecution:  schedule.LastExecution,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      schedule.UpdatedAt,
	}
}

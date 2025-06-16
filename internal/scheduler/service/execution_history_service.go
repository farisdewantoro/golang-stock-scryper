package service

import (
	"context"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/scheduler/dto"
	"golang-stock-scryper/internal/scheduler/repository"
	"golang-stock-scryper/pkg/logger"
)

// ExecutionHistoryService defines the interface for managing execution history.
type ExecutionHistoryService interface {
	GetExecutionHistoryByID(ctx context.Context, id uint) (*dto.ExecutionHistoryResponse, error)
	GetAllExecutionHistories(ctx context.Context) ([]*dto.ExecutionHistoryResponse, error)
	GetExecutionHistoriesByJobID(ctx context.Context, jobID uint) ([]*dto.ExecutionHistoryResponse, error)
}

// NewExecutionHistoryService creates a new execution history service.
func NewExecutionHistoryService(historyRepo repository.TaskExecutionHistoryRepository, logger *logger.Logger) ExecutionHistoryService {
	return &executionHistoryService{
		historyRepo: historyRepo,
		logger:      logger,
	}
}

type executionHistoryService struct {
	historyRepo repository.TaskExecutionHistoryRepository
	logger      *logger.Logger
}

// GetExecutionHistoryByID retrieves an execution history record by its ID.
func (s *executionHistoryService) GetExecutionHistoryByID(ctx context.Context, id uint) (*dto.ExecutionHistoryResponse, error) {
	history, err := s.historyRepo.FindByID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to find execution history", logger.ErrorField(err), logger.Field("history_id", id))
		return nil, err
	}
	return s.mapToExecutionHistoryResponse(history), nil
}

// GetAllExecutionHistories retrieves all execution history records.
func (s *executionHistoryService) GetAllExecutionHistories(ctx context.Context) ([]*dto.ExecutionHistoryResponse, error) {
	histories, err := s.historyRepo.FindAll(ctx)
	if err != nil {
		s.logger.Error("Failed to get all execution histories", logger.ErrorField(err))
		return nil, err
	}

	var historyResponses []*dto.ExecutionHistoryResponse
	for _, history := range histories {
		historyResponses = append(historyResponses, s.mapToExecutionHistoryResponse(&history))
	}

	return historyResponses, nil
}

// GetExecutionHistoriesByJobID retrieves all execution history records for a specific job.
func (s *executionHistoryService) GetExecutionHistoriesByJobID(ctx context.Context, jobID uint) ([]*dto.ExecutionHistoryResponse, error) {
	histories, err := s.historyRepo.FindAllByJobID(ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get execution histories by job ID", logger.ErrorField(err), logger.Field("job_id", jobID))
		return nil, err
	}

	var historyResponses []*dto.ExecutionHistoryResponse
	for _, history := range histories {
		historyResponses = append(historyResponses, s.mapToExecutionHistoryResponse(&history))
	}

	return historyResponses, nil
}

// mapToExecutionHistoryResponse maps an entity.TaskExecutionHistory to a dto.ExecutionHistoryResponse.
func (s *executionHistoryService) mapToExecutionHistoryResponse(history *entity.TaskExecutionHistory) *dto.ExecutionHistoryResponse {
	var duration int64
	if history.CompletedAt.Valid {
		duration = history.CompletedAt.Time.Sub(history.StartedAt).Milliseconds()
	}

	return &dto.ExecutionHistoryResponse{
		ID:         history.ID,
		JobID:      history.JobID,
		ScheduleID: history.ScheduleID,
		Status:     string(history.Status),
		ExecutedAt: history.StartedAt,
		Duration:   duration,
		Output:     history.Output.String,
	}
}

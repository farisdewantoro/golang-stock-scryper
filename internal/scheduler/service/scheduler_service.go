package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/scheduler/config"
	"golang-stock-scryper/internal/scheduler/repository"
	"golang-stock-scryper/pkg/common"
	"golang-stock-scryper/pkg/logger"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

// SchedulerService defines the interface for the job scheduling service.
type SchedulerService interface {
	Start(ctx context.Context)
	ProcessJobs(ctx context.Context)
}

// NewSchedulerService creates a new scheduler service.
func NewSchedulerService(jobRepo repository.JobRepository, scheduleRepo repository.TaskScheduleRepository, historyRepo repository.TaskExecutionHistoryRepository, redisClient *redis.Client, logger *logger.Logger, pollingInterval time.Duration, cfg *config.Config) SchedulerService {
	return &schedulerService{
		jobRepo:         jobRepo,
		scheduleRepo:    scheduleRepo,
		historyRepo:     historyRepo,
		redisClient:     redisClient,
		logger:          logger,
		pollingInterval: pollingInterval,
		cronParser:      cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		cfg:             cfg,
	}
}

type schedulerService struct {
	jobRepo         repository.JobRepository
	scheduleRepo    repository.TaskScheduleRepository
	historyRepo     repository.TaskExecutionHistoryRepository
	redisClient     *redis.Client
	logger          *logger.Logger
	pollingInterval time.Duration
	cronParser      cron.Parser
	cfg             *config.Config
}

// Start begins the periodic job processing loop.
func (s *schedulerService) Start(ctx context.Context) {
	ticker := time.NewTicker(s.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Scheduler service stopping")
			return
		case <-ticker.C:
			s.ProcessJobs(ctx)
		}
	}
}

// ProcessJobs finds and enqueues jobs that are due.
func (s *schedulerService) ProcessJobs(ctx context.Context) {
	schedules, err := s.scheduleRepo.FindJobsToSchedule(ctx)
	if err != nil {
		s.logger.Error("Failed to find jobs to schedule", logger.ErrorField(err))
		return
	}

	for _, schedule := range schedules {
		s.publishTask(ctx, schedule)
	}
}

func (s *schedulerService) publishTask(ctx context.Context, schedule entity.TaskSchedule) {
	now := time.Now()

	history := &entity.TaskExecutionHistory{
		JobID:      schedule.JobID,
		ScheduleID: schedule.ID,
		Status:     entity.StatusRunning, // Or a new "Queued" status
		StartedAt:  now,
	}

	if err := s.historyRepo.Create(ctx, history); err != nil {
		s.logger.Error("Failed to create task history", logger.ErrorField(err), logger.Field("schedule_id", schedule.ID))
		return
	}

	taskPayload, err := json.Marshal(history) // Pass history object to executor
	if err != nil {
		s.logger.Error("Failed to marshal task payload", logger.ErrorField(err), logger.Field("history_id", history.ID))
		return
	}

	if err := s.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: common.SchedulerTaskExecutionEventName,
		Values: map[string]interface{}{"payload": taskPayload},
		MaxLen: s.cfg.Redis.StreamMaxLen, // Limit the stream size
	}).Err(); err != nil {
		s.logger.Error("Failed to enqueue task", logger.ErrorField(err), logger.Field("history_id", history.ID))
		history.Status = entity.StatusFailed
		history.CompletedAt.Time = time.Now()
		history.CompletedAt.Valid = true
		history.ErrorMessage = sql.NullString{String: err.Error(), Valid: true}
		errInner := s.historyRepo.Update(ctx, history)
		if errInner != nil {
			s.logger.Error("Failed to update task history", logger.ErrorField(errInner), logger.Field("history_id", history.ID))
		}
		return
	}

	s.logger.Info("Task published successfully", logger.Field("history_id", history.ID))

	// Update schedule for next run
	cronSchedule, err := s.cronParser.Parse(schedule.CronExpression)
	if err != nil {
		s.logger.Error("Failed to parse cron expression", logger.ErrorField(err), logger.Field("schedule_id", schedule.ID))
		return
	}

	schedule.LastExecution.Time = now
	schedule.LastExecution.Valid = true
	schedule.NextExecution.Time = cronSchedule.Next(now)
	schedule.NextExecution.Valid = true

	if err := s.scheduleRepo.Update(ctx, &schedule); err != nil {
		s.logger.Error("Failed to update next execution time", logger.ErrorField(err), logger.Field("schedule_id", schedule.ID))
	}
}

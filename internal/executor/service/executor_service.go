package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/internal/executor/strategy"
	"golang-stock-scryper/pkg/common"
	"golang-stock-scryper/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// ExecutorService manages the execution of tasks.
type ExecutorService interface {
	ProcessTask(ctx context.Context)
}

// NewExecutorService creates a new ExecutorService.
func NewExecutorService(
	redisClient *redis.Client,
	jobRepo repository.JobRepository,
	historyRepo repository.TaskExecutionHistoryRepository,
	log *logger.Logger,
	strategies []strategy.JobExecutionStrategy,
) ExecutorService {
	strategyMap := make(map[entity.JobType]strategy.JobExecutionStrategy)
	for _, s := range strategies {
		strategyMap[s.GetType()] = s
	}

	return &executorService{
		redisClient:        redisClient,
		jobRepo:            jobRepo,
		historyRepo:        historyRepo,
		logger:             log,
		executorStrategies: strategyMap,
	}
}

type executorService struct {
	redisClient        *redis.Client
	jobRepo            repository.JobRepository
	historyRepo        repository.TaskExecutionHistoryRepository
	logger             *logger.Logger
	executorStrategies map[entity.JobType]strategy.JobExecutionStrategy
}

// ProcessTask dequeues and executes a single task.
func (s *executorService) ProcessTask(ctx context.Context) {
	streams, err := s.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    common.RedisStreamGroup,
		Consumer: common.RedisStreamConsumer,
		Streams:  []string{common.SchedulerTaskExecutionEventName, ">"}, // ">" means only new messages
		Count:    1,
		Block:    2 * time.Second, // Block for 2 seconds to allow graceful shutdown
		NoAck:    true,
	}).Result()

	if err != nil {
		// Ignore context cancellation and timeout errors, as they are expected during shutdown or idle periods.
		if err == context.Canceled || err == redis.Nil {
			return
		}
		s.logger.Error("Failed to read from stream", logger.ErrorField(err))
		return
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return
	}

	message := streams[0].Messages[0]

	// The task data is expected to be a JSON string in the 'payload' field.
	taskData, ok := message.Values["payload"].(string)
	if !ok {
		s.logger.Error("field 'payload' not found or not a string in stream message", logger.Field("message_id", message.ID))
		return
	}

	var taskHistory entity.TaskExecutionHistory
	if err := json.Unmarshal([]byte(taskData), &taskHistory); err != nil {
		s.logger.Error("Failed to unmarshal task data", logger.ErrorField(err), logger.Field("message_id", message.ID))
		// Acknowledge the message to prevent reprocessing of a malformed message.
		if err := s.redisClient.XAck(ctx, common.SchedulerTaskExecutionEventName, common.RedisStreamGroup, message.ID).Err(); err != nil {
			s.logger.Error("Failed to acknowledge malformed message", logger.ErrorField(err), logger.Field("message_id", message.ID))
		}
		return
	}

	s.logger.Info("Processing job", logger.Field("job_id", taskHistory.JobID), logger.Field("history_id", taskHistory.ID))

	job, err := s.jobRepo.FindByID(ctx, taskHistory.JobID)
	if err != nil {
		s.logger.Error("Failed to find job", logger.ErrorField(err), logger.Field("job_id", taskHistory.JobID))
		return
	}

	executionCtx, cancelExec := context.WithTimeout(ctx, time.Duration(job.Timeout)*time.Second)
	defer cancelExec()

	s.executeAndUpdate(executionCtx, job, &taskHistory)

}

func (s *executorService) executeAndUpdate(ctx context.Context, job *entity.Job, history *entity.TaskExecutionHistory) {
	strategy, ok := s.executorStrategies[job.Type]
	if !ok {
		err := fmt.Errorf("no executor strategy found for task type: %s", job.Type)
		s.logger.Error("Job execution failed", logger.ErrorField(err), logger.Field("job_id", job.ID))
		history.Status = entity.StatusFailed
		history.ErrorMessage = sql.NullString{String: err.Error(), Valid: true}
	} else {
		output, err := strategy.Execute(ctx, job)
		if err != nil {
			s.logger.Error("Job execution failed", logger.ErrorField(err), logger.Field("job_id", job.ID), logger.IntField("history_id", int(history.ID)))
			history.Status = entity.StatusFailed
			history.ErrorMessage = sql.NullString{String: err.Error(), Valid: true}
		} else {
			s.logger.Info("Job executed successfully", logger.Field("job_id", job.ID), logger.IntField("history_id", int(history.ID)))
			history.Status = entity.StatusCompleted
		}
		history.Output = sql.NullString{String: output, Valid: true}
	}

	history.CompletedAt.Time = time.Now()
	history.CompletedAt.Valid = true

	if err := s.historyRepo.Update(ctx, history); err != nil {
		s.logger.Error("Failed to update task history", logger.ErrorField(err), logger.Field("history_id", history.ID))
	}
	s.logger.Info("Job execution completed", logger.Field("job_id", job.ID), logger.IntField("history_id", int(history.ID)))
}

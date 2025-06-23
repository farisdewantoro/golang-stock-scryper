package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/pkg/common"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/redis"
	"golang-stock-scryper/pkg/utils"

	goRedis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type StockPositionMonitorStrategy struct {
	logger            *logger.Logger
	redisClient       *redis.Client
	stockPositionRepo repository.StockPositionsRepository
}

type StockPositionMonitorPayload struct {
	Interval  string `json:"interval"`
	Range     string `json:"range"`
	SendNotif bool   `json:"send_notif"`
}

type StockPositionMonitorResult struct {
	StockCode string `json:"stock_code"`
	ID        uint   `json:"id"`
	Success   bool   `json:"success"`
	Error     string `json:"error"`
}

func NewStockPositionMonitorStrategy(
	log *logger.Logger,
	redisClient *redis.Client,
	stockPositionRepo repository.StockPositionsRepository) JobExecutionStrategy {
	return &StockPositionMonitorStrategy{logger: log, redisClient: redisClient, stockPositionRepo: stockPositionRepo}
}

func (s *StockPositionMonitorStrategy) GetType() entity.JobType {
	return entity.JobTypeStockPositionMonitor
}

func (s *StockPositionMonitorStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	var payload StockPositionMonitorPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.logger.Error("Failed to unmarshal job payload", logger.ErrorField(err), logger.Field("job_id", job.ID))
		return "", fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	stockPositions, err := s.stockPositionRepo.Get(ctx, dto.GetStockPositionsParam{
		MonitorPosition: utils.ToPointer(true),
		IsActive:        utils.ToPointer(true),
	})
	if err != nil {
		s.logger.Error("Failed to get stocks positions", logger.ErrorField(err))
		return "", fmt.Errorf("failed to get stocks positions: %w", err)
	}

	var results []StockPositionMonitorResult

	for _, stockPosition := range stockPositions {
		fieldsLog := []zap.Field{
			logger.Field("stock_code", stockPosition.StockCode),
			logger.Field("id", stockPosition.ID),
			logger.Field("user_id", stockPosition.UserID),
		}

		streamData := &dto.StreamDataStockPositionMonitor{
			ID:        stockPosition.ID,
			UserID:    stockPosition.UserID,
			StockCode: stockPosition.StockCode,
			Interval:  payload.Interval,
			Range:     payload.Range,
			SendNotif: payload.SendNotif,
		}
		streamDataJSON, err := json.Marshal(streamData)
		if err != nil {
			loggerFields := append(fieldsLog, logger.ErrorField(err))
			s.logger.Error("Failed to marshal stock position monitor payload", loggerFields...)
			results = append(results, StockPositionMonitorResult{
				StockCode: stockPosition.StockCode,
				ID:        stockPosition.ID,
				Success:   false,
				Error:     err.Error(),
			})
			continue
		}

		if err := s.redisClient.XAdd(ctx, &goRedis.XAddArgs{
			Stream: common.RedisStreamStockPositionMonitor,
			Values: map[string]interface{}{"payload": streamDataJSON},
		}).Err(); err != nil {
			loggerFields := append(fieldsLog, logger.ErrorField(err))
			s.logger.Error("Failed to enqueue stock position monitor task", loggerFields...)
			results = append(results, StockPositionMonitorResult{
				StockCode: stockPosition.StockCode,
				ID:        stockPosition.ID,
				Success:   false,
				Error:     err.Error(),
			})
			continue
		}
		results = append(results, StockPositionMonitorResult{
			StockCode: stockPosition.StockCode,
			ID:        stockPosition.ID,
			Success:   true,
		})
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		s.logger.Error("Failed to marshal results", logger.ErrorField(err))
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(resultJSON), nil
}

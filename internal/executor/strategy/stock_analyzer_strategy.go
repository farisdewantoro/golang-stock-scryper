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

	goRedis "github.com/redis/go-redis/v9"
)

// StockAnalyzerStrategy defines the strategy for analyzing stock news.
type StockAnalyzerStrategy struct {
	logger      *logger.Logger
	redisClient *redis.Client
	stockRepo   repository.StocksRepository
}

type StockAnalyzerPayload struct {
	Interval   string   `json:"interval"`
	Range      string   `json:"range"`
	SkipStocks []string `json:"skip_stocks"`
}

type StockAnalyzerResult struct {
	StockCode string `json:"stock_code"`
	Success   bool   `json:"success"`
	Error     string `json:"error"`
}

// NewStockAnalyzerStrategy creates a new StockAnalyzerStrategy.
func NewStockAnalyzerStrategy(log *logger.Logger, redisClient *redis.Client, stockRepo repository.StocksRepository) JobExecutionStrategy {
	return &StockAnalyzerStrategy{logger: log, redisClient: redisClient, stockRepo: stockRepo}
}

// GetType returns the job type this strategy handles.
func (s *StockAnalyzerStrategy) GetType() entity.JobType {
	return entity.JobTypeStockAnalyzer
}

// Execute performs the stock news analysis defined in the job's payload.
func (s *StockAnalyzerStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	var payload StockAnalyzerPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.logger.Error("Failed to unmarshal job payload", logger.ErrorField(err), logger.Field("job_id", job.ID))
		return "", fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	stocks, err := s.stockRepo.GetStocks(ctx)
	if err != nil {
		s.logger.Error("Failed to get stocks", logger.ErrorField(err))
		return "", fmt.Errorf("failed to get stocks: %w", err)
	}

	skipStocks := make(map[string]bool)
	if len(payload.SkipStocks) > 0 {
		for _, stock := range payload.SkipStocks {
			skipStocks[stock] = true
		}
	}

	isSuccess := false

	var results []StockAnalyzerResult
	for _, stock := range stocks {
		if skipStocks[stock.Code] {
			s.logger.Info("Skipping stock", logger.Field("stock_code", stock.Code))
			continue
		}

		streamData := &dto.StreamDataStockAnalyzer{
			StockCode: stock.Code,
		}

		streamDataJSON, err := json.Marshal(streamData)
		if err != nil {
			s.logger.Error("Failed to marshal stock analyzer payload", logger.ErrorField(err))
			results = append(results, StockAnalyzerResult{
				StockCode: stock.Code,
				Success:   false,
				Error:     err.Error(),
			})
			continue
		}

		if err := s.redisClient.XAdd(ctx, &goRedis.XAddArgs{
			Stream: common.RedisStreamStockAnalyzer,
			Values: map[string]interface{}{"payload": streamDataJSON},
		}).Err(); err != nil {
			s.logger.Error("Failed to enqueue stock analyzer task", logger.ErrorField(err), logger.Field("stock_code", stock.Code))
			results = append(results, StockAnalyzerResult{
				StockCode: stock.Code,
				Success:   false,
				Error:     err.Error(),
			})
			continue
		}
		isSuccess = true
		results = append(results, StockAnalyzerResult{
			StockCode: stock.Code,
			Success:   true,
		})
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		s.logger.Error("Failed to marshal results", logger.ErrorField(err))
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	if isSuccess {
		return string(resultJSON), nil
	}

	return "", fmt.Errorf("failed to enqueue stock analyzer task")
}

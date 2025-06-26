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
	logger          *logger.Logger
	redisClient     *redis.Client
	stockRepo       repository.StocksRepository
	tradingViewRepo repository.TradingViewRepository
}

type StockAnalyzerPayload struct {
	SkipStocks       []string `json:"skip_stocks"`
	UseTradingView   bool     `json:"use_trading_view"`
	UseStockList     bool     `json:"use_stock_list"`
	AdditionalStocks []string `json:"additional_stocks"`
}

type StockAnalyzerResult struct {
	StockCode string `json:"stock_code"`
	Success   bool   `json:"success"`
	Error     string `json:"error"`
}

// NewStockAnalyzerStrategy creates a new StockAnalyzerStrategy.
func NewStockAnalyzerStrategy(log *logger.Logger, redisClient *redis.Client, stockRepo repository.StocksRepository, tradingViewRepo repository.TradingViewRepository) JobExecutionStrategy {
	return &StockAnalyzerStrategy{logger: log, redisClient: redisClient, stockRepo: stockRepo, tradingViewRepo: tradingViewRepo}
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

	var stocks []string

	if payload.UseStockList {
		stocksList, err := s.stockRepo.GetStocks(ctx)
		if err != nil {
			s.logger.Error("Failed to get stocks", logger.ErrorField(err))
			return "", fmt.Errorf("failed to get stocks: %w", err)
		}

		for _, stock := range stocksList {
			stocks = append(stocks, stock.Code)
		}
	}

	if len(payload.AdditionalStocks) > 0 {
		stocks = append(stocks, payload.AdditionalStocks...)
	}

	if payload.UseTradingView {
		stocksList, err := s.tradingViewRepo.GetStockBuyList(ctx)
		if err != nil {
			s.logger.Error("Failed to get stocks", logger.ErrorField(err))
			return "", fmt.Errorf("failed to get stocks: %w", err)
		}

		s.logger.Info("Get stocks from TradingView for analysis", logger.IntField("count", len(stocksList)))

		stocks = append(stocks, stocksList...)
	}

	skipStocks := make(map[string]bool)
	if len(payload.SkipStocks) > 0 {
		for _, stock := range payload.SkipStocks {
			skipStocks[stock] = true
		}
	}

	isSuccess := false

	var results []StockAnalyzerResult
	for _, code := range stocks {
		if skipStocks[code] {
			s.logger.Info("Skipping stock", logger.Field("stock_code", code))
			continue
		}

		streamData := &dto.StreamDataStockAnalyzer{
			StockCode: code,
		}

		streamDataJSON, err := json.Marshal(streamData)
		if err != nil {
			s.logger.Error("Failed to marshal stock analyzer payload", logger.ErrorField(err))
			results = append(results, StockAnalyzerResult{
				StockCode: code,
				Success:   false,
				Error:     err.Error(),
			})
			continue
		}

		if err := s.redisClient.XAdd(ctx, &goRedis.XAddArgs{
			Stream: common.RedisStreamStockAnalyzer,
			Values: map[string]interface{}{"payload": streamDataJSON},
		}).Err(); err != nil {
			s.logger.Error("Failed to enqueue stock analyzer task", logger.ErrorField(err), logger.Field("stock_code", code))
			results = append(results, StockAnalyzerResult{
				StockCode: code,
				Success:   false,
				Error:     err.Error(),
			})
			continue
		}
		isSuccess = true
		results = append(results, StockAnalyzerResult{
			StockCode: code,
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

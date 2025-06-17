package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/telegram"
	"golang-stock-scryper/pkg/utils"
	"time"

	"github.com/patrickmn/go-cache"
)

// StockPriceAlertStrategy defines the strategy for scraping stock news.
type StockPriceAlertStrategy struct {
	logger                   *logger.Logger
	inmemoryCache            *cache.Cache
	yahooFinanceRepository   repository.YahooFinanceRepository
	telegramNotifier         telegram.Notifier
	stockPositionsRepository repository.StockPositionsRepository
}

// StockPriceAlertPayload defines the payload for stock price alert.
type StockPriceAlertPayload struct {
	DataInterval string `json:"data_interval"`
	DataRange    string `json:"data_range"`
}

// StockPriceAlertResult defines the result for stock price alert.
type StockPriceAlertResult struct {
	StockCode string `json:"stock_code"`
	Status    string `json:"status"`
	Errors    string `json:"errors"`
}

// NewStockPriceAlertStrategy creates a new instance of StockPriceAlertStrategy.
func NewStockPriceAlertStrategy(logger *logger.Logger, yahooFinanceRepository repository.YahooFinanceRepository, telegramNotifier telegram.Notifier, stockPositionsRepository repository.StockPositionsRepository) *StockPriceAlertStrategy {
	return &StockPriceAlertStrategy{
		logger:                   logger,
		inmemoryCache:            cache.New(5*time.Minute, 10*time.Minute),
		yahooFinanceRepository:   yahooFinanceRepository,
		telegramNotifier:         telegramNotifier,
		stockPositionsRepository: stockPositionsRepository,
	}
}

// GetType returns the job type this strategy handles.
func (s *StockPriceAlertStrategy) GetType() entity.JobType {
	return entity.JobTypeStockPriceAlert
}

// Execute runs the stock alert job.
func (s *StockPriceAlertStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	s.logger.DebugContext(ctx, "Executing stock alert job", logger.IntField("job_id", int(job.ID)))

	var (
		payload StockPriceAlertPayload
		result  []StockPriceAlertResult
	)
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return FAILED, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	stockPositions, err := s.stockPositionsRepository.Get(ctx, dto.GetStockPositionsParam{
		IsAlertTriggered: utils.ToPointer(true),
	})
	if err != nil {
		return FAILED, err
	}

	for _, stockPosition := range stockPositions {

		s.logger.DebugContext(ctx, "Processing stock alert", logger.StringField("stock_code", stockPosition.StockCode))
		stockData, err := s.yahooFinanceRepository.Get(ctx, dto.GetStockDataParam{
			StockCode: stockPosition.StockCode,
			Range:     payload.DataRange,
			Interval:  payload.DataInterval,
		})
		if err != nil {
			s.logger.Error("Failed to get stock data", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			result = append(result, StockPriceAlertResult{
				StockCode: stockPosition.StockCode,
				Status:    FAILED,
				Errors:    err.Error(),
			})
			continue
		}

		reachTakeProfitIn := 0.0
		reachStopLossIn := 0.0
		for _, stockDataPoint := range stockData.OHLCV {
			if stockDataPoint.High >= stockPosition.TakeProfitPrice {
				reachTakeProfitIn = stockDataPoint.High
			}
			if stockDataPoint.Low <= stockPosition.StopLossPrice {
				reachStopLossIn = stockDataPoint.Low
			}
		}

		if reachTakeProfitIn > 0 {
			message := telegram.FormatStockAlertResultForTelegram(telegram.TakeProfit, stockPosition.StockCode, stockPosition.TakeProfitPrice, reachTakeProfitIn)
			err := s.telegramNotifier.SendMessage(message)
			if err != nil {
				s.logger.Error("Failed to send take profit alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			}
		}
		if reachStopLossIn > 0 {
			message := telegram.FormatStockAlertResultForTelegram(telegram.StopLoss, stockPosition.StockCode, stockPosition.StopLossPrice, reachStopLossIn)
			err := s.telegramNotifier.SendMessage(message)
			if err != nil {
				s.logger.Error("Failed to send stop loss alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			}
		}

		if reachTakeProfitIn > 0 || reachStopLossIn > 0 {
			stockPosition.LastAlertedAt = utils.TimeNowWIB()
			err := s.stockPositionsRepository.Update(ctx, stockPosition)
			if err != nil {
				s.logger.Error("Failed to update stock position", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			}
		}

		// set result
		if err != nil {
			result = append(result, StockPriceAlertResult{
				StockCode: stockPosition.StockCode,
				Status:    FAILED,
				Errors:    err.Error(),
			})
		} else if reachTakeProfitIn > 0 || reachStopLossIn > 0 {
			result = append(result, StockPriceAlertResult{
				StockCode: stockPosition.StockCode,
				Status:    SUCCESS,
			})
		} else {
			result = append(result, StockPriceAlertResult{
				StockCode: stockPosition.StockCode,
				Status:    SKIPPED,
			})
		}
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(resultJSON), nil
}

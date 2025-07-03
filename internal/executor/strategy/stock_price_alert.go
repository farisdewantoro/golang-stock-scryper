package strategy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/pkg/logger"
	redisPkg "golang-stock-scryper/pkg/redis"
	"golang-stock-scryper/pkg/telegram"
	"golang-stock-scryper/pkg/utils"
	"math"
	"strconv"
	"time"

	redis "github.com/redis/go-redis/v9"

	"github.com/patrickmn/go-cache"
)

const (
	REDIS_KEY_STOCK_PRICE_ALERT = "stock_price_alert:%s:%s"
	REDIS_KEY_LAST_PRICE        = "last_price:%s"
)

// StockPriceAlertStrategy defines the strategy for scraping stock news.
type StockPriceAlertStrategy struct {
	logger                   *logger.Logger
	inmemoryCache            *cache.Cache
	yahooFinanceRepository   repository.YahooFinanceRepository
	telegramNotifier         telegram.Notifier
	stockPositionsRepository repository.StockPositionsRepository
	redisClient              *redisPkg.Client
}

// StockPriceAlertPayload defines the payload for stock price alert.
type StockPriceAlertPayload struct {
	DataInterval                string  `json:"data_interval"`
	DataRange                   string  `json:"data_range"`
	AlertTriggerWindowDuration  string  `json:"alert_trigger_window_duration"`
	AlertCacheDuration          string  `json:"alert_cache_duration"`
	AlertResendThresholdPercent float64 `json:"alert_resend_threshold_percent"`
}

// StockPriceAlertResult defines the result for stock price alert.
type StockPriceAlertResult struct {
	StockCode string `json:"stock_code"`
	Status    string `json:"status"`
	Errors    string `json:"errors"`
}

// NewStockPriceAlertStrategy creates a new instance of StockPriceAlertStrategy.
func NewStockPriceAlertStrategy(logger *logger.Logger, yahooFinanceRepository repository.YahooFinanceRepository, telegramNotifier telegram.Notifier, stockPositionsRepository repository.StockPositionsRepository, redisClient *redisPkg.Client) *StockPriceAlertStrategy {
	return &StockPriceAlertStrategy{
		logger:                   logger,
		inmemoryCache:            cache.New(5*time.Minute, 10*time.Minute),
		yahooFinanceRepository:   yahooFinanceRepository,
		telegramNotifier:         telegramNotifier,
		stockPositionsRepository: stockPositionsRepository,
		redisClient:              redisClient,
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
		results []StockPriceAlertResult
	)
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.logger.Error("Failed to unmarshal job payload", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return FAILED, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	alertTriggerWindowDuration, err := time.ParseDuration(payload.AlertTriggerWindowDuration)
	if err != nil {
		s.logger.Error("Failed to parse alert_trigger_window_duration", logger.ErrorField(err), logger.StringField("alert_trigger_window_duration", payload.AlertTriggerWindowDuration), logger.IntField("job_id", int(job.ID)))
		return FAILED, fmt.Errorf("failed to parse alert_trigger_window_duration: %w", err)
	}

	alertCacheDuration, err := time.ParseDuration(payload.AlertCacheDuration)
	if err != nil {
		s.logger.Error("Failed to parse alert_cache_duration", logger.ErrorField(err), logger.StringField("alert_cache_duration", payload.AlertCacheDuration), logger.IntField("job_id", int(job.ID)))
		return FAILED, fmt.Errorf("failed to parse alert_cache_duration: %w", err)
	}

	alertTriggerWindowTime := utils.TimeNowWIB().Add(-alertTriggerWindowDuration)

	stockPositions, err := s.stockPositionsRepository.Get(ctx, dto.GetStockPositionsParam{
		PriceAlert: utils.ToPointer(true),
		IsActive:   utils.ToPointer(true),
	})
	if err != nil {
		return FAILED, err
	}

	for _, stockPosition := range stockPositions {

		resultData := StockPriceAlertResult{
			StockCode: stockPosition.StockCode,
		}

		s.logger.DebugContext(ctx, "Processing stock alert", logger.StringField("stock_code", stockPosition.StockCode))
		stockData, err := s.yahooFinanceRepository.Get(ctx, dto.GetStockDataParam{
			StockCode: stockPosition.StockCode,
			Range:     payload.DataRange,
			Interval:  payload.DataInterval,
		})
		if err != nil {
			s.logger.Error("Failed to get stock data", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			resultData.Status = FAILED
			resultData.Errors = err.Error()
			results = append(results, resultData)
			continue
		}

		// set last price in Redis
		key := fmt.Sprintf(REDIS_KEY_LAST_PRICE, stockPosition.StockCode)
		redisPipe := s.redisClient.Pipeline()
		redisPipe.HSet(ctx, key, map[string]interface{}{
			"price":     stockData.MarketPrice,
			"timestamp": utils.TimeNowWIB().Unix(),
		})
		redisPipe.Expire(ctx, key, alertCacheDuration+2*time.Minute)
		_, errRedis := redisPipe.Exec(ctx)
		if errRedis != nil {
			s.logger.Error("Failed to execute Redis pipeline",
				logger.ErrorField(errRedis), logger.StringField("stock_code", stockPosition.StockCode))
		}

		reachTakeProfitIn := 0.0
		reachStopLossIn := 0.0
		timestampProfit := int64(0)
		timestampLoss := int64(0)

		// check if historical price already reach take profit or stop loss
		for _, stockDataPoint := range stockData.OHLCV {
			if stockDataPoint.Timestamp < alertTriggerWindowTime.Unix() {
				continue
			}
			if stockDataPoint.High >= stockPosition.TakeProfitPrice {
				reachTakeProfitIn = stockDataPoint.High
				timestampProfit = stockDataPoint.Timestamp

			}
			if stockDataPoint.Low <= stockPosition.StopLossPrice {
				reachStopLossIn = stockDataPoint.Low
				timestampLoss = stockDataPoint.Timestamp
			}
		}

		// check if market price already reach take profit or stop loss
		if stockData.MarketPrice != 0 && stockData.MarketPrice >= stockPosition.TakeProfitPrice {
			reachTakeProfitIn = stockData.MarketPrice
			timestampProfit = utils.TimeNowWIB().Unix()
		}
		if stockData.MarketPrice != 0 && stockData.MarketPrice <= stockPosition.StopLossPrice {
			reachStopLossIn = stockData.MarketPrice
			timestampLoss = utils.TimeNowWIB().Unix()
		}

		if reachTakeProfitIn > 0 {
			err = s.sendTelegramMessageAlert(
				ctx,
				&stockPosition,
				telegram.TakeProfit,
				reachTakeProfitIn,
				stockPosition.TakeProfitPrice,
				timestampProfit,
				alertCacheDuration,
				payload.AlertResendThresholdPercent,
			)
		}
		if reachStopLossIn > 0 {
			err = s.sendTelegramMessageAlert(
				ctx,
				&stockPosition,
				telegram.StopLoss,
				reachStopLossIn,
				stockPosition.StopLossPrice,
				timestampLoss,
				alertCacheDuration,
				payload.AlertResendThresholdPercent,
			)
		}

		if reachTakeProfitIn > 0 || reachStopLossIn > 0 {
			stockPosition.LastPriceAlertAt = utils.ToPointer(utils.TimeNowWIB())
			errSql := s.stockPositionsRepository.Update(ctx, stockPosition)
			if errSql != nil {
				s.logger.Error("Failed to update stock position", logger.ErrorField(errSql), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Status = FAILED
				resultData.Errors = errSql.Error()
				results = append(results, resultData)
			}
		}

		// set result
		if err != nil {
			s.logger.Error("Failed to send stock alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			resultData.Status = FAILED
			resultData.Errors = err.Error()
			results = append(results, resultData)
		} else if reachTakeProfitIn > 0 || reachStopLossIn > 0 {
			resultData.Status = SUCCESS
			results = append(results, resultData)
		} else {
			resultData.Status = SKIPPED
			results = append(results, resultData)
		}
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(resultJSON), nil
}

func (s *StockPriceAlertStrategy) sendTelegramMessageAlert(ctx context.Context,
	stockPosition *entity.StockPosition,
	alertType telegram.AlertType,
	triggerPrice float64,
	targetPrice float64,
	timestamp int64,
	cacheDuration time.Duration,
	alertResendThresholdPercent float64) error {
	ok, err := s.shouldTriggerAlert(ctx, stockPosition, triggerPrice, alertType, alertResendThresholdPercent)
	if err != nil {
		s.logger.Error("Failed to check alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
		return err
	}
	if !ok {
		return nil
	}

	message := telegram.FormatStockAlertResultForTelegram(alertType, stockPosition.StockCode, triggerPrice, targetPrice, timestamp)
	err = s.telegramNotifier.SendMessageUser(message, stockPosition.User.TelegramID)
	if err != nil {
		s.logger.Error("Failed to send alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
	}

	s.logger.Debug("Send alert", logger.StringField("stock_code", stockPosition.StockCode), logger.StringField("alert_type", string(alertType)))

	return s.redisClient.Set(ctx, fmt.Sprintf(REDIS_KEY_STOCK_PRICE_ALERT, alertType, stockPosition.StockCode), triggerPrice, cacheDuration).Err()
}

func (s *StockPriceAlertStrategy) getLastAlertPrice(ctx context.Context, stockPosition *entity.StockPosition, alertType telegram.AlertType) (float64, error) {
	lastAlertPrice, err := s.redisClient.Get(ctx, fmt.Sprintf(REDIS_KEY_STOCK_PRICE_ALERT, alertType, stockPosition.StockCode)).Result()
	if err != nil && errors.Is(err, redis.Nil) {
		return 0, nil // belum pernah ada alert
	}

	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}

	return strconv.ParseFloat(lastAlertPrice, 64)
}

func (s *StockPriceAlertStrategy) shouldTriggerAlert(ctx context.Context,
	stockPosition *entity.StockPosition,
	triggerPrice float64,
	alertType telegram.AlertType,
	alertResendThresholdPercent float64) (bool, error) {

	lastAlertPrice, err := s.getLastAlertPrice(ctx, stockPosition, alertType)
	if err != nil {
		return false, err
	}

	if lastAlertPrice == 0 {
		// Belum ada alert sebelumnya, trigger
		return true, nil
	}

	// Hitung selisih persentase
	diff := math.Abs(triggerPrice - lastAlertPrice)
	percentChange := (diff / lastAlertPrice) * 100

	if percentChange >= alertResendThresholdPercent {
		s.logger.Debug("Trigger Resend alert", logger.StringField("stock_code", stockPosition.StockCode), logger.IntField("trigger_price", int(triggerPrice)), logger.IntField("last_alert_price", int(lastAlertPrice)), logger.IntField("percent_change", int(percentChange)))
		return true, nil
	}

	s.logger.Debug("Skip Resend alert", logger.StringField("stock_code", stockPosition.StockCode), logger.IntField("trigger_price", int(triggerPrice)), logger.IntField("last_alert_price", int(lastAlertPrice)), logger.IntField("percent_change", int(percentChange)))

	return false, nil
}

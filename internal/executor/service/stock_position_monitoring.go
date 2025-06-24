package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/pkg/common"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/telegram"
	"golang-stock-scryper/pkg/utils"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type StockPositionMonitoringService interface {
	ProcessTask(ctx context.Context)
	ProcessRetries(ctx context.Context)
	Execute(ctx context.Context, streamData dto.StreamDataStockPositionMonitor) error
}

type stockPositionMonitoringService struct {
	cfg                         *config.Config
	log                         *logger.Logger
	redisClient                 *redis.Client
	aiRepo                      repository.AIRepository
	yahooFinance                repository.YahooFinanceRepository
	stockPositionRepo           repository.StockPositionsRepository
	stockNewsSummaryRepo        repository.StockNewsSummaryRepository
	stockPositionMonitoringRepo repository.StockPositionsMonitoringsRepository
	telegramBot                 telegram.Notifier
}

func NewStockPositionMonitoringService(cfg *config.Config, log *logger.Logger,
	redisClient *redis.Client,
	aiRepo repository.AIRepository,
	yahooFinance repository.YahooFinanceRepository,
	stockPositionRepo repository.StockPositionsRepository,
	stockNewsSummaryRepo repository.StockNewsSummaryRepository,
	stockPositionMonitoringRepo repository.StockPositionsMonitoringsRepository,
	telegramBot telegram.Notifier) StockPositionMonitoringService {
	return &stockPositionMonitoringService{
		cfg:                         cfg,
		log:                         log,
		redisClient:                 redisClient,
		aiRepo:                      aiRepo,
		yahooFinance:                yahooFinance,
		stockPositionRepo:           stockPositionRepo,
		stockNewsSummaryRepo:        stockNewsSummaryRepo,
		stockPositionMonitoringRepo: stockPositionMonitoringRepo,
		telegramBot:                 telegramBot,
	}
}

func (s *stockPositionMonitoringService) ProcessTask(ctx context.Context) {
	streams, err := s.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    common.RedisStreamGroup,
		Consumer: common.RedisStreamConsumer,
		Streams:  []string{common.RedisStreamStockPositionMonitor, ">"}, // ">" means only new messages
		Count:    1,
		Block:    2 * time.Second, // Block for 2 seconds to allow graceful shutdown
	}).Result()
	if err != nil {

		// Ignore context cancellation and timeout errors, as they are expected during shutdown or idle periods.
		if err == context.Canceled || err == redis.Nil {
			return
		}
		s.log.Error("Failed to read from stream", logger.ErrorField(err))

		return
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		s.log.Debug("No messages found", logger.StringField("stream", common.RedisStreamStockPositionMonitor))
		return
	}

	message := streams[0].Messages[0]

	// The task data is expected to be a JSON string in the 'payload' field.
	taskData, ok := message.Values["payload"].(string)
	if !ok {
		s.log.Error("field 'payload' not found or not a string in stream message", logger.Field("message_id", message.ID))
		return
	}

	var streamData dto.StreamDataStockPositionMonitor
	if err := json.Unmarshal([]byte(taskData), &streamData); err != nil {
		s.log.Error("Failed to unmarshal task data", logger.ErrorField(err), logger.Field("message_id", message.ID))
		return
	}

	loggerFields := []zap.Field{
		logger.StringField("stock_code", streamData.StockCode),
		logger.StringField("interval", streamData.Interval),
		logger.StringField("range", streamData.Range),
		logger.StringField("message_id", message.ID),
	}

	s.log.Debug("Processing stock position monitor task", loggerFields...)

	if err := s.Execute(ctx, streamData); err != nil {
		loggerFields = append(loggerFields, logger.ErrorField(err))
		s.log.Error("Failed to execute stock position monitor task", loggerFields...)
		return
	}

	if err := s.AckNDel(ctx, common.RedisStreamStockPositionMonitor, message.ID); err != nil {
		loggerFields = append(loggerFields, logger.ErrorField(err))
		s.log.Error("Failed to acknowledge and delete stock position monitor task", loggerFields...)
		return
	}

	s.log.Debug("Stock position monitor task processed successfully", loggerFields...)

}

func (s *stockPositionMonitoringService) Execute(ctx context.Context, req dto.StreamDataStockPositionMonitor) error {
	stockData, err := s.yahooFinance.Get(ctx, dto.GetStockDataParam{
		StockCode: req.StockCode,
		Interval:  req.Interval,
		Range:     req.Range,
	})
	if err != nil {
		s.log.Error("Failed to get stock data", logger.ErrorField(err))
		return err
	}

	stockPositions, err := s.stockPositionRepo.Get(ctx, dto.GetStockPositionsParam{
		IDs: []uint{req.StockPositionID},
	})
	if err != nil {
		s.log.Error("Failed to get stock position", logger.ErrorField(err))
		return err
	}

	if len(stockPositions) == 0 {
		s.log.Error("Stock position not found", logger.Field("id", req.StockPositionID))
		return fmt.Errorf("stock position not found")
	}

	stockPosition := stockPositions[0]

	loggerFields := []zap.Field{
		logger.StringField("stock_code", req.StockCode),
		logger.StringField("interval", req.Interval),
		logger.StringField("range", req.Range),
		logger.IntField("user_id", int(stockPosition.UserID)),
		logger.IntField("id", int(stockPosition.ID)),
	}
	if !stockPosition.IsActive {
		s.log.Warn("Stock position is not active", loggerFields...)
		return nil
	}

	lastSummary, err := s.stockNewsSummaryRepo.GetLast(ctx, time.Now().Add(-time.Hour*24), stockPosition.StockCode)
	if err != nil {
		s.log.Error("Failed to get last stock news summary", logger.ErrorField(err))
		return err
	}

	aiResp, err := s.aiRepo.PositionMonitoring(ctx, &dto.PositionMonitoringRequest{
		Symbol:               stockPosition.StockCode,
		BuyPrice:             stockPosition.BuyPrice,
		BuyTime:              stockPosition.BuyDate,
		MaxHoldingPeriodDays: stockPosition.MaxHoldingPeriodDays,
		TargetPrice:          stockPosition.TakeProfitPrice,
		StopLoss:             stockPosition.StopLossPrice,
	}, stockData, lastSummary)

	if err != nil {
		s.log.Error("Failed to analyze stock", logger.ErrorField(err))
		return err
	}

	dataJSON, err := json.Marshal(aiResp)
	if err != nil {
		s.log.Error("Failed to marshal gemini response", logger.ErrorField(err))
		return err
	}

	err = s.stockPositionMonitoringRepo.Create(ctx, &entity.StockPositionMonitoring{
		UserID:          stockPosition.UserID,
		StockPositionID: stockPosition.ID,
		TriggeredAlert:  stockPosition.MonitorPosition,
		Interval:        req.Interval,
		Range:           req.Range,
		Signal:          aiResp.Recommendation.Action,
		ConfidenceScore: float64(aiResp.Recommendation.ConfidenceLevel),
		TechnicalScore:  float64(aiResp.TechnicalAnalysis.TechnicalScore),
		NewsScore:       float64(aiResp.NewsSummary.ConfidenceScore),
		Data:            dataJSON,
	})

	if err != nil {
		s.log.Error("Failed to create stock signal", logger.ErrorField(err))
		return err
	}

	shouldSendTelegram := (stockPosition.MonitorPosition &&
		aiResp.Recommendation.Action != "HOLD" && aiResp.Recommendation.ConfidenceLevel >= 60)

	if req.SendToTelegram || shouldSendTelegram {
		msg := telegram.FormatPositionMonitoringMessage(aiResp)
		if err := s.telegramBot.SendMessageUser(msg, int64(stockPosition.User.TelegramID)); err != nil {
			s.log.Error("Failed to send notification", logger.ErrorField(err))
			return nil
		}

		stockPosition.LastPriceAlertAt = utils.ToPointer(utils.TimeNowWIB())
		errSql := s.stockPositionRepo.Update(ctx, stockPosition)
		if errSql != nil {
			s.log.Error("Failed to update stock position", logger.ErrorField(errSql), logger.StringField("stock_code", stockPosition.StockCode))
		}
	}

	return nil
}

func (s *stockPositionMonitoringService) ProcessRetries(ctx context.Context) {
	msgs, _, err := s.redisClient.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   common.RedisStreamStockPositionMonitor,
		Group:    common.RedisStreamGroup,
		Consumer: common.RedisStreamConsumer + "-retry",
		MinIdle:  s.cfg.Executor.RedisStreamStockPositionMonitorMaxIdleDuration,
		Start:    "0",
		Count:    1,
	}).Result()

	if err != nil {
		s.log.Error("Failed to claim stock position monitor task on retry", logger.ErrorField(err))
		return
	}

	if len(msgs) == 0 {
		s.log.Debug("Retry No pending messages found", logger.StringField("stream", common.RedisStreamStockPositionMonitor))
		return
	}

	s.log.Info("Found pending messages", logger.StringField("stream", common.RedisStreamStockPositionMonitor))

	pendingInfo, err := s.redisClient.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: common.RedisStreamStockPositionMonitor,
		Group:  common.RedisStreamGroup,
		Start:  msgs[0].ID,
		End:    msgs[0].ID,
		Count:  1,
	}).Result()

	if err != nil {
		s.log.Error("Failed to get pending info", logger.ErrorField(err))
		return
	}

	if len(pendingInfo) == 0 {
		s.log.Warn("pending msg not found, but exist on xautoclaim",
			logger.StringField("stream", common.RedisStreamStockAnalyzer),
			logger.StringField("message_id", msgs[0].ID))
		return
	}

	msg := msgs[0]
	// The task data is expected to be a JSON string in the 'payload' field.
	taskData, ok := msg.Values["payload"].(string)
	if !ok {
		s.log.Error("field 'payload' not found or not a string in stream message", logger.Field("message_id", msg.ID))
		return
	}

	var streamData dto.StreamDataStockPositionMonitor
	if err := json.Unmarshal([]byte(taskData), &streamData); err != nil {
		s.log.Error("Failed to unmarshal task data", logger.ErrorField(err), logger.Field("message_id", msg.ID))
		return
	}

	if err := s.Execute(ctx, dto.StreamDataStockPositionMonitor{
		StockPositionID: streamData.StockPositionID,
		StockCode:       streamData.StockCode,
		Interval:        streamData.Interval,
		Range:           streamData.Range,
		SendToTelegram:  streamData.SendToTelegram,
		UserID:          streamData.UserID,
	}); err != nil {
		s.log.Error("Failed to analyze stock", logger.ErrorField(err), logger.Field("message_id", msg.ID), logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))

		if pendingInfo[0].RetryCount+1 >= int64(s.cfg.Executor.RedisStreamStockPositionMonitorMaxRetry) {
			s.log.Error("pending msg retry count exceeded",
				logger.StringField("stream", common.RedisStreamStockPositionMonitor),
				logger.StringField("message_id", msg.ID),
				logger.StringField("stock_code", streamData.StockCode),
				logger.StringField("interval", streamData.Interval),
				logger.StringField("range", streamData.Range),
				logger.IntField("retry_count", int(pendingInfo[0].RetryCount+1)),
				logger.IntField("max_retry", s.cfg.Executor.RedisStreamStockPositionMonitorMaxRetry),
			)
			errType := fmt.Sprintf("Retry count exceeded for event %s", common.RedisStreamStockPositionMonitor)
			data := fmt.Sprintf("%s | %s | %s", streamData.StockCode, streamData.Interval, streamData.Range)
			msgTelegram := telegram.FormatErrorAlertMessage(utils.TimeNowWIB(), errType, err.Error(), data)
			if err := s.telegramBot.SendMessage(msgTelegram); err != nil {
				s.log.Error("Failed to send telegram message retry exceeded ", logger.ErrorField(err), logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))
			}
			if err := s.AckNDel(ctx, common.RedisStreamStockPositionMonitor, msg.ID); err != nil {
				s.log.Error("Failed to acknowledge and delete stock position monitor task", logger.ErrorField(err), logger.Field("message_id", msg.ID))
				return
			}
			return
		}
		return
	}

	if err := s.AckNDel(ctx, common.RedisStreamStockPositionMonitor, msg.ID); err != nil {
		s.log.Error("Failed to acknowledge and delete stock position monitor task", logger.ErrorField(err), logger.Field("message_id", msg.ID))
		return
	}
	s.log.Info("Retry Stock position monitor task processed successfully", logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))

}

func (s *stockPositionMonitoringService) AckNDel(ctx context.Context, streamName string, messageID string) error {
	if err := s.redisClient.XAck(ctx, streamName, common.RedisStreamGroup, messageID).Err(); err != nil {
		loggerFields := []zap.Field{
			logger.StringField("stream_name", streamName),
			logger.StringField("message_id", messageID),
			logger.ErrorField(err),
		}
		s.log.Error("Failed to acknowledge stock position monitor task on retry", loggerFields...)
		return err
	}
	if err := s.redisClient.XDel(ctx, streamName, messageID).Err(); err != nil {
		loggerFields := []zap.Field{
			logger.StringField("stream_name", streamName),
			logger.StringField("message_id", messageID),
			logger.ErrorField(err),
		}
		s.log.Error("Failed to delete stock position monitor task on retry", loggerFields...)
		return err
	}
	return nil
}

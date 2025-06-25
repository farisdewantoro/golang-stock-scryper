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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"
)

type StockAnalyzerMultiTimeframeService interface {
	ProcessTask(ctx context.Context)
	ProcessRetries(ctx context.Context)
	Execute(ctx context.Context, data dto.StreamDataStockAnalyzer) error
}

type stockAnalyzerMultiTimeframeService struct {
	cfg                  *config.Config
	log                  *logger.Logger
	redisClient          *redis.Client
	aiRepo               repository.AIRepository
	yahooFinance         repository.YahooFinanceRepository
	stockNewsSummaryRepo repository.StockNewsSummaryRepository
	stockSignalRepo      repository.StockSignalRepository
	telegramBot          telegram.Notifier
}

func NewStockAnalyzerMultiTimeframeService(cfg *config.Config, log *logger.Logger,
	redisClient *redis.Client,
	aiRepo repository.AIRepository,
	yahooFinance repository.YahooFinanceRepository,
	stockNewsSummaryRepo repository.StockNewsSummaryRepository,
	stockSignalRepo repository.StockSignalRepository,
	telegramBot telegram.Notifier) StockAnalyzerMultiTimeframeService {
	return &stockAnalyzerMultiTimeframeService{
		cfg:                  cfg,
		log:                  log,
		redisClient:          redisClient,
		aiRepo:               aiRepo,
		yahooFinance:         yahooFinance,
		stockNewsSummaryRepo: stockNewsSummaryRepo,
		stockSignalRepo:      stockSignalRepo,
		telegramBot:          telegramBot,
	}
}

func (s *stockAnalyzerMultiTimeframeService) ProcessTask(ctx context.Context) {
	streams, err := s.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    common.RedisStreamGroup,
		Consumer: common.RedisStreamConsumer,
		Streams:  []string{common.RedisStreamStockAnalyzer, ">"}, // ">" means only new messages
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
		s.log.Debug("No messages found", logger.StringField("stream", common.RedisStreamStockAnalyzer))
		return
	}

	message := streams[0].Messages[0]

	// The task data is expected to be a JSON string in the 'payload' field.
	taskData, ok := message.Values["payload"].(string)
	if !ok {
		s.log.Error("field 'payload' not found or not a string in stream message", logger.Field("message_id", message.ID))
		return
	}

	var streamData dto.StreamDataStockAnalyzer
	if err := json.Unmarshal([]byte(taskData), &streamData); err != nil {
		s.log.Error("Failed to unmarshal task data", logger.ErrorField(err), logger.Field("message_id", message.ID))
		return
	}

	s.log.Debug("Processing stock analyzer task", logger.StringField("stock_code", streamData.StockCode))

	if err := s.Execute(ctx, streamData); err != nil {
		s.log.Error("Failed to analyze stock", logger.ErrorField(err), logger.Field("message_id", message.ID), logger.StringField("stock_code", streamData.StockCode))
		return
	}
	if err := s.AckNDel(ctx, common.RedisStreamStockAnalyzer, message.ID); err != nil {
		s.log.Error("Failed to acknowledge and delete stock analyzer task", logger.ErrorField(err), logger.Field("message_id", message.ID))
		return
	}

	s.log.Debug("Stock analyzer task processed successfully", logger.StringField("stock_code", streamData.StockCode))

}

func (s *stockAnalyzerMultiTimeframeService) Execute(ctx context.Context, streamData dto.StreamDataStockAnalyzer) error {

	stockDataMultiTimeframe, err := s.yahooFinance.GetMultiTimeframe(ctx, streamData.StockCode)
	if err != nil {
		s.log.Error("Failed to get stock data multi timeframe", logger.ErrorField(err))
		return err
	}

	lastSummary, err := s.stockNewsSummaryRepo.GetLast(ctx, time.Now().Add(-time.Hour*24), streamData.StockCode)
	if err != nil {
		s.log.Error("Failed to get last stock news summary", logger.ErrorField(err))
		return err
	}

	geminiResp, err := s.aiRepo.AnalyzeStockMultiTimeframe(ctx, streamData.StockCode, stockDataMultiTimeframe, lastSummary)
	if err != nil {
		s.log.Error("Failed to analyze stock", logger.ErrorField(err))
		return err
	}

	dataJSON, err := json.Marshal(geminiResp)
	if err != nil {
		s.log.Error("Failed to marshal gemini response", logger.ErrorField(err))
		return err
	}

	err = s.stockSignalRepo.Create(ctx, &entity.StockSignal{
		StockCode:       streamData.StockCode,
		Signal:          geminiResp.Action,
		ConfidenceScore: float64(geminiResp.ConfidenceLevel),
		NewsScore:       geminiResp.NewsSummary.ConfidenceScore,
		Data:            dataJSON,
		TechnicalScore:  geminiResp.TechnicalScore,
	})

	if err != nil {
		s.log.Error("Failed to create stock signal", logger.ErrorField(err))
		return err
	}

	if streamData.NotifyUser {
		msgCfg := tgbotapi.MessageConfig{
			ParseMode: tgbotapi.ModeHTML,
		}
		if err := s.telegramBot.SendMessageUser(telegram.FormatAnalysisMessage(geminiResp), streamData.TelegramID, msgCfg); err != nil {
			s.log.Error("Failed to send notification", logger.ErrorField(err))
		}
	}

	return nil
}

func (s *stockAnalyzerMultiTimeframeService) AckNDel(ctx context.Context, streamName string, messageID string) error {
	if err := s.redisClient.XAck(ctx, streamName, common.RedisStreamGroup, messageID).Err(); err != nil {
		s.log.Error("Failed to acknowledge stock analyzer task on retry", logger.ErrorField(err), logger.Field("message_id", messageID))
		return err
	}
	if err := s.redisClient.XDel(ctx, streamName, messageID).Err(); err != nil {
		s.log.Error("Failed to delete stock analyzer task on retry", logger.ErrorField(err), logger.Field("message_id", messageID))
		return err
	}
	return nil
}

func (s *stockAnalyzerMultiTimeframeService) ProcessRetries(ctx context.Context) {
	msgs, _, err := s.redisClient.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   common.RedisStreamStockAnalyzer,
		Group:    common.RedisStreamGroup,
		Consumer: common.RedisStreamConsumer + "-retry",
		MinIdle:  s.cfg.Executor.RedisStreamStockAnalyzerMaxIdleDuration,
		Start:    "0",
		Count:    1,
	}).Result()

	if err != nil {
		s.log.Error("Failed to claim stock analyzer task on retry", logger.ErrorField(err))
		return
	}

	if len(msgs) == 0 {
		s.log.Debug("Retry No pending messages found", logger.StringField("stream", common.RedisStreamStockAnalyzer))
		return
	}

	s.log.Info("Found pending messages", logger.StringField("stream", common.RedisStreamStockAnalyzer))

	pendingInfo, err := s.redisClient.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: common.RedisStreamStockAnalyzer,
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

	var streamData dto.StreamDataStockAnalyzer
	if err := json.Unmarshal([]byte(taskData), &streamData); err != nil {
		s.log.Error("Failed to unmarshal task data", logger.ErrorField(err), logger.Field("message_id", msg.ID))
		return
	}

	if err := s.Execute(ctx, streamData); err != nil {
		s.log.Error("Failed to analyze stock", logger.ErrorField(err), logger.Field("message_id", msg.ID), logger.StringField("stock_code", streamData.StockCode))

		if pendingInfo[0].RetryCount+1 >= int64(s.cfg.Executor.RedisStreamStockAnalyzerMaxRetry) {
			s.log.Error("pending msg retry count exceeded",
				logger.StringField("stream", common.RedisStreamStockAnalyzer),
				logger.StringField("message_id", msg.ID),
				logger.StringField("stock_code", streamData.StockCode),
				logger.IntField("retry_count", int(pendingInfo[0].RetryCount+1)),
				logger.IntField("max_retry", s.cfg.Executor.RedisStreamStockAnalyzerMaxRetry),
			)
			errType := fmt.Sprintf("Retry count exceeded for event %s", common.RedisStreamStockAnalyzer)
			rawJson, _ := json.Marshal(streamData)
			msgTelegram := telegram.FormatErrorAlertMessage(utils.TimeNowWIB(), errType, err.Error(), string(rawJson))
			if err := s.telegramBot.SendMessage(msgTelegram); err != nil {
				s.log.Error("Failed to send telegram message retry exceeded ", logger.ErrorField(err), logger.StringField("stock_code", streamData.StockCode))
			}
			if err := s.AckNDel(ctx, common.RedisStreamStockAnalyzer, msg.ID); err != nil {
				s.log.Error("Failed to acknowledge and delete stock analyzer task", logger.ErrorField(err), logger.Field("message_id", msg.ID))
				return
			}
			return
		}

		return
	}

	if err := s.AckNDel(ctx, common.RedisStreamStockAnalyzer, msg.ID); err != nil {
		s.log.Error("Failed to acknowledge and delete stock analyzer task", logger.ErrorField(err), logger.Field("message_id", msg.ID))
		return
	}
	s.log.Info("Retry Stock analyzer task processed successfully", logger.StringField("stock_code", streamData.StockCode))
}

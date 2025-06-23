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
)

type StockAnalyzerService interface {
	ProcessTask(ctx context.Context)
	ProcessRetries(ctx context.Context)
	Analyze(ctx context.Context, stockCode string, interval string, rangeData string) error
}

type stockAnalyzerService struct {
	cfg                  *config.Config
	log                  *logger.Logger
	redisClient          *redis.Client
	geminiRepoAI         repository.GeminiAIRepository
	yahooFinance         repository.YahooFinanceRepository
	stockNewsSummaryRepo repository.StockNewsSummaryRepository
	stockSignalRepo      repository.StockSignalRepository
	telegramBot          telegram.Notifier
}

func NewStockAnalyzerService(cfg *config.Config, log *logger.Logger,
	redisClient *redis.Client,
	geminiRepoAI repository.GeminiAIRepository,
	yahooFinance repository.YahooFinanceRepository,
	stockNewsSummaryRepo repository.StockNewsSummaryRepository,
	stockSignalRepo repository.StockSignalRepository,
	telegramBot telegram.Notifier) StockAnalyzerService {
	return &stockAnalyzerService{
		cfg:                  cfg,
		log:                  log,
		redisClient:          redisClient,
		geminiRepoAI:         geminiRepoAI,
		yahooFinance:         yahooFinance,
		stockNewsSummaryRepo: stockNewsSummaryRepo,
		stockSignalRepo:      stockSignalRepo,
		telegramBot:          telegramBot,
	}
}

func (s *stockAnalyzerService) ProcessTask(ctx context.Context) {
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

	s.log.Debug("Processing stock analyzer task", logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))

	if err := s.Analyze(ctx, streamData.StockCode, streamData.Interval, streamData.Range); err != nil {
		s.log.Error("Failed to analyze stock", logger.ErrorField(err), logger.Field("message_id", message.ID), logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))
		return
	}
	if err := s.AckNDel(ctx, common.RedisStreamStockAnalyzer, message.ID); err != nil {
		s.log.Error("Failed to acknowledge and delete stock analyzer task", logger.ErrorField(err), logger.Field("message_id", message.ID))
		return
	}

	s.log.Debug("Stock analyzer task processed successfully", logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))

}

func (s *stockAnalyzerService) Analyze(ctx context.Context, stockCode string, interval string, rangeData string) error {
	stockData, err := s.yahooFinance.Get(ctx, dto.GetStockDataParam{
		StockCode: stockCode,
		Interval:  interval,
		Range:     rangeData,
	})
	if err != nil {
		s.log.Error("Failed to get stock data", logger.ErrorField(err))
		return err
	}

	lastSummary, err := s.stockNewsSummaryRepo.GetLast(ctx, time.Now().Add(-time.Hour*24), stockCode)
	if err != nil {
		s.log.Error("Failed to get last stock news summary", logger.ErrorField(err))
		return err
	}

	geminiResp, err := s.geminiRepoAI.AnalyzeStock(ctx, stockCode, stockData, lastSummary)
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
		StockCode:       stockCode,
		Interval:        interval,
		Range:           rangeData,
		Signal:          geminiResp.Recommendation.Action,
		ConfidenceScore: float64(geminiResp.Recommendation.ConfidenceLevel),
		TechnicalScore:  geminiResp.TechnicalAnalysis.TechnicalScore,
		NewsScore:       geminiResp.NewsSummary.ConfidenceScore,
		Data:            dataJSON,
	})

	if err != nil {
		s.log.Error("Failed to create stock signal", logger.ErrorField(err))
		return err
	}

	return nil
}

func (s *stockAnalyzerService) AckNDel(ctx context.Context, streamName string, messageID string) error {
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

func (s *stockAnalyzerService) ProcessRetries(ctx context.Context) {
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

	if pendingInfo[0].RetryCount >= int64(s.cfg.Executor.RedisStreamStockAnalyzerMaxRetry) {
		s.log.Error("pending msg retry count exceeded",
			logger.StringField("stream", common.RedisStreamStockAnalyzer),
			logger.StringField("message_id", msg.ID),
			logger.StringField("stock_code", streamData.StockCode),
			logger.StringField("interval", streamData.Interval),
			logger.StringField("range", streamData.Range),
			logger.IntField("retry_count", int(pendingInfo[0].RetryCount)),
			logger.IntField("max_retry", s.cfg.Executor.RedisStreamStockAnalyzerMaxRetry),
		)
		msgTelegram := telegram.FormatErrorAlertMessage(utils.TimeNowWIB(), fmt.Sprintf("Stock analyzer task retry count exceeded for stock %s, interval %s, range %s", streamData.StockCode, streamData.Interval, streamData.Range))
		if err := s.telegramBot.SendMessage(msgTelegram); err != nil {
			s.log.Error("Failed to send telegram message retry exceeded ", logger.ErrorField(err), logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))
		}
		if err := s.AckNDel(ctx, common.RedisStreamStockAnalyzer, msg.ID); err != nil {
			s.log.Error("Failed to acknowledge and delete stock analyzer task", logger.ErrorField(err), logger.Field("message_id", msg.ID))
			return
		}
		return
	}

	if err := s.Analyze(ctx, streamData.StockCode, streamData.Interval, streamData.Range); err != nil {
		s.log.Error("Failed to analyze stock", logger.ErrorField(err), logger.Field("message_id", msg.ID), logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))
		return
	}

	if err := s.AckNDel(ctx, common.RedisStreamStockAnalyzer, msg.ID); err != nil {
		s.log.Error("Failed to acknowledge and delete stock analyzer task", logger.ErrorField(err), logger.Field("message_id", msg.ID))
		return
	}
	s.log.Info("Retry Stock analyzer task processed successfully", logger.StringField("stock_code", streamData.StockCode), logger.StringField("interval", streamData.Interval), logger.StringField("range", streamData.Range))
}

package consumer

import (
	"context"
	"sync"
	"time"

	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/service"
	"golang-stock-scryper/pkg/common"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/utils"

	"github.com/redis/go-redis/v9"
)

// RedisConsumer manages the consumption of tasks from a Redis stream.
type RedisConsumer struct {
	cfg                            *config.Config
	redisClient                    *redis.Client
	executorService                service.ExecutorService
	stockAnalyzerService           service.StockAnalyzerService
	stockPositionMonitoringService service.StockPositionMonitoringService
	logger                         *logger.Logger
	stopChan                       chan struct{}
	wg                             sync.WaitGroup
}

// NewRedisConsumer creates a new RedisConsumer.
func NewRedisConsumer(
	cfg *config.Config,
	redisClient *redis.Client,
	executorService service.ExecutorService,
	stockAnalyzerService service.StockAnalyzerService,
	stockPositionMonitoringService service.StockPositionMonitoringService,
	log *logger.Logger,
) *RedisConsumer {
	return &RedisConsumer{
		cfg:                            cfg,
		redisClient:                    redisClient,
		executorService:                executorService,
		stockAnalyzerService:           stockAnalyzerService,
		stockPositionMonitoringService: stockPositionMonitoringService,
		logger:                         log,
		stopChan:                       make(chan struct{}),
	}
}

// Start begins the consumer's task processing loop.
func (c *RedisConsumer) Start(ctx context.Context) {
	c.logger.Info("Redis consumer started")
	c.RegisterStreamHandler(ctx, c.executorService.ProcessTask, common.RedisStreamSchedulerTaskExecution, c.cfg.Executor.RedisStreamTaskExecutionTimeout)
	c.RegisterStreamHandler(ctx, c.stockAnalyzerService.ProcessTask, common.RedisStreamStockAnalyzer, c.cfg.Executor.RedisStreamStockAnalyzerTimeout)
	c.RegisterStreamHandler(ctx, c.stockPositionMonitoringService.ProcessTask, common.RedisStreamStockPositionMonitor, c.cfg.Executor.RedisStreamStockPositionMonitorTimeout)

	//handle retry
	c.RegisterTickerHandler(ctx, c.stockAnalyzerService.ProcessRetries, c.cfg.Executor.RedisStreamStockAnalyzerRetryInterval, c.cfg.Executor.RedisStreamStockAnalyzerMaxIdleDuration, common.RedisStreamStockAnalyzer+"-retry")
	c.RegisterTickerHandler(ctx, c.stockPositionMonitoringService.ProcessRetries, c.cfg.Executor.RedisStreamStockPositionMonitorRetryInterval, c.cfg.Executor.RedisStreamStockPositionMonitorMaxIdleDuration, common.RedisStreamStockPositionMonitor+"-retry")
}

func (c *RedisConsumer) RegisterStreamHandler(ctx context.Context, fn func(ctx context.Context), streamName string, timeout time.Duration) {
	c.logger.Info("Registering stream handler", logger.Field("stream", streamName))
	c.wg.Add(1)
	utils.GoSafe(func() {
		defer c.wg.Done()
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("Redis consumer stopping due to context cancellation")
				return
			case <-c.stopChan:
				c.logger.Info("Redis consumer stopping")
				return
			default:
				ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()
				fn(ctxTimeout)
			}

		}

	})
}

func (c *RedisConsumer) RegisterTickerHandler(ctx context.Context, fn func(ctx context.Context), interval time.Duration, timeout time.Duration, name string) {
	c.logger.Info("Registering ticker handler",
		logger.Field("name", name),
		logger.Field("interval", interval),
		logger.Field("timeout", timeout))
	c.wg.Add(1)
	utils.GoSafe(func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
				fn(ctxTimeout)
				cancel()
			case <-ctx.Done():
				c.logger.Info("Ticker handler stopping due to context cancellation", logger.Field("name", name))
				return
			case <-c.stopChan:
				c.logger.Info("Ticker handler stopping", logger.Field("name", name))
				return
			}
		}
	})
}

// Stop gracefully shuts down the consumer.
func (c *RedisConsumer) Stop() {
	close(c.stopChan)
	c.wg.Wait()
	c.logger.Info("Redis consumer stopped")
}

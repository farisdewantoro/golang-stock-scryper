package consumer

import (
	"context"
	"sync"

	"golang-stock-scryper/internal/executor/service"
	"golang-stock-scryper/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// RedisConsumer manages the consumption of tasks from a Redis stream.
type RedisConsumer struct {
	redisClient     *redis.Client
	executorService service.ExecutorService
	logger          *logger.Logger
	stopChan        chan struct{}
	wg              sync.WaitGroup
}

// NewRedisConsumer creates a new RedisConsumer.
func NewRedisConsumer(
	redisClient *redis.Client,
	executorService service.ExecutorService,
	log *logger.Logger,
) *RedisConsumer {
	return &RedisConsumer{
		redisClient:     redisClient,
		executorService: executorService,
		logger:          log,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the consumer's task processing loop.
func (c *RedisConsumer) Start(ctx context.Context) {
	c.logger.Info("Redis consumer started")
	c.wg.Add(1)
	go func() {
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
			}

			// ProcessTask will block for a short time, allowing the loop
			// to remain responsive to shutdown signals.
			c.executorService.ProcessTask(ctx)
		}
	}()
}

// Stop gracefully shuts down the consumer.
func (c *RedisConsumer) Stop() {
	close(c.stopChan)
	c.wg.Wait()
	c.logger.Info("Redis consumer stopped")
}

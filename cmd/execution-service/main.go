package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/delivery/consumer"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/internal/executor/service"
	"golang-stock-scryper/internal/executor/strategy"
	"golang-stock-scryper/pkg/common"
	"golang-stock-scryper/pkg/decoder"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/postgres"
	"golang-stock-scryper/pkg/redis"
	"golang-stock-scryper/pkg/telegram"

	"google.golang.org/genai"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var configPath string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the execution service",
	Run:   runServe,
}

func runServe(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.New(cfg.Logger.Level, cfg.Logger.Encoding)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = appLogger.Sync() }()

	appLogger.Info("Starting Execution Service", zap.String("name", cfg.App.Name))

	// Initialize database
	postgresCfg := postgres.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		DBName:          cfg.Database.DBName,
		SSLMode:         cfg.Database.SSLMode,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}
	db, err := postgres.NewDB(postgresCfg)
	if err != nil {
		appLogger.Fatal("Failed to initialize database", zap.Error(err))
	}
	if sqlDB, err := db.DB.DB(); err == nil {
		defer sqlDB.Close()
	}

	// Initialize Redis
	redisCfg := redis.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	}
	redisClient, err := redis.NewClient(redisCfg)
	if err != nil {
		appLogger.Fatal("Failed to initialize Redis", zap.Error(err))
	}
	defer redisClient.Close()

	// Create the consumer group if it doesn't exist
	// MKSTREAM creates the stream if it doesn't exist
	if err := redisClient.XGroupCreateMkStream(context.Background(), common.RedisStreamSchedulerTaskExecution, common.RedisStreamGroup, "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			appLogger.Fatal("Failed to create consumer group", logger.ErrorField(err))
		}
	}
	if err := redisClient.XGroupCreateMkStream(context.Background(), common.RedisStreamStockAnalyzer, common.RedisStreamGroup, "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			appLogger.Fatal("Failed to create consumer group", logger.ErrorField(err))
		}
	}
	if err := redisClient.XGroupCreateMkStream(context.Background(), common.RedisStreamStockPositionMonitor, common.RedisStreamGroup, "0").Err(); err != nil {
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			appLogger.Fatal("Failed to create consumer group", logger.ErrorField(err))
		}
	}

	// Initialize repositories
	jobRepo := repository.NewJobRepository(db.DB)
	historyRepo := repository.NewTaskExecutionHistoryRepository(db.DB)
	stockMentionRepo := repository.NewStockMentionRepository(db.DB, appLogger)
	stockNewsRepo := repository.NewStockNewsRepository(db.DB)
	stockNewsSummaryRepo := repository.NewStockNewsSummaryRepository(db.DB)
	stockPositionsRepo := repository.NewStockPositionsRepository(db.DB)
	stocksRepo := repository.NewStocksRepository(db.DB)
	yahooFinanceRepo, err := repository.NewYahooFinanceRepository(cfg, appLogger)
	stockSignalRepo := repository.NewStockSignalRepository(db.DB)
	stockPositionMonitoringRepo := repository.NewStockPositionsMonitoringsRepository(db.DB)

	if err != nil {
		appLogger.Fatal("Failed to initialize Yahoo Finance repository", zap.Error(err))
	}

	// Initialize AI provider
	var aiRepo repository.AIRepository
	switch cfg.AI.Provider {
	case "gemini":
		genAiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey: cfg.Gemini.APIKey,
		})
		if err != nil {
			appLogger.Fatal("Failed to initialize Gemini AI client", zap.Error(err))
		}
		repo, err := repository.NewGeminiAIRepository(cfg, appLogger, genAiClient)
		if err != nil {
			appLogger.Fatal("Failed to initialize Gemini AI repository", zap.Error(err))
		}
		aiRepo = repo
	default:
		appLogger.Fatal("Invalid AI provider specified in config", zap.String("provider", cfg.AI.Provider))
	}

	telegramNotifier, err := telegram.NewClient(cfg.Telegram.BotToken, cfg.Telegram.ChatID)
	if err != nil {
		appLogger.Fatal("Failed to initialize Telegram notifier", zap.Error(err))
	}

	// Initialize decoder
	decoder := decoder.NewGoogleDecoder(appLogger)

	// Initialize Strategies
	strategies := []strategy.JobExecutionStrategy{
		strategy.NewHTTPStrategy(appLogger),
		strategy.NewStockNewsScraperStrategy(
			db.DB,
			appLogger,
			decoder,
			aiRepo,
			stockMentionRepo,
			stockNewsRepo,
			stocksRepo,
		),
		strategy.NewStockPriceAlertStrategy(
			appLogger,
			yahooFinanceRepo,
			telegramNotifier,
			stockPositionsRepo,
			redisClient,
		),
		strategy.NewStockAnalyzerStrategy(appLogger, redisClient, stocksRepo),
		strategy.NewStockNewsSummaryStrategy(
			db.DB,
			appLogger,
			stocksRepo,
			stockNewsRepo,
			stockNewsSummaryRepo,
			aiRepo,
			telegramNotifier,
		),
		strategy.NewStockPositionMonitorStrategy(
			appLogger,
			redisClient,
			stockPositionsRepo,
		),
	}

	// Initialize executor service
	executorSvc := service.NewExecutorService(cfg, redisClient.Client, jobRepo, historyRepo, appLogger, strategies)
	stockAnalyzerMultiTimeframeSvc := service.NewStockAnalyzerMultiTimeframeService(cfg, appLogger, redisClient.Client, aiRepo, yahooFinanceRepo, stockNewsSummaryRepo, stockSignalRepo, telegramNotifier)
	stockPositionMonitoringSvc := service.NewStockPositionMonitoringMultiTimeframeService(cfg, appLogger, redisClient.Client, aiRepo, yahooFinanceRepo, stockPositionsRepo, stockNewsSummaryRepo, stockPositionMonitoringRepo, telegramNotifier)

	// Initialize and start the Redis consumer
	redisConsumer := consumer.NewRedisConsumer(cfg, redisClient.Client, executorSvc, stockAnalyzerMultiTimeframeSvc, stockPositionMonitoringSvc, appLogger)
	redisConsumer.Start(ctx)

	appLogger.Info("Execution service started. Waiting for tasks...")

	// Wait for interrupt signal to gracefully shut down the service
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down execution service...")
	cancel()
	redisConsumer.Stop()
	appLogger.Info("Execution service stopped.")
}

func main() {
	rootCmd := &cobra.Command{Use: "execution-service"}

	serveCmd.Flags().StringVarP(&configPath, "config", "c", "configs/config-executor.yaml", "Path to the configuration file")

	rootCmd.AddCommand(serveCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing execution-service CLI: %s\n", err)
		os.Exit(1)
	}
}

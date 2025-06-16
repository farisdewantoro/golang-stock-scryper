package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang-stock-scryper/internal/scheduler/config"
	delivery "golang-stock-scryper/internal/scheduler/delivery/http"
	_ "golang-stock-scryper/internal/scheduler/docs"
	"golang-stock-scryper/internal/scheduler/repository"
	"golang-stock-scryper/internal/scheduler/service"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/postgres"
	"golang-stock-scryper/pkg/redis"

	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
	swagger "github.com/swaggo/echo-swagger"
)

var configPath string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts the scheduling service",
	Run:   runServe,
}

func runServe(cmd *cobra.Command, args []string) {
	// Create a context that is canceled on interrupt signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	appLogger.Info("Starting Scheduling Service", logger.Field("name", cfg.App.Name))

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
		appLogger.Fatal("Failed to initialize database", logger.ErrorField(err))
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
		appLogger.Fatal("Failed to initialize Redis", logger.ErrorField(err))
	}
	defer redisClient.Close()

	// Initialize repositories
	jobRepo := repository.NewJobRepository(db.DB)
	scheduleRepo := repository.NewTaskScheduleRepository(db.DB)
	historyRepo := repository.NewTaskExecutionHistoryRepository(db.DB)

	// Initialize services
	pollingInterval, err := time.ParseDuration(cfg.Scheduler.PollingInterval)
	if err != nil {
		appLogger.Fatal("Invalid polling interval", logger.ErrorField(err))
	}
	schedulerSvc := service.NewSchedulerService(jobRepo, scheduleRepo, historyRepo, redisClient.Client, appLogger, pollingInterval, cfg)
	jobSvc := service.NewJobService(jobRepo, appLogger)
	scheduleSvc := service.NewScheduleService(scheduleRepo, appLogger)
	historySvc := service.NewExecutionHistoryService(historyRepo, appLogger)

	// Start scheduler service
	go schedulerSvc.Start(ctx)

	// Initialize Echo server
	e := echo.New()
	e.HideBanner = true

	// Initialize handlers and routes
	jobHandler := delivery.NewJobHandler(jobSvc, appLogger)
	apiV1 := e.Group("/api/v1")
	jobsGroup := apiV1.Group("/jobs")
	jobHandler.RegisterRoutes(jobsGroup)

	scheduleHandler := delivery.NewScheduleHandler(scheduleSvc, appLogger)
	schedulesGroup := apiV1.Group("/schedules")
	scheduleHandler.RegisterRoutes(schedulesGroup)

	historyHandler := delivery.NewExecutionHistoryHandler(historySvc, appLogger)
	executionsGroup := apiV1.Group("/executions")
	historyHandler.RegisterRoutes(executionsGroup)
	historyHandler.RegisterJobRoutes(jobsGroup)

	e.GET("/swagger/*", swagger.WrapHandler)

	// Start server
	go func() {
		addr := fmt.Sprintf(":%d", cfg.API.Port)
		appLogger.Info("HTTP server starting", logger.Field("address", addr))
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			appLogger.Error("HTTP server failed to start", logger.ErrorField(err))
			stop() // trigger shutdown
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	appLogger.Info("Shutting down server...")

	// Gracefully shutdown the server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		appLogger.Fatal("Server forced to shutdown", logger.ErrorField(err))
	}

	appLogger.Info("Server exiting")
}

// @title Job Scheduler API
// @version 1.0
// @description This is a sample server for a job scheduler.
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /api/v1
func main() {
	rootCmd := &cobra.Command{Use: "scheduling-service"}

	serveCmd.Flags().StringVarP(&configPath, "config", "c", "configs/config-scheduler.yaml", "Path to the configuration file")

	rootCmd.AddCommand(serveCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing scheduling-service CLI: %s\n", err)
		os.Exit(1)
	}
}

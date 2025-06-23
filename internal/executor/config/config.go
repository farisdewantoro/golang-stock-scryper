package config

import (
	"golang-stock-scryper/pkg/config"
	"time"
)

// Executor holds executor-specific configuration.
type Executor struct {
	MaxConcurrentTasks              int           `mapstructure:"max_concurrent_tasks"`
	RedisStreamTaskExecutionTimeout time.Duration `mapstructure:"redis_stream_task_execution_timeout"`

	// Stock Analyzer
	RedisStreamStockAnalyzerTimeout         time.Duration `mapstructure:"redis_stream_stock_analyzer_timeout"`
	RedisStreamStockAnalyzerRetryInterval   time.Duration `mapstructure:"redis_stream_stock_analyzer_retry_interval"`
	RedisStreamStockAnalyzerMaxIdleDuration time.Duration `mapstructure:"redis_stream_stock_analyzer_max_idle_duration"`
	RedisStreamStockAnalyzerMaxRetry        int           `mapstructure:"redis_stream_stock_analyzer_max_retry"`

	// Stock Position Monitoring
	RedisStreamStockPositionMonitorTimeout         time.Duration `mapstructure:"redis_stream_stock_position_monitor_timeout"`
	RedisStreamStockPositionMonitorRetryInterval   time.Duration `mapstructure:"redis_stream_stock_position_monitor_retry_interval"`
	RedisStreamStockPositionMonitorMaxIdleDuration time.Duration `mapstructure:"redis_stream_stock_position_monitor_max_idle_duration"`
	RedisStreamStockPositionMonitorMaxRetry        int           `mapstructure:"redis_stream_stock_position_monitor_max_retry"`
}

// OpenRouter holds the configuration for the OpenRouter API.
type OpenRouter struct {
	APIKey string `mapstructure:"api_key"`
	Model  string `mapstructure:"model"`
}

// Gemini holds the configuration for the Gemini API.
type Gemini struct {
	APIKey              string `mapstructure:"api_key"`
	Model               string `mapstructure:"model"`
	MaxRequestPerMinute int    `mapstructure:"max_request_per_minute"`
	MaxTokenPerMinute   int    `mapstructure:"max_token_per_minute"`
}

// AI holds configuration for AI providers.
type AI struct {
	Provider string `mapstructure:"provider"`
}

// Telegram holds configuration for the Telegram notifier.
type Telegram struct {
	BotToken string `mapstructure:"bot_token"`
	ChatID   int64  `mapstructure:"chat_id"`
}

// YahooFinance holds the configuration for the Yahoo Finance API.
type YahooFinance struct {
	BaseURL             string `mapstructure:"base_url"`
	MaxRequestPerMinute int    `mapstructure:"max_request_per_minute"`
}

// Config holds the full configuration for the executor service.
type Config struct {
	App          config.App      `mapstructure:"app"`
	Logger       config.Logger   `mapstructure:"logger"`
	Database     config.Database `mapstructure:"database"`
	Redis        config.Redis    `mapstructure:"redis"`
	Executor     Executor        `mapstructure:"executor"`
	OpenRouter   OpenRouter      `mapstructure:"openrouter"`
	Gemini       Gemini          `mapstructure:"gemini"`
	AI           AI              `mapstructure:"ai"`
	Telegram     Telegram        `mapstructure:"telegram"`
	YahooFinance YahooFinance    `mapstructure:"yahoo_finance"`
}

// Load loads the executor configuration from the given path.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := config.Load(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

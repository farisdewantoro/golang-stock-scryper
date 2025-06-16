package config

import (
	"golang-stock-scryper/pkg/config"
)

// Executor holds executor-specific configuration.
type Executor struct {
	MaxConcurrentTasks int    `mapstructure:"max_concurrent_tasks"`
	DefaultTaskTimeout string `mapstructure:"default_task_timeout"`
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

// Config holds the full configuration for the executor service.
type Config struct {
	App        config.App      `mapstructure:"app"`
	Logger     config.Logger   `mapstructure:"logger"`
	Database   config.Database `mapstructure:"database"`
	Redis      config.Redis    `mapstructure:"redis"`
	Executor   Executor        `mapstructure:"executor"`
	OpenRouter OpenRouter      `mapstructure:"openrouter"`
	Gemini     Gemini          `mapstructure:"gemini"`
	AI         AI              `mapstructure:"ai"`
	Telegram   Telegram        `mapstructure:"telegram"`
}

// Load loads the executor configuration from the given path.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := config.Load(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

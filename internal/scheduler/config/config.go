package config

import (
	"golang-stock-scryper/pkg/config"
)

// Scheduler holds scheduler-specific configuration.
type Scheduler struct {
	PollingInterval   string `mapstructure:"polling_interval"`
	MaxConcurrentJobs int    `mapstructure:"max_concurrent_jobs"`
	DefaultTimeout    string `mapstructure:"default_timeout"`
}

// Config holds the full configuration for the scheduler service.
type Config struct {
	App       config.App      `mapstructure:"app"`
	Logger    config.Logger   `mapstructure:"logger"`
	Database  config.Database `mapstructure:"database"`
	Redis     config.Redis    `mapstructure:"redis"`
	API       config.API      `mapstructure:"api"`
	Scheduler Scheduler       `mapstructure:"scheduler"`
}

// Load loads the scheduler configuration from the given path.
func Load(path string) (*Config, error) {
	var cfg Config
	if err := config.Load(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

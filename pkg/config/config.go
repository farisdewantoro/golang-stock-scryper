package config

import (
	"log"
	"strings"

	"github.com/spf13/viper"
)

// App holds application configuration.
type App struct {
	Name    string `mapstructure:"name"`
	Env     string `mapstructure:"env"`
	Version string `mapstructure:"version"`
}

// Logger holds logger configuration.
type Logger struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

// Database holds database configuration.
type Database struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"name"`
	SSLMode         string `mapstructure:"ssl_mode"`
	TimeZone        string `mapstructure:"time_zone"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`
	LogLevel        string `mapstructure:"log_level"`
}

// Redis holds Redis configuration.
type Redis struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	StreamMaxLen int64  `mapstructure:"stream_max_len"`
}

// API holds API server configuration.
type API struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// Load loads configuration from a file into the given config struct.
func Load(path string, config interface{}) error {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Println("Failed to read config file .env config try read from environment variables")
	}

	return viper.Unmarshal(config)
}

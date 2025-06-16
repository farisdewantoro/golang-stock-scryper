package postgres

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Config holds the configuration for the PostgreSQL database connection.
type Config struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	DBName          string `yaml:"name"`
	SSLMode         string `yaml:"ssl_mode"`
	TimeZone        string `yaml:"time_zone"` // e.g., "Asia/Shanghai"
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"` // e.g., "5m"
	LogLevel        string `yaml:"log_level"`         // GORM logger level: Silent, Error, Warn, Info
}

// DB is a wrapper around the gorm.DB client for PostgreSQL.
type DB struct {
	*gorm.DB
}

// NewDB creates a new GORM database connection instance.
func NewDB(cfg Config) (*DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode)
	if cfg.TimeZone != "" {
		dsn += fmt.Sprintf(" TimeZone=%s", cfg.TimeZone)
	}

	var gormLogLevel gormlogger.LogLevel
	switch cfg.LogLevel {
	case "Silent":
		gormLogLevel = gormlogger.Silent
	case "Error":
		gormLogLevel = gormlogger.Error
	case "Warn":
		gormLogLevel = gormlogger.Warn
	case "Info":
		gormLogLevel = gormlogger.Info
	default:
		gormLogLevel = gormlogger.Warn // Default to Warn
	}

	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormLogLevel),
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database using GORM: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB from GORM: %w", err)
	}

	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime != "" {
		duration, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err != nil {
			// Attempt to close the connection if parsing fails to prevent resource leaks
			_ = sqlDB.Close()
			return nil, fmt.Errorf("invalid connection max lifetime format '%s': %w", cfg.ConnMaxLifetime, err)
		}
		sqlDB.SetConnMaxLifetime(duration)
	}

	// Ping is not strictly necessary with GORM as Open usually verifies connection,
	// but can be kept for explicit check if desired.
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL at %s:%d: %w", cfg.Host, cfg.Port, err)
	}

	return &DB{db}, nil
}

// Close closes the database connection.
// For GORM, this typically closes the underlying *sql.DB connection pool.
func (d *DB) Close() error {
	if d.DB != nil {
		sqlDB, err := d.DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get underlying sql.DB from GORM for closing: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}

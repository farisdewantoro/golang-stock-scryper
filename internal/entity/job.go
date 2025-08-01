package entity

import (
	"time"

	"gorm.io/datatypes"
)

type JobType string

const (
	JobTypeHTTP                 JobType = "http_request"
	JobTypeStockNewsScraper     JobType = "stock_news_scraper"
	JobTypeStockNewsSummary     JobType = "stock_news_summary"
	JobTypeStockPriceAlert      JobType = "stock_price_alert"
	JobTypeStockAnalyzer        JobType = "stock_analyzer"
	JobTypeStockPositionMonitor JobType = "stock_position_monitor"
)

type Job struct {
	ID          uint                   `gorm:"primaryKey"`
	Name        string                 `gorm:"type:varchar(255);not null"`
	Description string                 `gorm:"type:text"`
	Type        JobType                `gorm:"type:varchar(50);not null"`
	Payload     datatypes.JSON         `gorm:"type:jsonb;not null"`
	RetryPolicy datatypes.JSON         `gorm:"type:jsonb"`
	Timeout     int                    `gorm:"default:60"`
	CreatedAt   time.Time              `gorm:"autoCreateTime"`
	UpdatedAt   time.Time              `gorm:"autoUpdateTime"`
	Schedules   []TaskSchedule         `gorm:"foreignKey:JobID"`
	Histories   []TaskExecutionHistory `gorm:"foreignKey:JobID"`
}

func (Job) TableName() string {
	return "jobs"
}

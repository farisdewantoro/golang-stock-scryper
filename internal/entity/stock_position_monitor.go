package entity

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StockPositionMonitoring struct {
	ID              int64          `json:"id"`
	UserID          uint           `json:"user_id"`
	StockPositionID uint           `json:"stock_position_id"`
	Signal          string         `json:"signal"`
	ConfidenceScore float64        `json:"confidence_score"`
	TechnicalScore  float64        `json:"technical_score"`
	NewsScore       float64        `json:"news_score"`
	Interval        string         `json:"interval"`
	Range           string         `json:"range"`
	TriggeredAlert  bool           `json:"triggered_alert"`
	Data            datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"deleted_at"`
	StockPosition   StockPosition  `gorm:"foreignKey:StockPositionID"`
	User            User           `gorm:"foreignKey:UserID"`
}

func (StockPositionMonitoring) TableName() string {
	return "stock_position_monitorings"
}

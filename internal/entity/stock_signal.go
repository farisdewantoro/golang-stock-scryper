package entity

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StockSignal struct {
	ID              int64          `json:"id"`
	StockCode       string         `json:"stock_code"`
	Signal          string         `json:"signal"`
	ConfidenceScore float64        `json:"confidence_score"`
	TechnicalScore  int            `json:"technical_score"`
	NewsScore       float64        `json:"news_score"`
	Data            datatypes.JSON `gorm:"type:jsonb"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"deleted_at"`
}

func (StockSignal) TableName() string {
	return "stock_signals"
}

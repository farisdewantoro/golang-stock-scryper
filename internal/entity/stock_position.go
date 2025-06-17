package entity

import "time"

type StockPosition struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	StockCode            string    `gorm:"not null" json:"stock_code"`
	BuyPrice             float64   `gorm:"not null" json:"buy_price"`
	TakeProfitPrice      float64   `gorm:"not null" json:"take_profit_price"`
	StopLossPrice        float64   `gorm:"not null" json:"stop_loss_price"`
	BuyDate              time.Time `gorm:"not null" json:"buy_date"`
	MaxHoldingPeriodDays int       `gorm:"not null" json:"max_holding_period_days"`
	IsAlertTriggered     bool      `gorm:"not null" json:"is_alert_triggered"`
	LastAlertedAt        time.Time `gorm:"not null" json:"last_alerted_at"`
	CreatedAt            time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (StockPosition) TableName() string {
	return "stock_positions"
}

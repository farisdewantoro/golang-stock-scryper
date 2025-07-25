package dto

import (
	"golang-stock-scryper/internal/entity"
)

// NewsAnalysisResult represents the structured result from the news analysis API.
type NewsAnalysisResult struct {
	Summary       string                `json:"summary"`
	ImpactScore   float64               `json:"impact_score"`
	KeyIssue      []string              `json:"key_issue"`
	StockMentions []entity.StockMention `json:"stock_mentions"`
}

type StreamDataStockAnalyzer struct {
	StockCode  string `json:"stock_code"`
	TelegramID int64  `json:"telegram_id"`
	NotifyUser bool   `json:"notify_user"`
}

type StreamDataStockPositionMonitor struct {
	StockPositionID uint   `json:"stock_position_id"`
	UserID          uint   `json:"user_id"`
	StockCode       string `json:"stock_code"`
	SendToTelegram  bool   `json:"send_to_telegram"`
}

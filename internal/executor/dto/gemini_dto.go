package dto

import "time"

// GeminiAPIRequest is the request payload for the Gemini API.
type GeminiAPIRequest struct {
	Contents []Content `json:"contents"`
}

// Content represents the content of a request or response.
type Content struct {
	Parts []Part `json:"parts"`
}

// Part is a part of the content.
type Part struct {
	Text string `json:"text"`
}

// GeminiAPIResponse is the response from the Gemini API.
type GeminiAPIResponse struct {
	Candidates []Candidate `json:"candidates"`
}

// Candidate is a candidate response from the Gemini API.
type Candidate struct {
	Content Content `json:"content"`
}

// NewsSummaryResult is the expected JSON structure for a news summary.
type NewsSummaryResult struct {
	StockCode              string   `json:"stock_code"`
	SummarySentiment       string   `json:"summary_sentiment"`
	SummaryImpact          string   `json:"summary_impact"`
	SummaryConfidenceScore float64  `json:"summary_confidence_score"`
	KeyIssues              []string `json:"key_issues"`
	SuggestedAction        string   `json:"suggested_action"`
	Reasoning              string   `json:"reasoning"`
	ShortSummary           string   `json:"short_summary"`
}

type IndividualAnalysisResponse struct {
	Symbol            string            `json:"symbol"`
	AnalysisDate      time.Time         `json:"analysis_date"`
	TechnicalAnalysis TechnicalAnalysis `json:"technical_analysis"`
	Recommendation    Recommendation    `json:"recommendation"`
	NewsSummary       NewsSummary       `json:"news_summary,omitempty"`
}

// Technical Analysis
type TechnicalAnalysis struct {
	Trend                  string   `json:"trend"`
	Momentum               string   `json:"momentum"`
	EMASignal              string   `json:"ema_signal"`
	RSISignal              string   `json:"rsi_signal"`
	MACDSignal             string   `json:"macd_signal"`
	BollingerBandsPosition string   `json:"bollinger_bands_position"`
	SupportLevel           float64  `json:"support_level"`
	ResistanceLevel        float64  `json:"resistance_level"`
	TechnicalScore         int      `json:"technical_score"`
	KeyInsights            []string `json:"key_insights"`
}

// Recommendation
type Recommendation struct {
	Action          string  `json:"action"`
	BuyPrice        float64 `json:"buy_price,omitempty"`
	TargetPrice     float64 `json:"target_price,omitempty"`
	CutLoss         float64 `json:"cut_loss,omitempty"`
	ConfidenceLevel int     `json:"confidence_level"`
	Reasoning       string  `json:"reasoning"`
	RiskRewardRatio float64 `json:"risk_reward_ratio"`
}

// News Summary
type NewsSummary struct {
	ConfidenceScore float64  `json:"confidence_score"`
	Sentiment       string   `json:"sentiment"`
	Impact          string   `json:"impact"`
	KeyIssues       []string `json:"key_issues"`
}

type PositionMonitoringRequest struct {
	Symbol               string    `json:"symbol" binding:"required"`
	BuyPrice             float64   `json:"buy_price" binding:"required"`
	BuyTime              time.Time `json:"buy_time" binding:"required"`
	MaxHoldingPeriodDays int       `json:"max_holding_period_days" binding:"required"`
	TargetPrice          float64   `json:"target_price" binding:"required"`
	StopLoss             float64   `json:"stop_loss" binding:"required"`
}

type PositionMonitoringResponse struct {
	Symbol               string                 `json:"symbol"`
	MarketPrice          float64                `json:"market_price"`
	BuyDate              time.Time              `json:"buy_date"`
	BuyPrice             float64                `json:"buy_price"`
	MaxHoldingPeriodDays int                    `json:"max_holding_period_days"`
	TechnicalAnalysis    TechnicalAnalysis      `json:"technical_analysis"`
	NewsSummary          NewsSummary            `json:"news_summary,omitempty"`
	Recommendation       RecommendationPosition `json:"recommendation,omitempty"`
}

type RecommendationPosition struct {
	Action          string   `json:"action"`
	BuyPrice        float64  `json:"buy_price,omitempty"`
	TargetPrice     float64  `json:"target_price,omitempty"`
	CutLoss         float64  `json:"cut_loss,omitempty"`
	ConfidenceLevel int      `json:"confidence_level"`
	ExitReasoning   string   `json:"exit_reasoning"`
	ExitConditions  []string `json:"exit_conditions"`
	RiskRewardRatio float64  `json:"risk_reward_ratio"`
}

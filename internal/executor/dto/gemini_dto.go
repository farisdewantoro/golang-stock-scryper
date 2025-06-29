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
	MarketPrice       float64           `json:"market_price"`
	Symbol            string            `json:"symbol"`
	AnalysisDate      time.Time         `json:"analysis_date"`
	TechnicalAnalysis TechnicalAnalysis `json:"technical_analysis"`
	Recommendation    Recommendation    `json:"recommendation"`
	NewsSummary       NewsSummary       `json:"news_summary,omitempty"`
}

type IndividualAnalysisResponseMultiTimeframe struct {
	MarketPrice          float64           `json:"market_price"`
	Symbol               string            `json:"symbol"`
	AnalysisDate         time.Time         `json:"analysis_date"`
	Action               string            `json:"action"`
	BuyPrice             float64           `json:"buy_price,omitempty"`
	TargetPrice          float64           `json:"target_price,omitempty"`
	CutLoss              float64           `json:"cut_loss,omitempty"`
	ConfidenceLevel      int               `json:"confidence_level,omitempty"`
	Reasoning            string            `json:"reasoning"`
	RiskRewardRatio      float64           `json:"risk_reward_ratio,omitempty"`
	TechnicalScore       int               `json:"technical_score,omitempty"`
	NewsSummary          NewsSummary       `json:"news_summary,omitempty"`
	EstimatedHoldingDays int               `json:"estimated_holding_days"`
	TimeframeAnalysis    TimeframeAnalysis `json:"timeframe_analysis"`
}

type TimeframeSummaries struct {
	TimeFrame1D string `json:"time_frame_1d"`
	TimeFrame4H string `json:"time_frame_4h"`
	TimeFrame1H string `json:"time_frame_1h"`
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
	ConfidenceScore float64 `json:"confidence_score"`
	Sentiment       string  `json:"sentiment"`
	Impact          string  `json:"impact"`
	Reasoning       string  `json:"reasoning"`
}

type PositionMonitoringRequest struct {
	Symbol               string    `json:"symbol" binding:"required"`
	BuyPrice             float64   `json:"buy_price" binding:"required"`
	BuyTime              time.Time `json:"buy_time" binding:"required"`
	MaxHoldingPeriodDays int       `json:"max_holding_period_days" binding:"required"`
	TargetPrice          float64   `json:"target_price" binding:"required"`
	StopLoss             float64   `json:"stop_loss" binding:"required"`
}

type PositionMonitoringResponseMultiTimeframe struct {
	MarketPrice          float64           `json:"market_price"`
	Symbol               string            `json:"symbol"`
	AnalysisDate         time.Time         `json:"analysis_date"`
	Action               string            `json:"action"`
	BuyPrice             float64           `json:"buy_price,omitempty"`
	BuyDate              time.Time         `json:"buy_date,omitempty"`
	MaxHoldingPeriodDays int               `json:"max_holding_period_days,omitempty"`
	TargetPrice          float64           `json:"target_price,omitempty"`
	CutLoss              float64           `json:"cut_loss,omitempty"`
	ExitTargetPrice      float64           `json:"exit_target_price,omitempty"`
	ExitCutLossPrice     float64           `json:"exit_cut_loss_price,omitempty"`
	ConfidenceLevel      int               `json:"confidence_level"`
	Reasoning            string            `json:"reasoning"`
	RiskRewardRatio      float64           `json:"risk_reward_ratio"`
	ExitRiskRewardRatio  float64           `json:"exit_risk_reward_ratio"`
	TechnicalScore       int               `json:"technical_score"`
	NewsSummary          NewsSummary       `json:"news_summary,omitempty"`
	TimeframeAnalysis    TimeframeAnalysis `json:"timeframe_analysis"`
}

type TimeframeAnalysis struct {
	Timeframe1D TimeframeAnalysisData `json:"time_frame_1d"`
	Timeframe4H TimeframeAnalysisData `json:"time_frame_4h"`
	Timeframe1H TimeframeAnalysisData `json:"time_frame_1h"`
}

type TimeframeAnalysisData struct {
	Trend      string  `json:"trend"`
	KeySignal  string  `json:"key_signal"`
	RSI        int     `json:"rsi"`
	Support    float64 `json:"support"`
	Resistance float64 `json:"resistance"`
}

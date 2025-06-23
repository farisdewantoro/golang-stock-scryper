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
	Symbol               string            `json:"symbol"`
	AnalysisDate         time.Time         `json:"analysis_date"`
	TechnicalAnalysis    TechnicalAnalysis `json:"technical_analysis"`
	Recommendation       Recommendation    `json:"recommendation"`
	RiskLevel            string            `json:"risk_level"`
	TechnicalSummary     TechnicalSummary  `json:"technical_summary"`
	MaxHoldingPeriodDays int               `json:"max_holding_period_days,omitempty"`
	NewsSummary          NewsSummary       `json:"news_summary,omitempty"`
}

// Technical Analysis
type TechnicalAnalysis struct {
	Trend                  string    `json:"trend"`
	ShortTermTrend         string    `json:"short_term_trend"`
	MediumTermTrend        string    `json:"medium_term_trend"`
	EMASignal              string    `json:"ema_signal"`
	RSISignal              string    `json:"rsi_signal"`
	MACDSignal             string    `json:"macd_signal"`
	StochasticSignal       string    `json:"stochastic_signal"`
	BollingerBandsPosition string    `json:"bollinger_bands_position"`
	SupportLevel           float64   `json:"support_level"`
	ResistanceLevel        float64   `json:"resistance_level"`
	KeySupportLevels       []float64 `json:"key_support_levels"`
	KeyResistanceLevels    []float64 `json:"key_resistance_levels"`
	VolumeTrend            string    `json:"volume_trend"`
	VolumeConfirmation     string    `json:"volume_confirmation"`
	Momentum               string    `json:"momentum"`
	CandlestickPattern     string    `json:"candlestick_pattern"`
	MarketStructure        string    `json:"market_structure"`
	TrendStrength          string    `json:"trend_strength"`
	BreakoutPotential      string    `json:"breakout_potential"`
	ConsolidationLevel     string    `json:"consolidation_level"`
	TechnicalScore         int       `json:"technical_score"`
}

// Recommendation
type Recommendation struct {
	Action             string             `json:"action"`
	BuyPrice           float64            `json:"buy_price,omitempty"`
	TargetPrice        float64            `json:"target_price,omitempty"`
	CutLoss            float64            `json:"cut_loss,omitempty"`
	ConfidenceLevel    int                `json:"confidence_level"`
	Reasoning          string             `json:"reasoning"`
	RiskRewardAnalysis RiskRewardAnalysis `json:"risk_reward_analysis"`
}

// Technical Summary
type TechnicalSummary struct {
	OverallSignal   string   `json:"overall_signal"`
	TrendStrength   string   `json:"trend_strength"`
	VolumeSupport   string   `json:"volume_support"`
	Momentum        string   `json:"momentum"`
	RiskLevel       string   `json:"risk_level"`
	ConfidenceLevel int      `json:"confidence_level"`
	KeyInsights     []string `json:"key_insights"`
}

// News Summary
type NewsSummary struct {
	ConfidenceScore float64  `json:"confidence_score"`
	Sentiment       string   `json:"sentiment"`
	Impact          string   `json:"impact"`
	KeyIssues       []string `json:"key_issues"`
}

// Risk Reward Analysis
type RiskRewardAnalysis struct {
	PotentialProfit           float64 `json:"potential_profit"`
	PotentialProfitPercentage float64 `json:"potential_profit_percentage"`
	PotentialLoss             float64 `json:"potential_loss"`
	PotentialLossPercentage   float64 `json:"potential_loss_percentage"`
	RiskRewardRatio           float64 `json:"risk_reward_ratio"`
	RiskLevel                 string  `json:"risk_level"`
	ExpectedHoldingPeriod     string  `json:"expected_holding_period"`
	SuccessProbability        int     `json:"success_probability"`
	TrendProbability          int     `json:"trend_probability"`
	VolumeProbability         int     `json:"volume_probability"`
	TechnicalProbability      int     `json:"technical_probability"`
}

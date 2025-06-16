package dto

type ExecutorSummaryResult struct {
	StockCode string `json:"stock_code"`
	IsSuccess bool   `json:"is_success"`
	Error     string `json:"error"`
}

type NewsSummaryTelegramResult struct {
	StockCode       string  `json:"stock_code"`
	ShortSummary    string  `json:"short_summary"`
	Action          string  `json:"action"`
	Sentiment       string  `json:"sentiment"`
	ConfidenceScore float64 `json:"confidence_score"`
}

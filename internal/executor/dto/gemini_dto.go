package dto

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

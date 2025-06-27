package entity

import (
	"time"

	"github.com/lib/pq"
)

// StockNewsSummary represents a summary of news articles for a specific stock.
type StockNewsSummary struct {
	ID                     uint           `gorm:"primaryKey" json:"id"`
	StockCode              string         `gorm:"type:varchar(50);not null" json:"stock_code"`
	SummarySentiment       string         `gorm:"type:varchar(50)" json:"summary_sentiment"`
	SummaryImpact          string         `gorm:"type:varchar(50)" json:"summary_impact"`
	SummaryConfidenceScore float64        `json:"summary_confidence_score"`
	KeyIssues              pq.StringArray `gorm:"type:text[]" json:"key_issues"`
	SuggestedAction        string         `gorm:"type:varchar(10)" json:"suggested_action"`
	Reasoning              string         `gorm:"type:text" json:"reasoning"`
	ShortSummary           string         `gorm:"type:text" json:"short_summary"`
	SummaryStart           time.Time      `json:"summary_start"`
	SummaryEnd             time.Time      `json:"summary_end"`
	HashIdentifier         string         `gorm:"type:text;not null" json:"hash_identifier"`
	CreatedAt              time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the table name for the StockNewsSummary model.
func (StockNewsSummary) TableName() string {
	return "stock_news_summary"
}

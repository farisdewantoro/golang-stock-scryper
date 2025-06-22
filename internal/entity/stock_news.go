package entity

import (
	"time"

	"github.com/lib/pq"
)

// StockNews represents a news article related to stocks.
type StockNews struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Title          string         `gorm:"not null" json:"title"`
	Link           string         `gorm:"unique;not null" json:"link"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	RawContent     string         `json:"raw_content"`
	Summary        string         `json:"summary"`
	HashIdentifier string         `gorm:"unique;not null" json:"hash_identifier"`
	Source         string         `json:"source"`
	GoogleRSSLink  string         `json:"google_rss_link"`
	ImpactScore    float64        `json:"impact_score"`
	KeyIssue       pq.StringArray `gorm:"key_issue;type:text[]" json:"key_issue"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	StockMentions  []StockMention `gorm:"foreignKey:StockNewsID" json:"stock_mentions"`

	// Fields populated by custom query for ranking
	Sentiment       string  `gorm:"-" json:"sentiment,omitempty"`
	Impact          string  `gorm:"-" json:"impact,omitempty"`
	ConfidenceScore float64 `gorm:"-" json:"confidence_score,omitempty"`
	Reason          string  `gorm:"-" json:"reason,omitempty"`
}

// TableName specifies the table name for the StockNews model.
func (StockNews) TableName() string {
	return "stock_news"
}

// StockMention represents a mention of a stock in a news article.
type StockMention struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	StockNewsID     uint      `json:"stock_news_id"`
	StockCode       string    `gorm:"not null" json:"stock_code"`
	Sentiment       string    `gorm:"not null" json:"sentiment"`
	Impact          string    `gorm:"not null" json:"impact"`
	Reason          string    `gorm:"not null" json:"reason"`
	ConfidenceScore float64   `gorm:"not null" json:"confidence_score"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (StockMention) TableName() string {
	return "stock_mentions"
}

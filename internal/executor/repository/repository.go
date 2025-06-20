package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
)

// NewsAnalyzerRepository defines the generic interface for a news analysis service.
type NewsAnalyzerRepository interface {
	Analyze(ctx context.Context, stockCode, title, publishedDate, content string) (*dto.NewsAnalysisResult, error)
}

// GeminiAIRepository defines the interface for the Gemini AI service, including news analysis and summarization.
type GeminiAIRepository interface {
	NewsAnalyzerRepository
	GenerateNewsSummary(ctx context.Context, stockCode string, newsItems []entity.StockNews) (*dto.NewsSummaryResult, error)
}

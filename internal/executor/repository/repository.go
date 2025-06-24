package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
)

type AIRepository interface {
	GenerateNewsSummary(ctx context.Context, stockCode string, newsItems []entity.StockNews) (*dto.NewsSummaryResult, error)
	AnalyzeStock(ctx context.Context, symbol string, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.IndividualAnalysisResponse, error)
	PositionMonitoring(ctx context.Context, request *dto.PositionMonitoringRequest, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.PositionMonitoringResponse, error)
	NewsAnalyze(ctx context.Context, title, publishedDate, content string) (*dto.NewsAnalysisResult, error)
}

package repository

import (
	"context"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
)

type AIRepository interface {
	GenerateNewsSummary(ctx context.Context, stockCode string, newsItems []entity.StockNews) (*dto.NewsSummaryResult, error)
	NewsAnalyze(ctx context.Context, title, publishedDate, content string) (*dto.NewsAnalysisResult, error)
	AnalyzeStockMultiTimeframe(ctx context.Context, symbol string, stockData *dto.StockDataMultiTimeframe, summary *entity.StockNewsSummary) (*dto.IndividualAnalysisResponseMultiTimeframe, error)
	PositionMonitoringMultiTimeframe(ctx context.Context, request *dto.PositionMonitoringRequest, stockData *dto.StockDataMultiTimeframe, summary *entity.StockNewsSummary) (*dto.PositionMonitoringResponseMultiTimeframe, error)
}

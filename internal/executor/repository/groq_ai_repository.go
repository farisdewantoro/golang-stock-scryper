package repository

import (
	"context"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/logger"
	"net/http"
	"time"
)

type groqAIRepository struct {
	client *http.Client
	cfg    *config.Config
	logger *logger.Logger
}

func NewGroqAIRepository(cfg *config.Config, logger *logger.Logger) AIRepository {
	return &groqAIRepository{
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
		cfg:    cfg,
		logger: logger,
	}
}

func (r *groqAIRepository) NewsAnalyze(ctx context.Context, title, publishedDate, content string) (*dto.NewsAnalysisResult, error) {
	return nil, nil
}

func (r *groqAIRepository) GenerateNewsSummary(ctx context.Context, stockCode string, newsItems []entity.StockNews) (*dto.NewsSummaryResult, error) {
	return nil, nil
}

func (r *groqAIRepository) AnalyzeStock(ctx context.Context, symbol string, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.IndividualAnalysisResponse, error) {
	return nil, nil
}

func (r *groqAIRepository) PositionMonitoring(ctx context.Context, request *dto.PositionMonitoringRequest, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.PositionMonitoringResponse, error) {
	return nil, nil
}

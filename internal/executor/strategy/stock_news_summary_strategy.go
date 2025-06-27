package strategy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/internal/executor/repository"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/telegram"
	"golang-stock-scryper/pkg/utils"

	"gorm.io/gorm"
)

// StockNewsSummaryStrategy defines the strategy for summarizing stock news.
type StockNewsSummaryStrategy struct {
	db                   *gorm.DB
	logger               *logger.Logger
	stockRepo            repository.StocksRepository
	stockNewsRepo        repository.StockNewsRepository
	stockNewsSummaryRepo repository.StockNewsSummaryRepository
	aiRepo               repository.AIRepository
	telegramNotifier     telegram.Notifier
}

// NewStockNewsSummaryStrategy creates a new instance of StockNewsSummaryStrategy.
func NewStockNewsSummaryStrategy(
	db *gorm.DB,
	logger *logger.Logger,
	stockRepo repository.StocksRepository,
	stockNewsRepo repository.StockNewsRepository,
	stockNewsSummaryRepo repository.StockNewsSummaryRepository,
	aiRepo repository.AIRepository,
	telegramNotifier telegram.Notifier,
) *StockNewsSummaryStrategy {
	return &StockNewsSummaryStrategy{
		db:                   db,
		logger:               logger,
		stockRepo:            stockRepo,
		stockNewsRepo:        stockNewsRepo,
		stockNewsSummaryRepo: stockNewsSummaryRepo,
		aiRepo:               aiRepo,
		telegramNotifier:     telegramNotifier,
	}
}

// GetType returns the job type this strategy handles.
func (s *StockNewsSummaryStrategy) GetType() entity.JobType {
	return entity.JobTypeStockNewsSummary
}

// StockNewsSummaryPayload defines the payload for the stock news summary job.
type StockNewsSummaryPayload struct {
	MinToSummarizeNews int     `json:"min_to_summarize_news"`
	MinConfidenceScore float64 `json:"min_confidence_score"`
	MaxNewsAgeInDays   int     `json:"max_news_age_in_days"`
	MaxNewsEachStock   int     `json:"max_news_each_stock"`
}

// Execute runs the stock news summary job.
func (s *StockNewsSummaryStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	var payload StockNewsSummaryPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return "", fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []dto.ExecutorSummaryResult
	)

	stocks, err := s.stockNewsRepo.GetStocksToSummarize(ctx, payload.MaxNewsAgeInDays, payload.MinToSummarizeNews, payload.MinConfidenceScore)
	if err != nil {
		s.logger.Error("Failed to get stocks", logger.ErrorField(err))
		return "", fmt.Errorf("failed to get stocks: %w", err)
	}

	for _, code := range stocks {
		wg.Add(1)
		utils.GoSafe(func() {
			defer wg.Done()
			s.logger.Info("Executing stock news summary job", logger.StringField("stock_code", code))

			// 1. Fetch ranked news from the database
			rankedNews, err := s.stockNewsRepo.FindRankedNews(ctx, code, payload.MaxNewsEachStock, payload.MaxNewsAgeInDays, []string{})
			if err != nil {
				s.logger.Error("Failed to fetch ranked news", logger.ErrorField(err), logger.StringField("stock_code", code))
				mu.Lock()
				results = append(results, dto.ExecutorSummaryResult{
					StockCode: code,
					IsSuccess: false,
					Error:     err.Error(),
				})
				mu.Unlock()
				return
			}

			if len(rankedNews) == 0 {
				s.logger.Info("No news found for summary generation", logger.StringField("stock_code", code))
				mu.Lock()
				results = append(results, dto.ExecutorSummaryResult{
					StockCode: code,
					IsSuccess: false,
					Error:     "no news found for summary generation",
				})
				mu.Unlock()
				return
			}

			var combineStr strings.Builder
			for _, item := range rankedNews {
				if combineStr.Len() > 0 {
					combineStr.WriteString("|")
				}
				if item.PublishedAt == nil {
					continue
				}
				combineStr.WriteString(item.Link + "|" + item.StockCode)
			}

			hashIdentifier := sha256.Sum256([]byte(combineStr.String()))
			hashString := hex.EncodeToString(hashIdentifier[:])

			summaryExists, err := s.stockNewsSummaryRepo.Get(ctx, &dto.GetStockSummaryParam{
				HashIdentifier: hashString,
			})
			if err != nil {
				s.logger.Error("Failed to get stock news summary", logger.ErrorField(err))
				mu.Lock()
				results = append(results, dto.ExecutorSummaryResult{
					StockCode: code,
					IsSuccess: false,
					Error:     err.Error(),
				})
				mu.Unlock()
				return
			}

			if len(summaryExists) > 0 {
				s.logger.Info("Stock news summary already exists", logger.StringField("stock_code", code))
				mu.Lock()
				results = append(results, dto.ExecutorSummaryResult{
					StockCode: code,
					IsSuccess: false,
					Error:     "stock news summary already exists",
				})
				mu.Unlock()
				return
			}

			// 2. Call Gemini API to get the summary
			summaryResult, err := s.aiRepo.GenerateNewsSummary(ctx, code, rankedNews)
			if err != nil {
				s.logger.Error("Failed to generate news summary from Gemini", logger.ErrorField(err))
				mu.Lock()
				results = append(results, dto.ExecutorSummaryResult{
					StockCode: code,
					IsSuccess: false,
					Error:     err.Error(),
				})
				mu.Unlock()
				return
			}

			// 3. Parse the response and save it to the database

			summary := entity.StockNewsSummary{
				StockCode:              summaryResult.StockCode,
				SummarySentiment:       summaryResult.SummarySentiment,
				SummaryImpact:          summaryResult.SummaryImpact,
				SummaryConfidenceScore: summaryResult.SummaryConfidenceScore,
				KeyIssues:              summaryResult.KeyIssues,
				SuggestedAction:        summaryResult.SuggestedAction,
				Reasoning:              summaryResult.Reasoning,
				ShortSummary:           summaryResult.ShortSummary,
				CreatedAt:              utils.TimeNowWIB(),
				HashIdentifier:         hashString,
			}

			// set summary start and end
			for _, news := range rankedNews {
				if news.PublishedAt == nil {
					continue
				}
				if summary.SummaryStart.IsZero() {
					summary.SummaryStart = *news.PublishedAt
				}
				if summary.SummaryEnd.IsZero() {
					summary.SummaryEnd = *news.PublishedAt
				}
				if news.PublishedAt.Before(summary.SummaryStart) {
					summary.SummaryStart = *news.PublishedAt
				}
				if news.PublishedAt.After(summary.SummaryEnd) {
					summary.SummaryEnd = *news.PublishedAt
				}
			}

			if err := s.stockNewsSummaryRepo.Create(ctx, &summary); err != nil {
				s.logger.Error("Failed to save news summary", logger.ErrorField(err))
				mu.Lock()
				results = append(results, dto.ExecutorSummaryResult{
					StockCode: code,
					IsSuccess: false,
					Error:     err.Error(),
				})
				mu.Unlock()
				return
			}

			s.logger.Info("Successfully generated and saved stock news summary", logger.StringField("stock_code", code))

			mu.Lock()
			results = append(results, dto.ExecutorSummaryResult{
				StockCode: code,
				IsSuccess: true,
			})
			mu.Unlock()
		})
	}
	wg.Wait()

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(resultJSON), nil
}

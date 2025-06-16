package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

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
	stockNewsRepo        repository.StockNewsRepository
	stockNewsSummaryRepo repository.StockNewsSummaryRepository
	geminiRepo           repository.GeminiAIRepository
	telegramNotifier     telegram.Notifier
}

// NewStockNewsSummaryStrategy creates a new instance of StockNewsSummaryStrategy.
func NewStockNewsSummaryStrategy(
	db *gorm.DB,
	logger *logger.Logger,
	stockNewsRepo repository.StockNewsRepository,
	stockNewsSummaryRepo repository.StockNewsSummaryRepository,
	geminiRepo repository.GeminiAIRepository,
	telegramNotifier telegram.Notifier,
) *StockNewsSummaryStrategy {
	return &StockNewsSummaryStrategy{
		db:                   db,
		logger:               logger,
		stockNewsRepo:        stockNewsRepo,
		stockNewsSummaryRepo: stockNewsSummaryRepo,
		geminiRepo:           geminiRepo,
		telegramNotifier:     telegramNotifier,
	}
}

// GetType returns the job type this strategy handles.
func (s *StockNewsSummaryStrategy) GetType() entity.JobType {
	return entity.JobTypeStockNewsSummary
}

// StockNewsSummaryPayload defines the payload for the stock news summary job.
type StockNewsSummaryPayload struct {
	StockCodes         []string `json:"stock_codes"`
	MaxNews            int      `json:"max_news"`
	MaxNewsAgeInDays   int      `json:"max_news_age_in_days"`
	PriorityDomainList []string `json:"priority_domain_list"`
}

// Execute runs the stock news summary job.
func (s *StockNewsSummaryStrategy) Execute(ctx context.Context, job *entity.Job) (string, error) {
	var payload StockNewsSummaryPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return "", fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	// var results []scrapeResult
	var (
		wg              sync.WaitGroup
		mu              sync.Mutex
		results         []dto.ExecutorSummaryResult
		telegramResults []dto.NewsSummaryTelegramResult
	)

	for _, stockCode := range payload.StockCodes {
		wg.Add(1)
		code := stockCode
		utils.GoSafe(func() {
			defer wg.Done()
			s.logger.Info("Executing stock news summary job", logger.StringField("stock_code", code))

			// 1. Fetch ranked news from the database
			rankedNews, err := s.stockNewsRepo.FindRankedNews(ctx, code, payload.MaxNews, payload.MaxNewsAgeInDays, payload.PriorityDomainList)
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

			// 2. Call Gemini API to get the summary
			summaryResult, err := s.geminiRepo.GenerateNewsSummary(ctx, code, rankedNews)
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
			}

			// set summary start and end
			for _, news := range rankedNews {
				if news.PublishedAt == nil {
					continue
				}
				if news.PublishedAt.After(summary.SummaryStart) {
					summary.SummaryStart = *news.PublishedAt
				}
				if news.PublishedAt.Before(summary.SummaryEnd) {
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
			telegramResults = append(telegramResults, dto.NewsSummaryTelegramResult{
				StockCode:       code,
				ShortSummary:    summaryResult.ShortSummary,
				Action:          summaryResult.SuggestedAction,
				Sentiment:       summaryResult.SummarySentiment,
				ConfidenceScore: summaryResult.SummaryConfidenceScore,
			})
			mu.Unlock()
		})
	}
	wg.Wait()

	messages := telegram.FormatNewsSummariesForTelegram(telegramResults)

	for _, message := range messages {
		if err := s.telegramNotifier.SendMessage(message); err != nil {
			s.logger.Error("Failed to send Telegram notification", logger.ErrorField(err))
		}
		time.Sleep(100 * time.Millisecond) // supaya tidak lebih dari 20 msg/detik
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(resultJSON), nil
}

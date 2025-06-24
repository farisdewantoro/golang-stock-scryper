package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/ratelimit"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

type openaiAIRepository struct {
	client         *http.Client
	cfg            *config.Config
	logger         *logger.Logger
	tokenLimiter   *ratelimit.TokenLimiter
	requestLimiter *rate.Limiter
}

func NewOpenAIRepository(cfg *config.Config, logger *logger.Logger) AIRepository {
	secondsPerRequest := time.Minute / time.Duration(cfg.OpenAI.MaxRequestPerMinute)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)

	tokenLimiter := ratelimit.NewTokenLimiter(cfg.OpenAI.MaxTokenPerMinute)

	return &openaiAIRepository{
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
		cfg:            cfg,
		logger:         logger,
		requestLimiter: requestLimiter,
		tokenLimiter:   tokenLimiter,
	}
}

func (r *openaiAIRepository) NewsAnalyze(ctx context.Context, title, publishedDate, content string) (*dto.NewsAnalysisResult, error) {
	prompt := BuildAnalyzeNewsPrompt(title, publishedDate, content)

	resp, err := r.SendRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	result := dto.NewsAnalysisResult{}
	err = r.parseResponseJSON(resp, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *openaiAIRepository) GenerateNewsSummary(ctx context.Context, stockCode string, newsItems []entity.StockNews) (*dto.NewsSummaryResult, error) {
	prompt := BuildSummarizeNewsPrompt(stockCode, newsItems)

	resp, err := r.SendRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	result := dto.NewsSummaryResult{}
	err = r.parseResponseJSON(resp, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *openaiAIRepository) AnalyzeStock(ctx context.Context, symbol string, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.IndividualAnalysisResponse, error) {
	prompt := BuildIndividualAnalysisPrompt(ctx, symbol, stockData, summary)

	resp, err := r.SendRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	result := dto.IndividualAnalysisResponse{}
	err = r.parseResponseJSON(resp, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *openaiAIRepository) PositionMonitoring(ctx context.Context, request *dto.PositionMonitoringRequest, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.PositionMonitoringResponse, error) {
	prompt := BuildPositionMonitoringPrompt(ctx, request, stockData, summary)

	resp, err := r.SendRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	result := dto.PositionMonitoringResponse{}
	err = r.parseResponseJSON(resp, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (r *openaiAIRepository) SendRequest(ctx context.Context, prompt string) (*dto.OpenAPIRes, error) {

	if err := r.requestLimiter.Wait(ctx); err != nil {
		r.logger.Error("failed to wait for request limit", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to wait for request limit: %w", err)
	}

	payload := dto.OpenAPIReq{
		Model: r.cfg.OpenAI.Model,
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	r.logger.Debug("Sending request to OpenAI API", logger.StringField("url", r.cfg.OpenAI.BaseURL), logger.StringField("prompt", prompt))

	req, err := http.NewRequestWithContext(ctx, "POST", r.cfg.OpenAI.BaseURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create new http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.cfg.OpenAI.APIKey))

	r.logger.Debug("Sending request to OpenAI API", logger.StringField("url", r.cfg.OpenAI.BaseURL), logger.StringField("model", r.cfg.OpenAI.Model))

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Error("Received non-OK response from OpenAI API", logger.IntField("status_code", resp.StatusCode), logger.StringField("url", r.cfg.OpenAI.BaseURL), logger.StringField("model", r.cfg.OpenAI.Model))
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("received non-OK response from OpenAI API: %d - %s", resp.StatusCode, string(body))
	}

	var openaiResp dto.OpenAPIRes
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	if int(openaiResp.Usage.TotalTokens) > r.cfg.OpenAI.MaxTokenPerMinute/2 {
		r.logger.Warn("Token has exceeded 50% of the limit", logger.IntField("remaining", r.tokenLimiter.GetRemaining()))
	}

	if err := r.tokenLimiter.Wait(ctx, openaiResp.Usage.TotalTokens); err != nil {
		r.logger.Error("failed to wait for token limit", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to wait for token limit: %w", err)
	}

	return &openaiResp, nil
}

func (r *openaiAIRepository) parseResponseJSON(resp *dto.OpenAPIRes, result interface{}) error {
	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.Content) == 0 {
		return fmt.Errorf("no content found in Gemini response")
	}

	rawJSON := resp.Choices[0].Message.Content
	rawJSON = strings.Trim(rawJSON, "`json\n`")

	if err := json.Unmarshal([]byte(rawJSON), result); err != nil {
		return fmt.Errorf("failed to unmarshal summary from Gemini response: %w", err)
	}

	return nil
}

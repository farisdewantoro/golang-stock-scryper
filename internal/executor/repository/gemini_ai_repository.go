package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/logger"
	"golang-stock-scryper/pkg/ratelimit"

	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

// geminiAIRepository is an implementation of NewsAnalyzerRepository that uses the Google Gemini API.
type geminiAIRepository struct {
	client         *http.Client
	cfg            *config.Config
	logger         *logger.Logger
	tokenLimiter   *ratelimit.TokenLimiter
	requestLimiter *rate.Limiter
	genAiClient    *genai.Client
}

// NewGeminiAIRepository creates a new instance of geminiAIRepository.
func NewGeminiAIRepository(cfg *config.Config, log *logger.Logger, genAiClient *genai.Client) (AIRepository, error) {
	secondsPerRequest := time.Minute / time.Duration(cfg.Gemini.MaxRequestPerMinute)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)

	tokenLimiter := ratelimit.NewTokenLimiter(cfg.Gemini.MaxTokenPerMinute)

	return &geminiAIRepository{
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
		cfg:            cfg,
		logger:         log,
		requestLimiter: requestLimiter,
		tokenLimiter:   tokenLimiter,
		genAiClient:    genAiClient,
	}, nil
}

// Analyze performs news analysis using the Google Gemini API.
func (r *geminiAIRepository) NewsAnalyze(ctx context.Context, title, publishedDate, content string) (*dto.NewsAnalysisResult, error) {
	prompt := BuildAnalyzeNewsPrompt(title, publishedDate, content)

	geminiResp, err := r.executeGeminiAIRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return r.parseGeminiResponse(geminiResp)
}

// GenerateNewsSummary creates a summary of news for a stock.
func (r *geminiAIRepository) GenerateNewsSummary(ctx context.Context, stockCode string, newsItems []entity.StockNews) (*dto.NewsSummaryResult, error) {
	prompt := BuildSummarizeNewsPrompt(stockCode, newsItems)

	geminiResp, err := r.executeGeminiAIRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return r.parseSummaryResponse(geminiResp)
}

func (r *geminiAIRepository) executeGeminiAIRequest(ctx context.Context, prompt string) (*dto.GeminiAPIResponse, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(prompt, "user"),
	}
	geminiTokenResp, err := r.genAiClient.Models.CountTokens(ctx, r.cfg.Gemini.Model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to count tokens: %w", err)
	}

	r.logger.Debug("Gemini token count",
		logger.IntField("total_tokens", int(geminiTokenResp.TotalTokens)),
		logger.IntField("remaining", r.tokenLimiter.GetRemaining()),
	)

	if err := r.tokenLimiter.Wait(ctx, int(geminiTokenResp.TotalTokens)); err != nil {
		return nil, fmt.Errorf("failed to wait for token limit: %w", err)
	}

	if err := r.requestLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("failed to wait for request limit: %w", err)
	}

	if int(geminiTokenResp.TotalTokens) > r.cfg.Gemini.MaxTokenPerMinute/2 {
		r.logger.Warn("Token has exceeded 50% of the limit", logger.IntField("remaining", r.tokenLimiter.GetRemaining()))
	}

	payload := dto.GeminiAPIRequest{
		Contents: []dto.Content{{Parts: []dto.Part{{Text: prompt}}}},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	apiURL := fmt.Sprintf("%s/%s:generateContent?key=%s", r.cfg.Gemini.BaseURL, r.cfg.Gemini.Model, r.cfg.Gemini.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create new http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Gemini API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("received non-OK response from Gemini API: %d - %s", resp.StatusCode, string(body))
	}

	var geminiResp dto.GeminiAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return &geminiResp, nil
}

func (r *geminiAIRepository) parseGeminiResponse(resp *dto.GeminiAPIResponse) (*dto.NewsAnalysisResult, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("invalid response from Gemini API: no content found")
	}

	jsonString := resp.Candidates[0].Content.Parts[0].Text
	jsonString = strings.Trim(jsonString, "`json\n`")

	var result dto.NewsAnalysisResult
	if err := json.Unmarshal([]byte(jsonString), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal analysis result from Gemini response: %w", err)
	}
	return &result, nil
}

func (r *geminiAIRepository) parseSummaryResponse(resp *dto.GeminiAPIResponse) (*dto.NewsSummaryResult, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content found in Gemini response")
	}

	rawJSON := resp.Candidates[0].Content.Parts[0].Text
	rawJSON = strings.Trim(rawJSON, "`json\n`")

	var result dto.NewsSummaryResult
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal summary from Gemini response: %w", err)
	}

	return &result, nil
}

func (r *geminiAIRepository) AnalyzeStock(ctx context.Context, symbol string, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.IndividualAnalysisResponse, error) {
	prompt := BuildIndividualAnalysisPrompt(ctx, symbol, stockData, summary)

	geminiResp, err := r.executeGeminiAIRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return r.parseIndividualAnalysisResponse(geminiResp)
}

func (r *geminiAIRepository) PositionMonitoring(ctx context.Context, request *dto.PositionMonitoringRequest, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.PositionMonitoringResponse, error) {
	prompt := BuildPositionMonitoringPrompt(ctx, request, stockData, summary)

	geminiResp, err := r.executeGeminiAIRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return r.parsePositionMonitoringResponse(geminiResp)
}

func (r *geminiAIRepository) parseIndividualAnalysisResponse(resp *dto.GeminiAPIResponse) (*dto.IndividualAnalysisResponse, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content found in Gemini response")
	}

	rawJSON := resp.Candidates[0].Content.Parts[0].Text
	rawJSON = strings.Trim(rawJSON, "`json\n`")

	var result dto.IndividualAnalysisResponse
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal individual analysis response from Gemini response: %w", err)
	}

	return &result, nil
}

func (r *geminiAIRepository) parsePositionMonitoringResponse(resp *dto.GeminiAPIResponse) (*dto.PositionMonitoringResponse, error) {
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content found in Gemini response")
	}

	rawJSON := resp.Candidates[0].Content.Parts[0].Text
	rawJSON = strings.Trim(rawJSON, "`json\n`")

	var result dto.PositionMonitoringResponse
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal position monitoring response from Gemini response: %w", err)
	}

	return &result, nil
}

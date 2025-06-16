package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang-stock-scryper/internal/executor/config"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/logger"
)

// openRouterRepository is an implementation of OpenRouterRepository that uses the OpenRouter API.
type openRouterRepository struct {
	client *http.Client
	cfg    *config.Config
	logger *logger.Logger
}

// NewOpenRouterRepository creates a new instance of openRouterRepository.
func NewOpenRouterRepository(cfg *config.Config, logger *logger.Logger) NewsAnalyzerRepository {
	return &openRouterRepository{
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
		cfg:    cfg,
		logger: logger,
	}
}

// Analyze performs news analysis using the OpenRouter API.
func (r *openRouterRepository) Analyze(ctx context.Context, stockCode, title, publishedDate, content string) (*dto.NewsAnalysisResult, error) {
	prompt := r.buildPrompt(stockCode, title, publishedDate, content)

	requestBody := map[string]interface{}{
		"model": r.cfg.OpenRouter.Model, // A cost-effective and fast model
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
		},
		"response_json": map[string]string{
			"schema": `{
				"type": "object",
				"properties": {
					"summary": { "type": "string" },
					"impact_score": { "type": "number" },
					"key_issue": { "type": "array", "items": { "type": "string" } },
					"stock_mentions": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"stock_code": { "type": "string" },
								"sentiment": { "type": "string", "enum": ["positive", "neutral", "negative"] },
								"impact": { "type": "string", "enum": ["bullish", "bearish", "sideways"] },
								"confidence_score": { "type": "number" }
							},
							"required": ["stock_code", "sentiment", "impact", "confidence_score"]
						}
					}
				},
				"required": ["summary", "impact_score", "key_issue", "stock_mentions"]
			}`,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		r.logger.Error("Failed to marshal request body", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		r.logger.Error("Failed to create new HTTP request", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to create new http request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.cfg.OpenRouter.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Error("Failed to send request to OpenRouter", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to send request to OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Error("Received non-OK response from OpenRouter", logger.IntField("status_code", resp.StatusCode))
		return nil, fmt.Errorf("received non-OK response from OpenRouter: %d", resp.StatusCode)
	}

	var openRouterResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&openRouterResponse); err != nil {
		r.logger.Error("Failed to decode OpenRouter response", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to decode OpenRouter response: %w", err)
	}

	if len(openRouterResponse.Choices) == 0 {
		r.logger.Warn("Received empty choices from OpenRouter")
		return nil, fmt.Errorf("received empty choices from OpenRouter")
	}

	analysisContent := openRouterResponse.Choices[0].Message.Content
	r.logger.Debug("Received analysis from OpenRouter", logger.StringField("content", analysisContent))

	// Clean the response content by removing markdown code blocks
	cleanedContent := strings.TrimSpace(analysisContent)
	if strings.HasPrefix(cleanedContent, "```json") {
		cleanedContent = strings.TrimPrefix(cleanedContent, "```json")
		cleanedContent = strings.TrimSuffix(cleanedContent, "```")
		cleanedContent = strings.TrimSpace(cleanedContent)
	}

	var result dto.NewsAnalysisResult
	if err := json.Unmarshal([]byte(cleanedContent), &result); err != nil {
		r.logger.Error("Failed to unmarshal analysis JSON", logger.ErrorField(err), logger.StringField("content", cleanedContent))
		return nil, fmt.Errorf("failed to unmarshal analysis JSON from OpenRouter: %w", err)
	}

	return &result, nil
}

func (r *openRouterRepository) buildPrompt(stockCode, title, publishedDate, content string) string {
	return fmt.Sprintf(`Berikut adalah berita terkait saham yang saya dapatkan dari RSS feed ketika mencari saham %s. Tolong analisa dan berikan output dalam JSON seperti:

Kriteria analisis:
- Sentimen: "positive", "neutral", atau "negative"
- Dampak harga: "bullish", "bearish", atau "sideways"
- Confidence Score: nilai antara 0.0 (sangat tidak yakin) hingga 1.0 (sangat yakin)
- News Impact Score: nilai antara 0.0 (sangat tidak berdampak / tidak berkualitas) hingga 1.0 (sangat berdampak / berkualitas)
- News Summary: ringkasan singkat dari berita tersebut
- News Key Issue: array dari isu-isu penting yang terkait dengan berita tersebut ('dividen', 'laporan keuangan', 'analisa', 'dan key issue lainnya (kamu define sendiri)')

Tolong analisa dan berikan output dalam format JSON dengan struktur berikut:
{
  "summary": "ANTM akan membagikan dividen terbesar dalam sejarah perusahaan. Langkah ini diambil karena laba bersih perusahaan naik signifikan sepanjang 2024...",
  "key_issue": ["dividen", "laporan keuangan", "analisa"],
  "impact_score": 0.88,
  "stock_mentions":[
    {
      "stock_code": "ANTM",
      "sentiment": "positive",
      "impact": "bullish",
      "confidence_score": 0.88
    }
  ]
}

Berikut Data News:
Judul: %s
Tanggal Publish: %s
Raw Content: %s


Jawaban hanya dalam format JSON saja.
`, stockCode, title, publishedDate, content)
}

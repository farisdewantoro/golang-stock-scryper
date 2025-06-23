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
	"golang-stock-scryper/pkg/utils"

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
func NewGeminiAIRepository(cfg *config.Config, log *logger.Logger, genAiClient *genai.Client) (GeminiAIRepository, error) {
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
func (r *geminiAIRepository) Analyze(ctx context.Context, title, publishedDate, content string) (*dto.NewsAnalysisResult, error) {
	prompt := r.buildAnalyzeNewsPrompt(title, publishedDate, content)

	geminiResp, err := r.executeGeminiAIRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return r.parseGeminiResponse(geminiResp)
}

// GenerateNewsSummary creates a summary of news for a stock.
func (r *geminiAIRepository) GenerateNewsSummary(ctx context.Context, stockCode string, newsItems []entity.StockNews) (*dto.NewsSummaryResult, error) {
	prompt := r.buildSummarizeNewsPrompt(stockCode, newsItems)

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

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", r.cfg.Gemini.Model, r.cfg.Gemini.APIKey)
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

func (r *geminiAIRepository) buildAnalyzeNewsPrompt(title, publishedDate, content string) string {
	return fmt.Sprintf(`Anda adalah analis pasar modal Indonesia yang ahli dalam mengaitkan peristiwa berita dengan saham. Tolong analisa dan berikan output dalam JSON seperti:

Kriteria analisis:
- Sentimen: "positive", "neutral", atau "negative"
- Dampak harga: "bullish", "bearish", atau "sideways"
- Confidence Score: nilai antara 0.0 (sangat tidak yakin) hingga 1.0 (sangat yakin)
- News Impact Score: nilai antara 0.0 (sangat tidak berdampak / tidak berkualitas) hingga 1.0 (sangat berdampak / berkualitas)
- News Summary: ringkasan singkat dari berita tersebut
- News Key Issue: array dari isu-isu penting yang terkait dengan berita tersebut ('dividen', 'laporan keuangan', 'analisa', 'dan key issue lainnya (kamu define sendiri)')
- News Reason: alasan mengapa berita tersebut berdampak pada saham tersebut

Catatan penting:
- "reason" WAJIB diisi dan tidak boleh kosong.
- Jika berita kurang berdampak langsung, buat inferensi logis dari konteks umum saham tersebut.
- Jika tidak berdampak sama sekali field stock_code pada array stock_mentions diisi dengan TIDAK_RELEVAN

Tolong analisa dan berikan output dalam format JSON dengan struktur berikut:
{
  "summary": "ANTM akan membagikan dividen terbesar dalam sejarah perusahaan. Langkah ini diambil karena laba bersih perusahaan naik signifikan sepanjang 2024...",
  "key_issue": ["dividen", "laporan keuangan", "analisa"],
  "impact_score": 0.88,
  "stock_mentions":[
    {
      "stock_code": "ANTM | TIDAK_RELEVAN" ,
      "sentiment": "positive",
      "impact": "bullish",
      "confidence_score": 0.88,
	  "reason": "Penjelasan logis dan spesifik kenapa berita ini berdampak ke saham ini."
    }
  ]
}




Berikut Data News:
Judul: %s
Tanggal Publish: %s
Raw Content: %s


Jawaban hanya dalam format JSON saja.
`, title, publishedDate, content)
}

func (r *geminiAIRepository) buildSummarizeNewsPrompt(stockCode string, newsItems []entity.StockNews) string {
	var newsBuilder strings.Builder
	for i, news := range newsItems {
		keyIssuesJSON, _ := json.Marshal(news.KeyIssue)
		publishedAtStr := "N/A"
		if news.PublishedAt != nil {
			publishedAtStr = news.PublishedAt.Format("2006-01-02 15:04:05")
		}
		// Each news item is formatted and appended to the builder
		newsBuilder.WriteString(fmt.Sprintf(
			"%d. Title: \"%s\"\n   Published At: %s\n   Summary: %s\n   Sentiment: %s\n   Reason: %s\n   Impact: %s\n   Confidence Score: %.2f\n   Key Issues: %s\n\n",
			i+1, news.Title, publishedAtStr, news.Summary, news.Sentiment, news.Reason, news.Impact, news.ConfidenceScore, string(keyIssuesJSON),
		))
	}

	// The main prompt template is now a multi-line string for readability
	promptTemplate := `Berikut adalah beberapa berita terbaru terkait saham %s:

%s
Berdasarkan semua informasi di atas, berikan analisis dengan format JSON:

{
  "stock_code": "%s",
  "summary_sentiment": "positive | negative | neutral",
  "summary_impact": "bullish | bearish | sideways",
  "summary_confidence_score": 0.0 - 1.0,
  "key_issues": ["{dalam bahasa indonesia}"],
  "suggested_action": "buy | hold | sell",
  "reasoning": "{dalam bahasa indonesia}",
  "short_summary": "{dalam bahasa indonesia, panjang maksimal 1 paragraf}",
}`

	return fmt.Sprintf(promptTemplate, stockCode, newsBuilder.String(), stockCode)
}

func (r *geminiAIRepository) AnalyzeStock(ctx context.Context, symbol string, stockData *dto.StockData, summary *entity.StockNewsSummary) (*dto.IndividualAnalysisResponse, error) {
	prompt := r.buildIndividualAnalysisPrompt(ctx, symbol, stockData, summary)

	geminiResp, err := r.executeGeminiAIRequest(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return r.parseIndividualAnalysisResponse(geminiResp)
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

func (r *geminiAIRepository) buildIndividualAnalysisPrompt(
	ctx context.Context,
	symbol string,
	stockData *dto.StockData,
	summary *entity.StockNewsSummary,
) string {
	// Convert OHLCV data to JSON string
	ohlcvJSON, _ := json.Marshal(stockData.OHLCV)

	// Ringkasan sentimen dari berita
	newsSummaryText := ""
	if summary != nil {
		newsSummaryText = fmt.Sprintf(`
### INPUT BERITA TERKINI
Berikut adalah ringkasan berita untuk saham %s selama periode %s hingga %s:
- Sentimen utama: %s
- Dampak terhadap harga: %s
- Key issues: %s
- Ringkasan singkat: %s
- Confidence score: %.2f
- Saran tindakan: %s
- Alasan: %s

**Gunakan informasi ini sebagai konteks eksternal saat menganalisis data teknikal.**
`,
			summary.StockCode,
			summary.SummaryStart.Format("2006-01-02"),
			summary.SummaryEnd.Format("2006-01-02"),
			summary.SummarySentiment,
			summary.SummaryImpact,
			strings.Join(summary.KeyIssues, ", "),
			summary.ShortSummary,
			summary.SummaryConfidenceScore,
			summary.SuggestedAction,
			summary.Reasoning,
		)
	}

	prompt := fmt.Sprintf(`
### PERAN ANDA
Anda adalah analis teknikal berpengalaman di pasar saham Indonesia. Tugas Anda adalah memberikan analisis swing trading jangka pendek (1-5 hari) untuk saham %s berdasarkan data harga dan berita pasar terbaru.

### INPUT BERITA
%s

### DATA OHLC (%s)
%s

### HARGA PASAR SAAT INI
%.2f

### ATURAN REKOMENDASI
- Berikan sinyal **BUY** hanya jika **risk/reward ratio ≥ 1:3**
- Jika tidak memenuhi syarat: keluarkan sinyal **HOLD**
- Gunakan indikator teknikal seperti EMA, MACD, RSI, volume, candlestick, Bollinger Bands
- Maksimum holding 1-5 hari
- Cut loss berbasis support kuat

### FORMAT OUTPUT (JSON)
{
  "symbol": "%s",
  "analysis_date": "%s",
  "max_holding_period_days": 3,
  "technical_analysis": {
    "trend": "BULLISH",
    "momentum": "STRONG",
    "ema_signal": "BULLISH",
    "rsi_signal": "NEUTRAL",
    "macd_signal": "BULLISH",
    "bollinger_bands_position": "UPPER",
    "support_level": 1420,
    "resistance_level": 1615,,
	"key_insights": [
      "Trend bullish dengan volume mendukung",
      "Support dan resistance teridentifikasi",
      "Risk/reward ratio layak untuk entry"
    ]
    "technical_score": 85
  },
  "recommendation": {
    "action": "BUY|HOLD",
    "buy_price": 1550,
    "target_price": 1720,
    "cut_loss": 1420,
    "risk_reward_ratio": 3.0,
    "confidence_level": 80,
    "reasoning": "Gabungkan hasil analisis teknikal dan konteks berita. Contoh: 'Trend bullish dengan indikator teknikal mendukung (EMA dan MACD positif, volume tinggi). Risk/reward ratio layak. Berita terbaru menunjukkan sentimen positif terhadap sektor EV dan kawasan industri, menjadi katalis penguatan jangka pendek.'"
  },
  "news_summary": {
    "sentiment": "positive",
    "impact": "bullish",
    "confidence_score": 0.8,
    "key_issues": ["EV", "industri", "investasi asing"]
  }
}
`, symbol, newsSummaryText, stockData.Range, string(ohlcvJSON), stockData.MarketPrice, symbol, utils.TimeNowWIB().Format("2006-01-02T15:04:05-07:00"))
	return prompt
}

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/utils"
	"strings"
	"time"
)

func BuildIndividualAnalysisPrompt(
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
  "technical_analysis": {
    "trend": "BULLISH|BEARISH|SIDEWAYS",
    "momentum": "WEAK_UP|STRONG_UP|FLAT|WEAK_DOWN|STRONG_DOWN",
    "ema_signal": "BULLISH|BEARISH|NO_CROSS",
    "rsi_signal": "NEUTRAL|OVERBOUGHT|OVERSOLD",
    "macd_signal": "BULLISH|BEARISH|NEUTRAL",
    "bollinger_bands_position": "UPPER_BAND|LOWER_BAND|MIDDLE_BAND",
    "support_level": 1420,
    "resistance_level": 1615,
	"key_insights": [
      "Trend bullish dengan volume mendukung",
      "Support dan resistance teridentifikasi",
      "Risk/reward ratio layak untuk entry"
    ],
    "technical_score": 85
  },
  "recommendation": {
    "action": "BUY|HOLD",
    "buy_price": 1550,
    "target_price": 1720,
    "cut_loss": 1420,
    "risk_reward_ratio": 3.0,
    "confidence_level": 80,
    "reasoning": "Trend bullish dengan volume tinggi dan indikator teknikal mendukung. Berita memberikan sentimen positif tambahan."
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

func BuildPositionMonitoringPrompt(ctx context.Context,
	request *dto.PositionMonitoringRequest,
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
Berikut adalah ringkasan sentimen berita untuk saham %s selama periode %s hingga %s:

- Sentimen utama: %s
- Dampak terhadap harga: %s
- Key issues: %s
- Ringkasan singkat: %s
- Confidence score: %.2f
- Saran tindakan: %s
- Alasan: %s

Gunakan ringkasan ini untuk mempertimbangkan konteks eksternal (berita) dalam analisis teknikal berikut.
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

	// Calculate remaining holding period
	positionAgeDays := int(time.Since(request.BuyTime).Hours() / 24)
	remainingDays := request.MaxHoldingPeriodDays - positionAgeDays
	if remainingDays < 0 {
		remainingDays = 0
	}

	prompt := fmt.Sprintf(`
### PERAN ANDA
Kamu adalah analis teknikal saham Indonesia yang ahli dalam swing trading. Evaluasi posisi saham berikut dan berikan rekomendasi: HOLD, SELL, atau CUT_LOSS.

### TUJUAN
Tujuanmu adalah mengevaluasi apakah posisi saham ini sebaiknya dipertahankan (HOLD), dijual (SELL), atau dihentikan (CUT_LOSS), berdasarkan kombinasi analisa teknikal (seperti trend, EMA, RSI, MACD, Bollinger Bands, volume), harga pasar saat ini, waktu tersisa dalam periode holding, serta ringkasan berita terbaru jika tersedia. 
Berita hanya boleh digunakan jika memiliki tingkat kepercayaan (confidence) yang tinggi dan memberikan dampak yang mendukung atau memperlemah sinyal teknikal utama. Abaikan berita yang tidak relevan atau bertentangan dengan analisa teknikal dominan.

Target Price dan Stop Loss sudah tersedia dan harus digunakan sebagai acuan awal. Namun, jika hasil analisis teknikal dan konteks berita menunjukkan bahwa target atau stop loss tersebut tidak lagi realistis atau terlalu agresif/defensif, kamu boleh merekomendasikan perubahan dengan alasan yang jelas dan terukur.

%s

### INPUT DATA SAHAM SAYA
Data posisi trading:
- Symbol: %s
- Buy Price: %.2f
- Buy Time: %s
- Max Holding Period: %d days
- Position Age: %d days
- Remaining Days: %d days
- Target Price: %.2f
- Stop Loss: %.2f


### DATA HARGA OHLC %s:
%s

### CURRENT MARKET PRICE
%.2f (ini adalah harga pasar saat ini)



### KRITERIA KEPUTUSAN (Gabungan Analisis Teknikal dan Berita):
- **HOLD** jika:
  - Trend jangka pendek & menengah masih positif
  - EMA, RSI, MACD mendukung kenaikan
  - Volume menguat
  - Risk-reward ≥ 1:3 dan waktu tersisa cukup
  - Berita mendukung (sentimen positif atau netral, berdampak bullish)

- **SELL** jika:
  - Indikator teknikal mulai melemah (trend, EMA, RSI, MACD)
  - Volume melemah saat harga naik
  - Target sulit tercapai dalam sisa waktu
  - Berita negatif dengan dampak bearish yang menguatkan sinyal teknikal

- **CUT_LOSS** jika:
  - Terjadi breakdown support penting atau sinyal reversal kuat
  - Risk tinggi, reward rendah, dan waktu hampir habis
  - Berita buruk meningkatkan risiko signifikan (confidence tinggi, dampak bearish)

Gunakan berita hanya jika relevan dan selaras atau berlawanan kuat dengan sinyal teknikal.

### FORMAT OUTPUT YANG DIHARAPKAN (JSON)
Pastikan field "exit_reasoning" dan "exit_conditions" selalu ditulis dalam bahasa Indonesia yang jelas dan mudah dipahami. Jelaskan alasan exit secara logis berdasarkan analisis teknikal dan konteks berita, serta jabarkan kondisi exit dalam bentuk poin-poin terstruktur berbahasa Indonesia.
{
  "symbol": "%s",
  "technical_analysis": {
    "trend": "BULLISH|BEARISH|SIDEWAYS",
    "momentum": "WEAK_UP|STRONG_UP|FLAT|WEAK_DOWN|STRONG_DOWN",
    "ema_signal": "BULLISH|BEARISH|NO_CROSS",
    "rsi_signal": "NEUTRAL|OVERBOUGHT|OVERSOLD",
    "macd_signal": "BULLISH|BEARISH|NEUTRAL",
    "bollinger_bands_position": "UPPER_BAND|LOWER_BAND|MIDDLE_BAND",
    "support_level": 1420,
    "resistance_level": 1615,
	"key_insights": [
      "Trend bullish dengan volume mendukung",
      "Support dan resistance teridentifikasi",
      "Risk/reward ratio layak untuk entry"
    ],
    "technical_score": 85
  },
  "recommendation": {
    "action": "HOLD|SELL|CUT_LOSS",
	"target_exit_price": 9500,
	"stop_loss_price": 8950,
	"exit_reasoning": "Trend bullish dengan volume tinggi dan indikator teknikal mendukung. Berita memberikan sentimen positif tambahan.",
	"exit_conditions": [
		"Mencapai target price 9500",
		"Stop loss di 8950",
		"Trend reversal signal"
	],
	"risk_reward_ratio": 3.0,
	"confidence_level": 80
  },
  "news_summary":{ (JIKA ADA NEWS SUMMARY)
    "confidence_score": 0.0 - 1.0,
    "sentiment": "positive, negative, neutral, mixed",
    "impact": "bullish, bearish, sideways"
    "key_issues": ["issue1", "issue2", "issue3"]
  }
}
  
### CATATAN
- Pastikan semua keputusan didasarkan pada kombinasi sinyal teknikal dan konteks berita, bukan berdasarkan perasaan atau prediksi jangka panjang. Jika indikator saling bertentangan, prioritaskan risk-reward dan waktu tersisa sebagai penentu akhir.
- Untuk bagian "exit_reasoning", berikan penjelasan ringkas namun jelas yang menggabungkan sinyal teknikal dan konteks berita (jika tersedia).
`, newsSummaryText, request.Symbol, request.BuyPrice, request.BuyTime.Format("2006-01-02T15:04:05-07:00"),
		request.MaxHoldingPeriodDays, positionAgeDays, remainingDays, request.TargetPrice, request.StopLoss, stockData.Range, string(ohlcvJSON),
		stockData.MarketPrice, request.Symbol)

	return prompt
}

func BuildSummarizeNewsPrompt(stockCode string, newsItems []entity.StockNews) string {
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

func BuildAnalyzeNewsPrompt(title, publishedDate, content string) string {
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

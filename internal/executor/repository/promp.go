package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"strings"
	"time"
)

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
  "summary_confidence_score": {0.0 - 1.0},
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
  "summary": "<string - wajib diisi dalam bahasa indonesia>",
  "key_issue": ["<string - wajib diisi 1 dalam bahasa indonesia>", "<string - wajib diisi 2 dalam bahasa indonesia>", "<string - wajib diisi 3 dalam bahasa indonesia>"],
  "impact_score": <float 0.0-1.0>,
  "stock_mentions":[
    {
      "stock_code": "STRING_SYMBOL | TIDAK_RELEVAN" ,
      "sentiment": "positive | neutral | negative",
      "impact": "bullish | bearish | sideways",
      "confidence_score": <float 0.0-1.0>,
	  "reason": "<string - wajib diisi dalam bahasa indonesia>"
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

func BuildIndividualAnalysisMultiTimeframePrompt(
	ctx context.Context,
	symbol string,
	stockData *dto.StockDataMultiTimeframe,
	summary *entity.StockNewsSummary,
) string {
	// Convert OHLCV data to JSON string
	ohlcvJSON1D, _ := json.Marshal(stockData.OHLCV1D)
	ohlcvJSON4H, _ := json.Marshal(stockData.OHLCV4H)
	ohlcvJSON1H, _ := json.Marshal(stockData.OHLCV1H)

	// Ringkasan sentimen dari berita
	newsSummaryText := `
### INPUT BERITA TERKINI
Tidak ada berita, jangan gunakan berita untuk analisa ini
`
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
Anda adalah analis saham berpengalaman dalam swing trading pasar saham Indonesia. Tugas Anda adalah menganalisis apakah saham %s layak untuk dibeli saat ini berdasarkan **multi-timeframe analysis** (1D, 4H, 1H) dan **ringkasan berita pasar terbaru**.

- **1D**: jangka menengah (sekitar 1-3 bulan terakhir)
- **4H**: jangka pendek (beberapa hari terakhir)
- **1H**: jangka sangat pendek (intraday/harian)

### TUJUAN
Evaluasi sinyal beli (BUY) berdasarkan trend dominan, volume, momentum, indikator teknikal (EMA, MACD, RSI, Bollinger Bands), serta sentimen berita. Hanya berikan sinyal **BUY** jika semua syarat teknikal dan berita terpenuhi.

### KRITERIA KEPUTUSAN:
- **BUY** jika:
  - 1D dan 4H menunjukkan trend BULLISH
  - EMA, MACD, dan RSI mendukung (tidak bertentangan)
  - 1H minimal netral atau rebound
  - Risk/Reward ≥ 3.0
  - Berita mendukung (impact bullish/neutral dan confidence ≥ 0.7)
- **HOLD** jika:
  - Sinyal teknikal tidak konklusif (sideways, mixed)
  - Berita tidak cukup kuat mendukung
  - Trend utama belum jelas
- Semua angka dan penilaian harus **konsisten secara logis dan matematis**. Jangan berikan sinyal BUY jika tidak memenuhi semua syarat di atas.
- Gunakan berita hanya jika relevan dan selaras atau berlawanan kuat dengan sinyal teknikal.

%s

### DATA HARGA OHLC

#### Timeframe: 1D
%s

#### Timeframe: 4H
%s

#### Timeframe: 1H
%s

### HARGA PASAR SAAT INI
%.2f


### FORMAT OUTPUT WAJIB:
Hanya berikan **output dalam format JSON valid**, tanpa penjelasan tambahan.

Contoh JSON yang di harapkan:
{
  "action": "BUY|HOLD",
  "buy_price": 1550,
  "target_price": 1720,
  "cut_loss": 1420,
  "risk_reward_ratio": 3.0,
  "confidence_level": 80,
  "reasoning": "Tulis penjelasan akhir keputusan analisis teknikal dalam Bahasa Indonesia.",
  "news_confidence_score": 70,
  "key_insights": [
    "Contoh insight teknikal 1 dalam Bahasa Indonesia.",
    "Contoh insight teknikal 2 dalam Bahasa Indonesia.",
    "Contoh insight teknikal 3 dalam Bahasa Indonesia."
  ],
  "technical_score": 85,
  "timeframe_summaries": {
 	"time_frame_1d": "Ringkasan teknikal jangka menengah dalam Bahasa Indonesia.",
    "time_frame_4h": "Ringkasan teknikal jangka pendek dalam Bahasa Indonesia.",
    "time_frame_1h": "Ringkasan teknikal jangka sangat pendek dalam Bahasa Indonesia."
  }
}
`, symbol, newsSummaryText, string(ohlcvJSON1D), string(ohlcvJSON4H), string(ohlcvJSON1H), stockData.MarketPrice)
	return prompt
}

func BuildPositionMonitoringMultiTimeframePrompt(ctx context.Context,
	request *dto.PositionMonitoringRequest,
	stockData *dto.StockDataMultiTimeframe,
	summary *entity.StockNewsSummary,
) string {
	// Convert OHLCV data to JSON string
	ohlcvJSON1D, _ := json.Marshal(stockData.OHLCV1D)
	ohlcvJSON4H, _ := json.Marshal(stockData.OHLCV4H)
	ohlcvJSON1H, _ := json.Marshal(stockData.OHLCV1H)

	// Ringkasan sentimen dari berita
	newsSummaryText := `
### INPUT BERITA TERKINI
Tidak ada berita, jangan gunakan berita untuk analisa ini	
`
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
Anda adalah analis teknikal swing trading. Evaluasi posisi saham yang sudah dibeli untuk memberikan rekomendasi: HOLD, SELL, atau CUT_LOSS berdasarkan analisis teknikal dari multi-timeframe (1D, 4H, 1H), serta ringkasan berita.

- **1D**: jangka menengah (sekitar 1-3 bulan terakhir)
- **4H**: jangka pendek (beberapa hari terakhir)
- **1H**: jangka sangat pendek (intraday/harian)

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


### INPUT DATA OHLC

#### Timeframe: 1D
%s

#### Timeframe: 4H
%s

#### Timeframe: 1H
%s


### HARGA PASAR SAAT INI
%.2f



### KRITERIA KEPUTUSAN (Gabungan Analisis Teknikal dan Berita):
- **HOLD** jika:
  - Trend dominan (1D dan 4H) masih naik
  - EMA, RSI, MACD mendukung kenaikan
  - 1H mungkin koreksi tapi tidak breakdown
  - Risk-reward ≥ 1:3 dan waktu tersisa cukup
  - Berita mendukung (sentimen positif atau netral, berdampak bullish)
  - Masih ada waktu dalam periode holding

- **SELL** jika:
  - Target price hampir tercapai dan indikator teknikal mulai melemah (trend, EMA, RSI, MACD)
  - Target sulit tercapai dalam sisa waktu
  - Berita negatif dengan dampak bearish yang menguatkan sinyal teknikal

- **CUT_LOSS** jika:
  - Breakdown support dengan volume
  - Trend berubah jadi BEARISH
  - Risk tinggi, reward rendah, dan waktu hampir habis
  - Berita buruk meningkatkan risiko signifikan (confidence tinggi, dampak bearish)

- Semua angka dan penilaian harus **konsisten secara logis dan matematis**. 
- Gunakan berita hanya jika relevan dan selaras atau berlawanan kuat dengan sinyal teknikal.


### FORMAT OUTPUT WAJIB (DALAM JSON)
{
  "action": "HOLD|SELL|CUTLOSS",
  "exit_target_price": 9500,
  "exit_cut_loss": 8950,
  "reasoning": "Tulis penjelasan dan alasan akhir keputusan analisis ini dalam Bahasa Indonesia.",
  "exit_conditions": [
    "Contoh Exit Condition 1 dalam Bahasa Indonesia.",
    "Contoh Exit Condition 2 dalam Bahasa Indonesia.",
    "Contoh Exit Condition 3 dalam Bahasa Indonesia."
  ],
  "risk_reward_ratio": 0.3, // HARUS dihitung dari rumus RRR = (Buy Price - SL) / (TP - Buy Price)
  "confidence_level": 80,
  "news_confidence_score": 70,	
  "key_insights": [
    "EMA menunjukkan uptrend kuat di semua timeframe.",
    "Terjadi konvergensi RSI antara 1H dan 4H, menandakan momentum menguat.",
    "Volume di 1D dan 4H meningkat, memberi dukungan pada breakout potensial.",
	"Level resistance kuat di 1600 (dari timeframe 1D) sedang diuji; jika breakout terjadi dengan volume, potensi rally ke 1720 terbuka lebar."
  ],
  "technical_score": 85,
  "timeframe_summaries": {
    "time_frame_1d": "Ringkasan teknikal jangka menengah dalam Bahasa Indonesia.",
    "time_frame_4h": "Ringkasan teknikal jangka pendek dalam Bahasa Indonesia.",
    "time_frame_1h": "Ringkasan teknikal jangka sangat pendek dalam Bahasa Indonesia."
  }
}
  
### CATATAN
- Pastikan semua keputusan didasarkan pada kombinasi sinyal teknikal dan konteks berita, bukan berdasarkan perasaan atau prediksi jangka panjang. Jika indikator saling bertentangan, prioritaskan risk-reward dan waktu tersisa sebagai penentu akhir.
`, newsSummaryText, request.Symbol, request.BuyPrice, request.BuyTime.Format("2006-01-02T15:04:05-07:00"),
		request.MaxHoldingPeriodDays, positionAgeDays, remainingDays, request.TargetPrice, request.StopLoss,
		string(ohlcvJSON1D), string(ohlcvJSON4H), string(ohlcvJSON1H), stockData.MarketPrice)

	return prompt
}

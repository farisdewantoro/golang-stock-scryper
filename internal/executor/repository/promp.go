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

	// Ringkasan sentimen dari berita (opsional)
	newsSummaryText := `
### INPUT BERITA TERKINI
Tidak ada berita, abaikan aspek berita dalam analisis ini.
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

**Gunakan informasi ini sebagai konteks eksternal saat menganalisis data teknikal, hanya jika relevan.**
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
Anda adalah analis saham berpengalaman dalam swing trading pasar saham Indonesia. Tugas Anda adalah menganalisis apakah saham %s layak untuk dibeli saat ini berdasarkan **multi-timeframe analysis** (1D, 4H, 1H) dan **ringkasan berita pasar terbaru** (jika tersedia).

### TUJUAN
Evaluasi sinyal beli (BUY) berdasarkan trend dominan, volume, momentum, indikator teknikal (EMA, MACD, RSI, Bollinger Bands), serta sentimen berita (jika tersedia).

Fokuskan analisis pada strategi **swing trading jangka pendek** dengan **estimasi holding period 1 hingga 7 hari kerja**. Oleh karena itu, prediksi harga dan keputusan beli harus mempertimbangkan potensi pergerakan harga dalam rentang waktu tersebut.

Hanya berikan sinyal **BUY** jika semua syarat teknikal dan, jika ada, berita juga mendukung.

### KRITERIA KEPUTUSAN:
- **BUY** jika:
  - 1D dan 4H menunjukkan trend BULLISH
  - EMA, MACD, dan RSI mendukung (tidak bertentangan)
  - 1H minimal netral atau rebound
  - Risk-reward ≥ 1:3
  - Jika tersedia, berita harus mendukung (impact bullish/neutral dan confidence ≥ 0.7)
- **HOLD** jika:
  - Sinyal teknikal tidak konklusif (sideways, mixed)
  - Trend utama belum jelas
  - Jika tersedia, berita tidak cukup kuat mendukung atau bertentangan
- Jika tidak ada berita, abaikan aspek berita dan fokus pada analisis teknikal.
- Semua angka dan penilaian harus **konsisten secara logis dan matematis**.
- Jangan berikan sinyal BUY jika tidak memenuhi semua syarat di atas.
- Jika ada konflik antar timeframe, prioritaskan analisis 1D dan 4H. Timeframe 1H hanya digunakan untuk validasi atau entry timing.

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

### INSTRUKSI PENGISIAN REASONING
- Jelaskan secara ringkas namun jelas alasan utama di balik keputusan akhir (BUY atau HOLD), berdasarkan indikator teknikal utama: EMA, MACD, RSI, Bollinger Bands, Support/Resistance, dan volume.
- Sebutkan kondisi dari indikator-indikator tersebut yang paling mendukung atau bertentangan dengan keputusan.
- Pastikan reasoning bersifat logis, seimbang, dan tidak mengabaikan sinyal teknikal yang bertentangan signifikan.
- Jika tersedia, sertakan pertimbangan dari berita: apakah sentimen mendukung keputusan teknikal atau justru bertentangan. Cantumkan dampaknya terhadap harga dan skor confidence dari berita.
- Jika tidak ada berita, jangan menyertakan analisis eksternal dan fokus pada indikator teknikal.
- Sertakan estimasi berapa lama saham sebaiknya di-hold (dalam hari kerja) untuk mencapai target price berdasarkan tren dan momentum saat ini (1-7 hari kerja).
- Penjelasan reasoning harus mendukung nilai "estimated_holding_days" yang diberikan. Sertakan alasan teknikal seperti kekuatan momentum, jarak ke resistance, atau prediksi waktu breakout yang memperkuat estimasi durasi tersebut.

### INSTRUKSI TEKNIS UNTUK PENGISIAN SKOR
- Field "confidence_level" (0-100) menunjukkan tingkat keyakinan atas keputusan akhir:
  - > 80 → Semua data teknikal dan berita (jika ada) mendukung keputusan dengan kuat
  - 60-80 → Mayoritas sinyal mendukung, namun ada potensi risiko kecil
  - 40-60 → Beberapa sinyal bertentangan, keyakinan keputusan masih cukup rendah
  - < 40 → Banyak konflik antar sinyal atau sinyal tidak jelas

- Field "technical_score" (0-100) menunjukkan kekuatan sinyal teknikal murni (tanpa mempertimbangkan berita):
  - > 85 → Semua indikator teknikal utama (EMA, MACD, RSI, Volume, Bollinger Bands) mendukung potensi kenaikan kuat
  - 60-85 → Mayoritas indikator mendukung potensi naik
  - 40-60 → Sinyal teknikal lemah atau tidak meyakinkan
  - < 40 → Banyak indikator menunjukkan potensi penurunan / tren melemah

### INSTRUKSI TEKNIS UNTUK PENGISIAN estimated_holding_days
- Field "estimated_holding_days" menunjukkan estimasi berapa lama (dalam hari kerja) saham diperkirakan akan mencapai "target_price" setelah sinyal "BUY" diberikan.
- Nilai harus berupa bilangan bulat antara **1 hingga 7**, sesuai dengan karakteristik swing trading jangka pendek.
- Gunakan analisis momentum harga, kekuatan trend, posisi terhadap resistance, dan volume untuk menentukan estimasi ini.

**Interpretasi berdasarkan "action":**
- Jika "action" == "BUY":
  - Field ini **wajib diisi**.
  - Berikan estimasi realistis kapan target_price kemungkinan tercapai.
- Jika "action" == "HOLD":
  - Field ini **boleh diisi** atau dikosongkan.
  - Jika diisi, berarti estimasi kapan saham bisa layak dipertimbangkan untuk dibeli.

### FORMAT OUTPUT WAJIB:
Hanya berikan **output dalam format JSON valid**, tanpa penjelasan tambahan.

Contoh JSON yang diharapkan:
{
  "action": "BUY|HOLD",
  "buy_price": <float64 DEFAULT 0>,
  "target_price": <float64 DEFAULT 0>,
  "cut_loss": <float64 DEFAULT 0>,
  "confidence_level": <int 0-100>,
  "reasoning": "<Tulis penjelasan akhir keputusan analisis teknikal dalam Bahasa Indonesia>",
  "technical_score": <int 0-100>,
  "estimated_holding_days": <int 1-7>,
  "timeframe_summaries": {
    "time_frame_1d": "<Ringkasan teknikal analisis yang menjelaskan Support/Resistance, EMA, MACD, RSI, Bollinger Bands dan informasi penting lainnya>",
    "time_frame_4h": "<Ringkasan teknikal analisis yang menjelaskan Support/Resistance, EMA, MACD, RSI, Bollinger Bands dan informasi penting lainnya>",
    "time_frame_1h": "<Ringkasan teknikal analisis yang menjelaskan Support/Resistance, EMA, MACD, RSI, Bollinger Bands dan informasi penting lainnya>"
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
Anda adalah analis teknikal swing trading. Evaluasi posisi saham yang sudah dibeli untuk memberikan rekomendasi: HOLD, SELL, atau CUTLOSS berdasarkan analisis teknikal dari multi-timeframe (1D, 4H, 1H), serta ringkasan berita.

### TUJUAN UTAMA
Menentukan keputusan posisi saham saat ini: HOLD, SELL, atau CUTLOSS, berdasarkan analisis teknikal multi-timeframe, risk-reward, dan sentimen berita (jika tersedia).

### KRITERIA PENILAIAN
Analisa berdasarkan:
- Trend dan indikator teknikal utama (EMA, RSI, MACD, Bollinger Bands, Volume).
- Risk-Reward Ratio relatif terhadap waktu tersisa (Max Holding).
- Relevansi dan kekuatan sentimen berita (opsional, jika confidence tinggi).
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


### INSTRUKSI PENGISIAN EXIT PRICE
- Gunakan "exit_target_price" untuk menyesuaikan target take profit secara dinamis:
  - Jika masih realistis, gunakan nilai "target_price" sebagai "exit_target_price".
  - Jika potensi kenaikan lebih besar dari "target_price", naikkan "exit_target_price" secara wajar berdasarkan sinyal teknikal terbaru.
  - Jika harga mulai melemah atau waktu tidak cukup, pertimbangkan menurunkan "exit_target_price" agar tetap merealisasikan profit.

- Gunakan "exit_cut_loss_price" untuk menentukan batas risiko terbaru:
  - Jika support teknikal berubah, sesuaikan level cut loss.
  - Jika ada sinyal distribusi atau pelemahan ekstrem, pertimbangkan menaikkan "exit_cut_loss_price" agar risiko tetap terkendali.


### INSTRUKSI PENGISIAN REASONING
- Jelaskan secara ringkas namun jelas alasan utama di balik keputusan akhir (SELL, HOLD atau CUTLOSS), berdasarkan indikator teknikal utama: EMA, MACD, RSI, Bollinger Bands, Support/Resistance, dan volume.
- Jika terjadi perubahan dari target awal (misalnya exit_target_price ≠ target_price, atau exit_cut_loss_price ≠ stop_loss), jelaskan alasan perubahan tersebut secara eksplisit berdasarkan sinyal teknikal atau waktu tersisa.
- Sebutkan kondisi dari indikator-indikator tersebut yang paling mendukung atau bertentangan dengan keputusan.
- Pastikan reasoning bersifat logis, seimbang, dan tidak mengabaikan sinyal teknikal yang bertentangan signifikan.
- Jika tersedia, sertakan pertimbangan dari berita: apakah sentimen mendukung keputusan teknikal atau justru bertentangan. Cantumkan dampaknya terhadap harga dan skor confidence dari berita.

### INSTRUKSI TEKNIS UNTUK PENGISIAN SKOR
- Field "confidence_level" (0-100) menunjukkan tingkat keyakinan atas keputusan akhir:
  - > 80 → Semua data teknikal dan berita (jika ada) mendukung keputusan dengan kuat
  - 60-80 → Mayoritas sinyal mendukung, namun ada potensi risiko kecil
  - 40-60 → Beberapa sinyal bertentangan, keyakinan keputusan masih cukup rendah
  - < 40 → Banyak konflik antar sinyal atau sinyal tidak jelas

- Field "technical_score" (0-100) menunjukkan kekuatan sinyal teknikal murni (tanpa mempertimbangkan berita):
  - > 85 → Semua indikator teknikal utama (EMA, MACD, RSI, Volume, Bollinger Bands) mendukung potensi kenaikan kuat
  - 60-85 → Mayoritas indikator mendukung potensi naik
  - 40-60 → Sinyal teknikal lemah atau tidak meyakinkan
  - < 40 → Banyak indikator menunjukkan potensi penurunan / tren melemah


### FORMAT OUTPUT WAJIB (DALAM JSON)
{
  "action": "HOLD|SELL|CUTLOSS",
  "exit_target_price": <float64 DEFAULT 0>,
  "exit_cut_loss_price": <float64 DEFAULT 0>,
  "reasoning": "<Tulis penjelasan dan alasan akhir keputusan analisis ini dalam Bahasa Indonesia>",
  "confidence_level": <int 0-100>,
  "technical_score": <int 0-100>,
  "timeframe_summaries": {
    "time_frame_1d": "<Ringkasan teknikal analisis yang menjelaskan Support/Resistance, EMA, MACD, RSI, Bollinger Bands dan informasi penting lainnya>",
    "time_frame_4h": "<Ringkasan teknikal analisis yang menjelaskan Support/Resistance, EMA, MACD, RSI, Bollinger Bands dan informasi penting lainnya>",
    "time_frame_1h": "<Ringkasan teknikal analisis yang menjelaskan Support/Resistance, EMA, MACD, RSI, Bollinger Bands dan informasi penting lainnya>"
  }
}

Ringkasan teknikal analisis yang menjelaskan Support/Resistance, EMA, MACD, RSI, Bollinger Bands dan juga pendapat lainnya yang penting untuk diinformasikan.

### CATATAN
- Pastikan semua keputusan didasarkan pada kombinasi sinyal teknikal dan konteks berita, bukan berdasarkan perasaan atau prediksi jangka panjang. Jika indikator saling bertentangan, prioritaskan risk-reward dan waktu tersisa sebagai penentu akhir.
`, newsSummaryText, request.Symbol, request.BuyPrice, request.BuyTime.Format("2006-01-02T15:04:05-07:00"),
		request.MaxHoldingPeriodDays, positionAgeDays, remainingDays, request.TargetPrice, request.StopLoss,
		string(ohlcvJSON1D), string(ohlcvJSON4H), string(ohlcvJSON1H), stockData.MarketPrice)

	return prompt
}

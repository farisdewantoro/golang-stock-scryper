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
Anda adalah analis saham berpengalaman dalam swing trading pasar saham Indonesia. Anda ahli dalam **analisa teknikal kuantitatif (indikator)** dan **analisa kualitatif (price action)**. Tugas Anda adalah menganalisis apakah saham %s layak untuk dibeli saat ini.

### TUJUAN
Evaluasi sinyal beli (BUY) berdasarkan:
1.  **Analisa Multi-Timeframe (1D, 4H, 1H)** untuk menentukan tren dominan.
2.  **Analisa Indikator Teknikal** (EMA, MACD, RSI, Bollinger Bands, Volume) untuk mengukur momentum dan kekuatan tren.
3.  **Analisa Price Action**, termasuk **pola candlestick** (misalnya, Doji, Engulfing, Hammer) dan **pola grafik** (misalnya, Triangles, Flags, Head and Shoulders) untuk konfirmasi dan sinyal dini.
4.  **Konteks Berita Pasar** (jika tersedia) sebagai faktor pendukung.

Fokuskan analisis pada strategi **swing trading jangka pendek** dengan **estimasi holding period 1 hingga 7 hari kerja**. Oleh karena itu, prediksi harga dan keputusan beli harus mempertimbangkan potensi pergerakan harga dalam rentang waktu tersebut.

Hanya berikan sinyal **BUY** jika semua syarat teknikal dan, jika ada, berita juga mendukung.

### KRITERIA KEPUTUSAN:
- **BUY** jika:
  - 1D dan 4H menunjukkan trend BULLISH.
  - **Ditemukan pola candlestick atau pola grafik bullish yang terkonfirmasi** pada timeframe 1D atau 4H (misalnya: Bullish Engulfing, breakout dari Ascending Triangle, Bullish Flag).
  - Indikator EMA, MACD, dan RSI mendukung (tidak ada divergensi bearish yang kuat).
  - 1H minimal netral atau menunjukkan sinyal rebound untuk timing entry.
  - Risk-reward ≥ 1:3.
  - Jika tersedia, berita harus mendukung (impact bullish/neutral dan confidence ≥ 0.7).
- **HOLD** jika:
  - Sinyal teknikal tidak konklusif (sideways, mixed) atau tidak ada pola konfirmasi yang kuat.
  - Trend utama belum jelas.
  - Jika tersedia, berita tidak cukup kuat mendukung atau bertentangan.
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
- Jelaskan secara ringkas namun jelas alasan utama di balik keputusan akhir (BUY atau HOLD), berdasarkan: **Pola Candlestick/Grafik yang teridentifikasi**, EMA, MACD, RSI, Bollinger Bands, Support/Resistance, dan volume.
- Sebutkan kondisi dari indikator-indikator dan pola-pola tersebut yang paling mendukung atau bertentangan dengan keputusan.
- **Sebutkan pola spesifik yang paling mendukung atau bertentangan dengan keputusan** (contoh: "Keputusan BUY didukung oleh breakout dari pola Symmetrical Triangle dan dikonfirmasi oleh candle Bullish Marubozu dengan volume tinggi").
- Pastikan reasoning bersifat logis, seimbang, dan tidak mengabaikan sinyal teknikal yang bertentangan signifikan.- Pastikan reasoning bersifat logis, seimbang, dan tidak mengabaikan sinyal teknikal yang bertentangan signifikan.
- Jika tersedia, sertakan pertimbangan dari berita: apakah sentimen mendukung keputusan teknikal atau justru bertentangan. Cantumkan dampaknya terhadap harga dan skor confidence dari berita.
- Jika tidak ada berita, jangan menyertakan analisis eksternal dan fokus pada indikator teknikal.
- Sertakan estimasi berapa lama saham sebaiknya di-hold (dalam hari kerja) untuk mencapai target price berdasarkan tren dan momentum saat ini (1-7 hari kerja).
- Penjelasan reasoning harus mendukung nilai "estimated_holding_days" yang diberikan. Sertakan alasan teknikal seperti kekuatan momentum, jarak ke resistance, atau prediksi waktu breakout yang memperkuat estimasi durasi tersebut.

### INSTRUKSI TEKNIS UNTUK PENGISIAN SKOR
- Field "confidence_level" (0-100) menunjukkan tingkat keyakinan atas keputusan akhir, dengan mempertimbangkan **SINTESIS dari kekuatan teknikal (technical_score) DAN dampak berita**.
  - **> 80 → Sinyal teknikal sangat kuat (technical_score > 85) DAN sentimen berita (jika ada, dengan confidence > 0.7) selaras mendukung keputusan.** Semua pilar analisis menunjuk ke arah yang sama.
  - **60-80 → Sinyal teknikal kuat, namun ada sedikit catatan.** Misalnya, technical_score tinggi tetapi berita bersifat netral, atau mayoritas sinyal teknikal mendukung tetapi ada satu aspek (misal: volume) yang kurang optimal.
  - **40-60 → Adanya konflik yang cukup signifikan.** Misalnya, sinyal teknikal bertentangan dengan sentimen berita, atau sinyal teknikal itu sendiri sudah tidak meyakinkan (technical_score rendah).
  - **< 40 → Banyak konflik antar sinyal atau berita yang sangat bertentangan.** Risiko sangat tinggi dan keputusan tidak dapat diandalkan.

- Field "technical_score" (0-100) menunjukkan kekuatan sinyal teknikal murni (tanpa mempertimbangkan berita), **menggabungkan analisa tren, pola (candlestick & grafik), indikator, dan volume.**
  - **> 85 → Semua pilar teknikal selaras dengan kuat.** Tren jelas (misal: UPTREND), ada **pola candlestick/grafik bullish yang terkonfirmasi** (misal: breakout triangle, bull flag), didukung oleh **volume tinggi**, dan indikator utama (EMA, MACD, RSI) semuanya memberikan sinyal bullish.
  - **60-85 → Mayoritas sinyal teknikal mendukung.** Tren utama bullish, namun mungkin **pola belum terkonfirmasi sepenuhnya**, volume rata-rata, atau salah satu indikator menunjukkan kondisi netral/jenuh.
  - **40-60 → Sinyal teknikal lemah, sideways, atau bertentangan.** Tren tidak jelas, **tidak ada pola yang kuat terbentuk**, dan indikator memberikan sinyal campuran (misal: MACD bullish tapi RSI bearish).
  - **< 40 → Banyak sinyal menunjukkan pelemahan atau potensi pembalikan bearish.** Misalnya, terbentuk **pola bearish (Head and Shoulders, Bearish Engulfing)**, harga breakdown dari support, atau adanya **divergensi bearish** yang kuat pada indikator.

### INSTRUKSI TEKNIS UNTUK PENGISIAN estimated_holding_days
- Isi field "estimated_holding_days" dengan memberikan **batas waktu maksimal** (dalam hari kerja, antara 1-7) di mana "target_price" seharusnya tercapai.
- Untuk menentukannya, pikirkan tentang rentang waktu yang realistis (misal: 3-5 hari), lalu ambil **angka tertingginya** (dalam contoh ini, 5) sebagai output.
- **Gunakan angka yang lebih kecil (misal: 2 atau 3)** untuk sinyal breakout yang sangat kuat dan momentumnya eksplosif.
- **Gunakan angka yang lebih besar (misal: 5, 6, atau 7)** untuk tren yang lebih lambat, bertahap, atau jika ada potensi konsolidasi.
- Nilai ini berfungsi sebagai 'time stop', yaitu jika target tidak tercapai dalam waktu ini, momentum dianggap hilang.

**Interpretasi berdasarkan "action":**
- Jika "action" == "BUY":
  - Field ini **wajib diisi**.
  - Berikan estimasi realistis kapan target_price kemungkinan tercapai.
- Jika "action" == "HOLD":
  - Field ini **boleh diisi** atau dikosongkan.
  - Jika diisi, berarti estimasi kapan saham bisa layak dipertimbangkan untuk dibeli.


### (FINAL) INSTRUKSI PENGISIAN 'timeframe_analysis'
Untuk setiap timeframe, isi field-field berikut dengan informasi yang paling ringkas dan penting:
- **trend**: Pilih salah satu ENUM: "BULLISH", "BEARISH", "SIDEWAYS", "WEAKENING_BULLISH" (melemah), "REVERSING_TO_BEARISH" (pembalikan).
- **key_signal**: Tulis **SATU** sinyal atau peristiwa teknikal **paling signifikan** dalam bentuk frasa singkat (maksimal 7 kata). Contoh: "Breakout dari Ascending Triangle", "Candlestick Hammer di support", "Menembus resistance 1500", "RSI menunjukkan Bearish Divergence".
- **rsi**: Tulis nilai numerik RSI saja (contoh: 68).
- **support**: Tulis SATU level support terpenting dan terdekat.
- **resistance**: Tulis SATU level resistance terpenting dan terdekat.

### FORMAT OUTPUT WAJIB:
Hanya berikan **output dalam format JSON valid yang sangat terstruktur**, tanpa penjelasan tambahan. Ikuti struktur di bawah ini dengan seksama.
{
  "action": "BUY|HOLD",
  "buy_price": <float64 DEFAULT 0>,
  "target_price": <float64 DEFAULT 0>,
  "cut_loss": <float64 DEFAULT 0>,
  "confidence_level": <int 0-100>,
  "reasoning": "<Sintesis akhir dari semua temuan di timeframe_analysis>",
  "technical_score": <int 0-100>,
  "estimated_holding_days": <int 1-7>,
  "timeframe_analysis": {
    "time_frame_1d": {
      "trend": "<ENUM>",
      "key_signal": "<Frasa singkat sinyal utama>",
      "rsi": <int>,
      "support": <float64>,
      "resistance": <float64>
    },
    "time_frame_4h": {
      // ... (struktur yang sama) ...
    },
    "time_frame_1h": {
      // ... (struktur yang sama) ...
    }
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
Anda adalah **Manajer Risiko dan Analis Posisi** untuk swing trading. Tugas Anda adalah mengevaluasi posisi saham yang sedang aktif (%s) dan memberikan rekomendasi taktis yang jelas: **HOLD, SELL (Take Profit), CUT_LOSS, atau ADJUST_STOP (Trailing Stop).**

### TUJUAN UTAMA
Lindungi modal dan maksimalkan keuntungan dengan mengevaluasi apakah posisi saat ini masih valid. Fokus pada **perubahan kondisi teknikal** sejak posisi dibuka dan **prospeknya** dalam sisa periode holding.

### INPUT DATA POSISI SAYA
Data posisi trading:
- Symbol: %s
- Buy Price: %.2f
- Buy Time: %s
- Max Holding Period: %d days
- Position Age: %d days
- Remaining Days: %d days
- Target Price: %.2f
- Stop Loss: %.2f

### HARGA PASAR SAAT INI
%.2f


### INPUT DATA OHLC

#### Timeframe: 1D
%s

#### Timeframe: 4H
%s

#### Timeframe: 1H
%s


%s // Ringkasan berita


### KRITERIA KEPUTUSAN UTAMA
- **HOLD**: Tren utama masih kuat, sinyal teknikal pendukung masih valid, dan masih ada potensi mencapai target dalam sisa waktu. Tidak ada sinyal bahaya yang signifikan.
- **SELL (Take Profit)**: Harga mendekati atau telah mencapai target, NAMUN muncul **sinyal pelemahan** (misal: divergensi bearish, pola candlestick pembalikan, volume klimaks) yang mengindikasikan potensi puncak. Atau, sisa waktu hampir habis dan momentum tidak cukup untuk mencapai target yang lebih tinggi.
- **CUT_LOSS**: Harga menembus level stop loss awal ATAU menembus level support krusial baru dengan konfirmasi. Tren dominan telah berbalik menjadi bearish.
- **ADJUST_STOP**: Harga telah bergerak naik secara signifikan (misal, sudah setengah jalan ke target) dan tren masih kuat. Rekomendasikan menaikkan level "exit_cut_loss_price" (misalnya ke harga beli/breakeven atau ke level support baru yang lebih tinggi) untuk **mengunci keuntungan dan menghilangkan risiko kerugian.**


### INSTRUKSI PENGISIAN EXIT PRICE
Isi "exit_target_price" dan "exit_cut_loss_price" berdasarkan "action" yang direkomendasikan dan analisis teknikal terbaru:

- **Jika "action" == "HOLD":**
  - "exit_target_price": Pertahankan target awal jika masih realistis. Jika momentum sangat kuat dan ada ruang, Anda boleh menaikkannya ke level resistance berikutnya.
  - "exit_cut_loss_price": Pertahankan stop loss awal, kecuali ada level support baru yang lebih tinggi dan lebih kuat yang terbentuk.

- **Jika "action" == "SELL" (Take Profit):**
  - "exit_target_price": Set ke harga pasar saat ini atau level resistance terdekat di mana sinyal pelemahan muncul. Tujuannya adalah untuk "mengamankan keuntungan sekarang".
  - "exit_cut_loss_price": Tidak relevan, karena aksi adalah untuk menjual. Isi dengan nilai stop loss awal.

- **Jika "action" == "CUT_LOSS":**
  - "exit_target_price": Tidak relevan. Isi dengan nilai target awal.
  - "exit_cut_loss_price": Set ke harga pasar saat ini. Tujuannya adalah untuk "keluar dari pasar secepatnya".

- **Jika "action" == "ADJUST_STOP":**
  - "exit_target_price": Pertahankan target awal, karena premisnya adalah tren masih akan berlanjut.
  - "exit_cut_loss_price": **Ini adalah field paling penting.** Naikkan ke level yang strategis, seperti:
    - Sedikit di atas harga beli (breakeven).
    - Di bawah level support kunci terbaru yang lebih tinggi.
    - Menggunakan metode trailing stop (misal: di bawah EMA 20 timeframe 4H).


### INSTRUKSI PENGISIAN REASONING
- **Fokus pada Perubahan:** Jelaskan **apa yang telah berubah** sejak posisi dibuka. Apakah momentum menguat atau melemah? Apakah ada sinyal pembalikan baru?
- **Justifikasi Aksi:** Berikan alasan teknikal yang jelas di balik setiap keputusan. Jika merekomendasikan "ADJUST_STOP", jelaskan mengapa ini saat yang tepat untuk melakukannya.
- **Penyesuaian Target/Stop:** Jika "exit_target_price" atau "exit_cut_loss_price" berbeda dari nilai awal, **jelaskan mengapa penyesuaian itu perlu** berdasarkan analisis teknikal terbaru (misal: "Stop loss dinaikkan ke 1100 untuk melindungi keuntungan karena support baru telah terbentuk di level tersebut").

### INSTRUKSI TEKNIS UNTUK PENGISIAN SKOR
Skor ini menilai **kesehatan posisi saat ini** dan **keyakinan pada aksi yang direkomendasikan**.

- **Field "confidence_level" (0-100):** Menunjukkan tingkat keyakinan atas "action" yang direkomendasikan (HOLD, SELL, dll.).
  - **> 80 → Sinyal sangat jelas dan searah.** Contoh: Untuk "HOLD", semua indikator masih sangat bullish. Untuk "SELL", sinyal pembalikan sangat terkonfirmasi.
  - **60-80 → Sinyal mayoritas mendukung**, tapi ada sedikit keraguan. Contoh: Untuk "HOLD", tren masih naik tapi RSI sudah overbought.
  - **40-60 → Sinyal campuran atau tidak jelas.** Ini seringkali menjadi alasan untuk "ADJUST_STOP" atau "HOLD" dengan waspada.
  - **< 40 → Kondisi sangat tidak menentu**, banyak sinyal konflik.

- **Field "technical_score" (0-100):** Menilai **kekuatan teknikal dari posisi itu sendiri**, terlepas dari aksi yang direkomendasikan. Ini menjawab pertanyaan: "Seberapa sehat premis bullish awal saat ini?"
  - **> 85 → Premis bullish masih sangat valid dan kuat.** Tren dominan masih naik kencang, tidak ada sinyal pembalikan signifikan.
  - **60-85 → Premis bullish masih valid, namun mulai menunjukkan tanda-tanda normal.** Misal: tren melambat, terjadi koreksi sehat, atau RSI mendingin dari overbought.
  - **40-60 → Premis bullish mulai goyah.** Tren menjadi sideways, muncul sinyal pelemahan awal (misal: volume menurun), atau harga kesulitan menembus resistance.
  - **< 40 → Premis bullish awal sudah tidak valid.** Tren telah berbalik, support penting telah ditembus, atau sinyal pembalikan bearish sangat kuat.

### INSTRUKSI PENGISIAN 'timeframe_analysis'
Untuk setiap timeframe, isi field-field berikut dengan informasi yang paling ringkas dan penting:
- **trend**: Pilih salah satu ENUM: "BULLISH", "BEARISH", "SIDEWAYS", "WEAKENING_BULLISH" (melemah), "REVERSING_TO_BEARISH" (pembalikan).
- **key_signal**: Tulis **SATU** sinyal atau peristiwa teknikal **paling signifikan** dalam bentuk frasa singkat (maksimal 7 kata). Contoh: "Breakout dari Ascending Triangle", "Candlestick Hammer di support", "Menembus resistance 1500", "RSI menunjukkan Bearish Divergence".
- **rsi**: Tulis nilai numerik RSI saja (contoh: 68).
- **support**: Tulis SATU level support terpenting dan terdekat.
- **resistance**: Tulis SATU level resistance terpenting dan terdekat.


### FORMAT OUTPUT WAJIB:
Hanya berikan **output dalam format JSON valid yang sangat terstruktur**, tanpa penjelasan tambahan. Ikuti struktur di bawah ini dengan seksama.
{
  "action": "HOLD|SELL|CUTLOSS",
  "exit_target_price": <float64 DEFAULT 0>,
  "exit_cut_loss_price": <float64 DEFAULT 0>,
  "reasoning": "<Penjelasan fokus pada PERUBAHAN kondisi dan justifikasi untuk aksi yang direkomendasikan>",
  "confidence_level": <int 0-100>,
  "technical_score": <int 0-100>,
  "timeframe_analysis": {
    "time_frame_1d": {
      "trend": "<ENUM>",
      "key_signal": "<Frasa singkat sinyal utama saat ini>",
      "rsi": <int>,
      "support": <float64>,
      "resistance": <float64>
    },
    "time_frame_4h": { /* ... struktur yang sama ... */ },
    "time_frame_1h": { /* ... struktur yang sama ... */ }
  }
}

### CATATAN
- Pastikan semua keputusan didasarkan pada kombinasi sinyal teknikal dan konteks berita, bukan berdasarkan perasaan atau prediksi jangka panjang. Jika indikator saling bertentangan, prioritaskan risk-reward dan waktu tersisa sebagai penentu akhir.
`, request.Symbol, request.Symbol, request.BuyPrice, request.BuyTime.Format("2006-01-02T15:04:05-07:00"),
		request.MaxHoldingPeriodDays, positionAgeDays, remainingDays, request.TargetPrice, request.StopLoss, stockData.MarketPrice,
		string(ohlcvJSON1D), string(ohlcvJSON4H), string(ohlcvJSON1H), newsSummaryText)

	return prompt
}

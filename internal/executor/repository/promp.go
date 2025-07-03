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
Evaluasi secara komprehensif apakah saham ini layak untuk posisi **BUY** saat ini untuk **swing trading (holding period 1-7 hari kerja)**. Analisis harus mencakup:
1.  **Analisa Multi-Timeframe (1D, 4H, 1H):** Untuk mengidentifikasi tren dominan dan keselarasan antar timeframe.
2.  **Analisa Kualitatif (Price Action):** Mengidentifikasi **pola candlestick** (misal: Bullish Engulfing, Hammer) dan **pola grafik** (misal: Triangle, Flag, Head and Shoulders).
3.  **Analisa Kuantitatif (Indikator):** Mengukur momentum dan kekuatan tren menggunakan EMA, MACD, RSI, Bollinger Bands, dan Volume.
4.  **Analisa Risiko/Imbalan (Risk/Reward):** Memastikan potensi keuntungan sepadan dengan risikonya.
5.  **Konteks Berita (jika tersedia):** Sebagai faktor pendukung atau penghambat.


### ATURAN WAJIB & KRITERIA KEPUTUSAN
- **Jangan gunakan pengetahuan eksternal.** Semua analisis HARUS didasarkan HANYA pada data OHLC dan berita yang disediakan di bawah ini.
- **Konsistensi Logis:** Semua angka, level support/resistance, dan kesimpulan harus konsisten dan dapat diverifikasi dari data yang diberikan.

#### Kriteria untuk "action": "BUY"
Berikan sinyal **BUY** HANYA JIKA **SEMUA** kondisi berikut terpenuhi:
1.  **Keselarasan Tren:** Timeframe 1D dan 4H menunjukkan tren **BULLISH** yang jelas. Timeframe 1H setidaknya netral atau menunjukkan sinyal reversal bullish.
2.  **Konfirmasi Pola:** Ditemukan **pola candlestick ATAU pola grafik bullish yang terkonfirmasi** pada timeframe 1D atau 4H. (Contoh: Breakout dari Ascending Triangle dengan volume tinggi, Bullish Engulfing di level support).
3.  **Dukungan Indikator:** Indikator EMA, MACD, dan RSI secara umum mendukung momentum bullish (tidak ada *strong bearish divergence*).
4.  **Risk/Reward Ratio (RRR):** Rasio imbalan terhadap risiko **WAJIB ≥ 3.0**. Hitung dengan rumus: (target_price - buy_price) / (buy_price - cut_loss).
5.  **Konteks Berita (Jika Ada):** Berita yang tersedia harus mendukung (impact bullish/netral dengan confidence score ≥ 0.7). Jika tidak ada berita, abaikan kriteria ini.

#### Kriteria untuk "action": "HOLD"
Berikan sinyal **HOLD** jika:
- Sinyal teknikal tidak selaras atau bertentangan (misalnya, 1D bullish tapi 4H bearish).
- Tren utama cenderung **SIDEWAYS** atau tidak jelas.
- Tidak ada pola konfirmasi bullish yang kuat.
- RRR < 3.0.
- Berita yang tersedia bersifat negatif atau bertentangan dengan sinyal teknikal.


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
Anda adalah **Manajer Risiko dan Analis Posisi** untuk swing trading. Tugas Anda adalah mengevaluasi posisi saham yang sedang aktif (%s) dan memberikan rekomendasi taktis yang jelas: **HOLD, TAKE_PROFIT, CUT_LOSS, atau TRAIL_STOP.**

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
Gunakan aturan ketat di bawah ini untuk menentukan "action".

- **HOLD**:
  - **Kondisi (Semua harus terpenuhi):**
    1.  **Struktur Tren:** Harga saat ini berada di atas MA20, DAN MA20 berada di atas MA50 pada timeframe 1D/4H.
    2.  **Momentum:** RSI berada di atas 50 dan tidak menunjukkan *bearish divergence* yang jelas.
    3.  **Keamanan:** Harga masih aman di atas level support terdekat (misal: *swing low* terakhir atau MA20).

- **TAKE_PROFIT**:
  - **Kondisi (Salah satu terpenuhi):**
    1.  **Target Tercapai:** Harga pasar (market_price) telah menyentuh atau melampaui target_price.
    2.  **Pelemahan Terkonfirmasi:** Harga mendekati target_price DAN muncul salah satu sinyal kuat berikut di timeframe 1D/4H:
        - Bearish Divergence yang jelas pada RSI atau MACD.
        - Muncul pola candlestick pembalikan kuat (Bearish Engulfing, Shooting Star).
        - Volume klimaks dimana harga gagal naik lebih lanjut.

- **CUT_LOSS**:
  - **Kondisi (Salah satu terpenuhi dengan konfirmasi):**
    1.  **Stop Loss Awal Ditembus:** Harga penutupan (close_price) berada di bawah stop_loss awal. Ini aturan absolut.
    2.  **Struktur Tren Patah:** Harga ditutup di bawah support krusial (MA50 pada 1D) selama 2 periode berturut-turut ATAU terjadi sinyal Death Cross (MA20 memotong ke bawah MA50).

- **TRAIL_STOP**:
  - **Kondisi (Posisi sudah profit DAN tren masih kuat):**
    1.  Telah tercapai Rasio Risk/Reward minimal 1:1.5 (current_price >= buy_price + (buy_price - stop_loss)).
    2.  Kondisi untuk HOLD masih terpenuhi (tren masih kuat).
  - **Tujuan:** Aksi ini adalah untuk **mengamankan profit** dengan menaikkan level exit_cut_loss_price, BUKAN untuk keluar dari pasar.


### INSTRUKSI PENGISIAN EXIT PRICE
Isi "exit_target_price" dan "exit_cut_loss_price" berdasarkan "action" yang direkomendasikan dan analisis teknikal terbaru:

- **Jika "action" == "HOLD":**
  - "exit_target_price": Pertahankan target awal jika masih realistis. Jika momentum sangat kuat dan ada ruang, Anda boleh menaikkannya ke level resistance berikutnya.
  - "exit_cut_loss_price": Pertahankan stop loss awal, kecuali ada level support baru yang lebih tinggi dan lebih kuat yang terbentuk.

- **Jika "action" == "TAKE_PROFIT":**
  - "exit_target_price": Set ke harga pasar saat ini atau level resistance terdekat di mana sinyal pelemahan muncul. Tujuannya adalah untuk "mengamankan keuntungan sekarang".
  - "exit_cut_loss_price": Tidak relevan, karena aksi adalah untuk menjual. Isi dengan nilai stop loss awal.

- **Jika "action" == "CUT_LOSS":**
  - "exit_target_price": Tidak relevan. Isi dengan nilai target awal.
  - "exit_cut_loss_price": Set ke harga pasar saat ini. Tujuannya adalah untuk "keluar dari pasar secepatnya".

- **Jika "action" == "TRAIL_STOP":**
  - "exit_target_price": Pertahankan target awal, karena premisnya adalah tren masih akan berlanjut.
  - "exit_cut_loss_price": **Ini adalah field paling penting.** Naikkan ke level yang strategis, seperti:
    - Sedikit di atas harga beli (breakeven).
    - Di bawah level support kunci terbaru yang lebih tinggi.
    - Menggunakan metode trailing stop (misal: di bawah EMA 20 timeframe 4H).


### INSTRUKSI PENGISIAN REASONING
- **Fokus pada Perubahan & Aturan:** Jelaskan **apa yang telah berubah** dan **aturan mana dari KRITERIA KEPUTUSAN** yang memicu rekomendasi Anda.
- **Justifikasi Aksi:** Berikan alasan teknikal yang jelas. Contoh: "Aksi adalah TRAIL_STOP karena R:R 1:1 telah tercapai dan harga masih kuat di atas MA20, sesuai kriteria."
- **Penyesuaian Stop:** Jika "action" adalah "TRAIL_STOP", jelaskan mengapa level "exit_cut_loss_price" yang baru itu dipilih. Contoh: "Stop loss dinaikkan ke 1100 (harga beli) untuk menghilangkan risiko kerugian."

### INSTRUKSI TEKNIS UNTUK PENGISIAN SKOR
Skor ini menilai **kesehatan posisi saat ini** dan **keyakinan pada aksi yang direkomendasikan**.

- **Field "confidence_level" (0-100):** Menunjukkan tingkat keyakinan atas "action" yang direkomendasikan (HOLD, SELL, dll.).
  - **> 80 → Sinyal sangat jelas dan searah.** Contoh: Untuk "HOLD", semua indikator masih sangat bullish. Untuk "SELL", sinyal pembalikan sangat terkonfirmasi.
  - **60-80 → Sinyal mayoritas mendukung**, tapi ada sedikit keraguan. Contoh: Untuk "HOLD", tren masih naik tapi RSI sudah overbought.
  - **40-60 → Sinyal campuran atau tidak jelas.** Ini seringkali menjadi alasan untuk "TRAIL_STOP" atau "HOLD" dengan waspada.
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
  "action": "HOLD|TAKE_PROFIT|CUT_LOSS|TRAIL_STOP",
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

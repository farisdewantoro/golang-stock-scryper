package telegram

import (
	"fmt"
	"strings"
	"time"

	"golang-stock-scryper/internal/entity"
	"golang-stock-scryper/internal/executor/dto"
	"golang-stock-scryper/pkg/utils"
)

// FormatNewsSummariesForTelegram formats a slice of NewsSummaryTelegramResult into multiple Markdown strings for Telegram,
// ensuring each message does not exceed the specified maximum length.
func FormatNewsSummariesForTelegram(summaries []dto.NewsSummaryTelegramResult) []string {
	if len(summaries) == 0 {
		return []string{"Tidak ada ringkasan berita saham untuk hari ini."}
	}

	const maxLen = 4090
	var messages []string
	var currentMessage strings.Builder
	part := 1

	// Helper function to start a new message part with the correct header
	startNewPart := func() {
		currentMessage.Reset()
		var header string
		if part == 1 {
			header = "📰 *Summary Berita Saham Harian* 📰\n\n"
		} else {
			header = fmt.Sprintf("---*Lanjutan Summary Berita Saham Harian Part %d*---\n\n", part)
		}
		currentMessage.WriteString(header)
	}

	// Start the first part
	startNewPart()

	for _, s := range summaries {
		var entryBuilder strings.Builder
		// --- Separator for each stock ---
		entryBuilder.WriteString(fmt.Sprintf("📈 *- - - - - %s - - - - -*\n", s.StockCode))

		// Short Summary
		entryBuilder.WriteString(fmt.Sprintf("💬 *Summary:* %s\n", s.ShortSummary))

		// Sentiment with icon
		var sentimentIcon string
		switch strings.ToLower(s.Sentiment) {
		case "positive", "bullish":
			sentimentIcon = "😊"
		case "negative", "bearish":
			sentimentIcon = "😟"
		default:
			sentimentIcon = "😐"
		}
		entryBuilder.WriteString(fmt.Sprintf("%s *Sentimen:* %s\n", sentimentIcon, s.Sentiment))

		// Suggested Action with icon
		var actionIcon string
		switch strings.ToLower(s.Action) {
		case "buy":
			actionIcon = "🟢"
		case "sell":
			actionIcon = "🔴"
		default: // Hold, Neutral
			actionIcon = "🟡"
		}
		entryBuilder.WriteString(fmt.Sprintf("%s *Action:* %s\n", actionIcon, s.Action))

		// Confidence Score
		entryBuilder.WriteString(fmt.Sprintf("🎯 *Confidence:* %.0f%%\n", s.ConfidenceScore*100))

		// Add a newline for spacing between entries
		entryBuilder.WriteString("\n")

		entryString := entryBuilder.String()

		// Check if adding the new entry exceeds the max length. We assume a single entry doesn't exceed the limit.
		if currentMessage.Len()+len(entryString) > maxLen {
			// Finalize the current message and add it to the slice
			messages = append(messages, currentMessage.String())

			// Start a new part
			part++
			startNewPart()
		}

		// Add the entry to the current message
		currentMessage.WriteString(entryString)
	}

	// Add the final message part to the slice
	messages = append(messages, currentMessage.String())

	return messages
}

// FormatStockNewsSummaryForTelegram formats the stock news summary into a Markdown string for Telegram.
func FormatStockNewsSummaryForTelegram(summary *entity.StockNewsSummary) string {
	var builder strings.Builder

	// --- Start of Summary ---
	builder.WriteString("--- 📰 *Stock News Summary* ---\n\n")

	builder.WriteString(fmt.Sprintf("📈 *Stock Code:* `%s`\n\n", summary.StockCode))

	// Sentiment with icon
	var sentimentIcon string
	switch summary.SummarySentiment {
	case "Positive":
		sentimentIcon = "😊"
	case "Negative":
		sentimentIcon = "😟"
	default:
		sentimentIcon = "😐"
	}
	builder.WriteString(fmt.Sprintf("%s *Sentiment:* %s\n", sentimentIcon, summary.SummarySentiment))

	// Impact with icon
	builder.WriteString(fmt.Sprintf("💥 *Impact:* %s\n", summary.SummaryImpact))

	// Confidence with icon
	builder.WriteString(fmt.Sprintf("🎯 *Confidence:* %.2f\n", summary.SummaryConfidenceScore))

	// Suggested Action with icon
	builder.WriteString(fmt.Sprintf("💡 *Suggested Action:* %s\n\n", summary.SuggestedAction))

	// Key Issues with icon
	builder.WriteString("🔑 *Key Issues:*\n")
	for _, issue := range summary.KeyIssues {
		builder.WriteString(fmt.Sprintf("  - %s\n", issue))
	}
	builder.WriteString("\n")

	// Reasoning with icon
	builder.WriteString(fmt.Sprintf("🤔 *Reasoning:*\n_%s_\n\n", summary.Reasoning))

	// --- End of Summary ---
	builder.WriteString("--- 🔚 *End of Summary* ---\n")

	return builder.String()
}

// AlertType represents the type of alert
type AlertType string

const (
	TakeProfit AlertType = "TAKE_PROFIT"
	StopLoss   AlertType = "STOP_LOSS"
)

// FormatStockAlertResultForTelegram formats the stock alert result into a Markdown string for Telegram.
func FormatStockAlertResultForTelegram(alertType AlertType, stockCode string, triggerPrice float64, targetPrice float64, timestamp int64) string {
	var builder strings.Builder

	var title, emoji string
	switch alertType {
	case TakeProfit:
		title = "Take Profit Triggered!"
		emoji = "🎯"
	case StopLoss:
		title = "Stop Loss Triggered!"
		emoji = "⚠️"
	default:
		title = "Price Alert"
		emoji = "🔔"
	}

	builder.WriteString(fmt.Sprintf("%s [%s] %s\n", emoji, stockCode, title))
	builder.WriteString(fmt.Sprintf("💰Harga menyentuh: %.3f (target: %.3f)\n", triggerPrice, targetPrice))
	builder.WriteString(fmt.Sprintf("%s\n", utils.PrettyDate(time.Unix(timestamp, 0))))
	return builder.String()
}

func FormatErrorAlertMessage(time time.Time, errType string, errMsg string, data string) string {
	return fmt.Sprintf(`📛 [ERROR ALERT] 
%s
🔧 %s
⚠️ %s	

📄 Data: %s
`, utils.PrettyDate(time), errType, errMsg, data)
}

func FormatAnalysisMessage(analysis *dto.IndividualAnalysisResponseMultiTimeframe) string {
	var sb strings.Builder
	signalIcon := "🟡"
	if analysis.Action == "BUY" {
		signalIcon = "🟢"
	}
	sb.WriteString(fmt.Sprintf("\n%s <b>SIGNAL %s: $%s</b> %s\n\n", signalIcon, analysis.Action, analysis.Symbol, signalIcon))
	// Recommendation
	if analysis.Action != "HOLD" {
		gain := float64(analysis.TargetPrice-analysis.BuyPrice) / float64(analysis.BuyPrice) * 100
		loss := float64(analysis.BuyPrice-analysis.CutLoss) / float64(analysis.BuyPrice) * 100
		sb.WriteString("<b>Trade Plan</b>\n")
		sb.WriteString(fmt.Sprintf("📌 Last Price: %d (%s)\n", int(analysis.MarketPrice), analysis.AnalysisDate.Format("01-02 15:04")))
		sb.WriteString(fmt.Sprintf("💵 Buy Area: $%d\n", int(analysis.BuyPrice)))
		sb.WriteString(fmt.Sprintf("🎯 Target Price: $%d %s\n", int(analysis.TargetPrice), utils.FormatPercentage(gain)))
		sb.WriteString(fmt.Sprintf("🛡 Cut Loss: $%d %s\n", int(analysis.CutLoss), utils.FormatPercentage(loss)))
		sb.WriteString(fmt.Sprintf("⚖️ Risk/Reward Ratio: %.2f\n", analysis.RiskRewardRatio))
		sb.WriteString(fmt.Sprintf("<i>⏳ Estimasi Waktu Profit: %d hari kerja</i>\n", analysis.EstimatedHoldingDays))
	} else if analysis.Action == "HOLD" {
		sb.WriteString("<b>Status saat ini</b>\n")
		sb.WriteString(fmt.Sprintf("📌 Last Price: %d (%s)\n", int(analysis.MarketPrice), analysis.AnalysisDate.Format("01-02 15:04")))
		if analysis.EstimatedHoldingDays > 0 {
			sb.WriteString(fmt.Sprintf("<i>🔍 Perkiraan Waktu Tunggu: %d hari kerja</i>\n", analysis.EstimatedHoldingDays))
		}
	}

	sb.WriteString("\n<b>Key Metrics</b>\n")
	sb.WriteString(fmt.Sprintf("📶 Confidence: %d%%\n", analysis.ConfidenceLevel))
	sb.WriteString(fmt.Sprintf("🔢 Technical Score: %d\n", analysis.TechnicalScore))

	// Reasoning
	sb.WriteString(fmt.Sprintf("\n🧠 <b>Reasoning:</b>\n%s\n\n", analysis.Reasoning))

	sb.WriteString("🔍 <b>Analisa Multi-Timeframe</b>")
	sb.WriteString(fmt.Sprintf("\n<b>Daily (1D)</b>: %s | RSI: %d\n", analysis.TimeframeAnalysis.Timeframe4H.Trend, analysis.TimeframeAnalysis.Timeframe1D.RSI))
	sb.WriteString(fmt.Sprintf("> Sinyal Kunci: %s\n", analysis.TimeframeAnalysis.Timeframe1D.KeySignal))
	sb.WriteString(fmt.Sprintf("> Support/Resistance: %d/%d\n", int(analysis.TimeframeAnalysis.Timeframe1D.Support), int(analysis.TimeframeAnalysis.Timeframe1D.Resistance)))

	sb.WriteString(fmt.Sprintf("\n<b>4 Hours (4H)</b>: %s | RSI: %d\n", analysis.TimeframeAnalysis.Timeframe4H.Trend, analysis.TimeframeAnalysis.Timeframe4H.RSI))
	sb.WriteString(fmt.Sprintf("> Sinyal Kunci: %s\n", analysis.TimeframeAnalysis.Timeframe4H.KeySignal))
	sb.WriteString(fmt.Sprintf("> Support/Resistance: %d/%d\n", int(analysis.TimeframeAnalysis.Timeframe4H.Support), int(analysis.TimeframeAnalysis.Timeframe4H.Resistance)))

	sb.WriteString(fmt.Sprintf("\n<b>1 Hour (1H)</b>: %s | RSI: %d\n", analysis.TimeframeAnalysis.Timeframe1H.Trend, analysis.TimeframeAnalysis.Timeframe1H.RSI))
	sb.WriteString(fmt.Sprintf("> Sinyal Kunci: %s\n", analysis.TimeframeAnalysis.Timeframe1H.KeySignal))
	sb.WriteString(fmt.Sprintf("> Support/Resistance: %d/%d\n", int(analysis.TimeframeAnalysis.Timeframe1H.Support), int(analysis.TimeframeAnalysis.Timeframe1H.Resistance)))

	// News Summary
	sb.WriteString("\n📰 <b>News Analysis:</b>\n")
	if analysis.NewsSummary.ConfidenceScore > 0 {
		sb.WriteString(fmt.Sprintf("Confidence Score: %.2f\n", analysis.NewsSummary.ConfidenceScore))
		sb.WriteString(fmt.Sprintf("Sentiment: %s\n", analysis.NewsSummary.Sentiment))
		sb.WriteString(fmt.Sprintf("Impact: %s\n\n", analysis.NewsSummary.Impact))
		sb.WriteString(fmt.Sprintf("🧠 News Insight: \n%s\n\n", analysis.NewsSummary.Reasoning))
	} else {
		sb.WriteString("<i>Belum ada data berita terbaru yang tersedia untuk saham ini.</i>\n\n")
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("📅 <i>Terakhir dianalisis: %s</i>\n", analysis.AnalysisDate.Format("2006-01-02 15:04:05")))

	return sb.String()
}

func FormatPositionMonitoringMessage(position *dto.PositionMonitoringResponseMultiTimeframe) string {
	var sb strings.Builder

	unrealizedPnLPercentage := ((position.MarketPrice - position.BuyPrice) / position.BuyPrice) * 100

	daysRemaining := utils.RemainingDays(position.MaxHoldingPeriodDays, position.BuyDate)
	ageDays := int(time.Since(position.BuyDate).Hours() / 24)

	iconAction := "❔"
	if position.Action == "HOLD" {
		iconAction = "🟡"
	} else if position.Action == "CUT_LOSS" {
		iconAction = "🔴"
	} else if position.Action == "TAKE_PROFIT" {
		iconAction = "🟢"
	} else if position.Action == "TRAIL_STOP" {
		iconAction = "🟠"
	}

	sb.WriteString(fmt.Sprintf("\n📊 <b>Position Update: %s</b>\n", position.Symbol))
	sb.WriteString(fmt.Sprintf("💰 Buy: $%d\n", int(position.BuyPrice)))
	sb.WriteString(fmt.Sprintf("📌 Last Price: $%d %s\n", int(position.MarketPrice), utils.FormatPercentage(unrealizedPnLPercentage)))
	sb.WriteString(fmt.Sprintf("🎯 TP: $%d | SL: $%d | RR: %.2f\n", int(position.TargetPrice), int(position.CutLoss), position.RiskRewardRatio))
	sb.WriteString(fmt.Sprintf("📈 Age: %d days | Remaining: %d days\n\n", ageDays, daysRemaining))

	// Recommendation
	gain := float64(position.ExitTargetPrice-position.BuyPrice) / float64(position.BuyPrice) * 100
	loss := float64(position.BuyPrice-position.ExitCutLossPrice) / float64(position.BuyPrice) * 100
	sb.WriteString("💡 <b>Recommendation:</b>\n")
	sb.WriteString(fmt.Sprintf(" • Action: %s %s\n", iconAction, position.Action))
	sb.WriteString(fmt.Sprintf(" • Target Price: $%d %s\n", int(position.ExitTargetPrice), utils.FormatPercentage(gain)))
	sb.WriteString(fmt.Sprintf(" • Stop Loss: $%d %s\n", int(position.ExitCutLossPrice), utils.FormatPercentage(loss)))
	sb.WriteString(fmt.Sprintf(" • Risk/Reward Ratio: %.2f\n", position.ExitRiskRewardRatio))
	sb.WriteString(fmt.Sprintf(" • Confidence: %d%%\n", position.ConfidenceLevel))
	sb.WriteString(fmt.Sprintf(" • Technical Score: %d\n\n", position.TechnicalScore))
	// Reasoning
	sb.WriteString(fmt.Sprintf("🧠 <b>Reasoning:</b>\n %s\n\n", position.Reasoning))

	// Technical Analysis
	sb.WriteString("🔍 <b>Analisa Multi-Timeframe</b>")
	sb.WriteString(fmt.Sprintf("\n<b>Daily (1D)</b>: %s | RSI: %d\n", position.TimeframeAnalysis.Timeframe4H.Trend, position.TimeframeAnalysis.Timeframe1D.RSI))
	sb.WriteString(fmt.Sprintf("> Sinyal Kunci: %s\n", position.TimeframeAnalysis.Timeframe1D.KeySignal))
	sb.WriteString(fmt.Sprintf("> Support/Resistance: %d/%d\n", int(position.TimeframeAnalysis.Timeframe1D.Support), int(position.TimeframeAnalysis.Timeframe1D.Resistance)))

	sb.WriteString(fmt.Sprintf("\n<b>4 Hours (4H)</b>: %s | RSI: %d\n", position.TimeframeAnalysis.Timeframe4H.Trend, position.TimeframeAnalysis.Timeframe4H.RSI))
	sb.WriteString(fmt.Sprintf("> Sinyal Kunci: %s\n", position.TimeframeAnalysis.Timeframe4H.KeySignal))
	sb.WriteString(fmt.Sprintf("> Support/Resistance: %d/%d\n", int(position.TimeframeAnalysis.Timeframe4H.Support), int(position.TimeframeAnalysis.Timeframe4H.Resistance)))

	sb.WriteString(fmt.Sprintf("\n<b>1 Hour (1H)</b>: %s | RSI: %d\n", position.TimeframeAnalysis.Timeframe1H.Trend, position.TimeframeAnalysis.Timeframe1H.RSI))
	sb.WriteString(fmt.Sprintf("> Sinyal Kunci: %s\n", position.TimeframeAnalysis.Timeframe1H.KeySignal))
	sb.WriteString(fmt.Sprintf("> Support/Resistance: %d/%d\n", int(position.TimeframeAnalysis.Timeframe1H.Support), int(position.TimeframeAnalysis.Timeframe1H.Resistance)))

	// News Summary
	sb.WriteString("\n📰 <b>News Analysis:</b>\n")
	if position.NewsSummary.ConfidenceScore > 0 {
		sb.WriteString(fmt.Sprintf("Confidence Score: %.2f\n", position.NewsSummary.ConfidenceScore))
		sb.WriteString(fmt.Sprintf("Sentiment: %s\n", position.NewsSummary.Sentiment))
		sb.WriteString(fmt.Sprintf("Impact: %s\n\n", position.NewsSummary.Impact))
		sb.WriteString(fmt.Sprintf("🧠 News Insight: \n%s\n\n", position.NewsSummary.Reasoning))
	} else {
		sb.WriteString("<i>Belum ada data berita terbaru yang tersedia untuk saham ini.</i>\n\n")
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("📅 <i>Terakhir dianalisis: %s</i>\n", position.AnalysisDate.Format("2006-01-02 15:04:05")))

	return sb.String()
}

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

func FormatAnalysisMessage(analysis *dto.IndividualAnalysisResponse) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("📊 **Analysis for %s**\n", analysis.Symbol))
	sb.WriteString(fmt.Sprintf("🎯 Signal: **%s**\n\n", analysis.Recommendation.Action))

	// Recommendation
	sb.WriteString("💡 **Recommendation:**\n")
	sb.WriteString(fmt.Sprintf("• 💵 Buy Price: $%.2f\n", analysis.Recommendation.BuyPrice))
	sb.WriteString(fmt.Sprintf("• 🎯 Target Price: $%.2f\n", analysis.Recommendation.TargetPrice))
	sb.WriteString(fmt.Sprintf("• 🛡 Stop Loss: $%.2f\n", analysis.Recommendation.CutLoss))
	sb.WriteString(fmt.Sprintf("• 🔁 Risk/Reward Ratio: %.2f\n", analysis.Recommendation.RiskRewardRatio))
	sb.WriteString(fmt.Sprintf("• 📊 Confidence: %d%%\n\n", analysis.Recommendation.ConfidenceLevel))
	// Reasoning
	sb.WriteString(fmt.Sprintf("🧠 **Reasoning:**\n %s\n\n", analysis.Recommendation.Reasoning))

	// Technical Analysis Summary
	sb.WriteString("🔧 **Technical Analysis:**\n")
	sb.WriteString(fmt.Sprintf("• Trend: %s \n", analysis.TechnicalAnalysis.Trend))
	sb.WriteString(fmt.Sprintf("• EMA Signal: %s\n", analysis.TechnicalAnalysis.EMASignal))
	sb.WriteString(fmt.Sprintf("• RSI: %s\n", analysis.TechnicalAnalysis.RSISignal))
	sb.WriteString(fmt.Sprintf("• MACD: %s\n", analysis.TechnicalAnalysis.MACDSignal))
	sb.WriteString(fmt.Sprintf("• Momentum: %s\n", analysis.TechnicalAnalysis.Momentum))
	sb.WriteString(fmt.Sprintf("• Bollinger Bands Position: %s\n", analysis.TechnicalAnalysis.BollingerBandsPosition))
	sb.WriteString(fmt.Sprintf("• Support Level: $%.2f\n", analysis.TechnicalAnalysis.SupportLevel))
	sb.WriteString(fmt.Sprintf("• Resistance Level: $%.2f\n", analysis.TechnicalAnalysis.ResistanceLevel))
	sb.WriteString(fmt.Sprintf("• Technical Score: %d/100\n", analysis.TechnicalAnalysis.TechnicalScore))
	if len(analysis.TechnicalAnalysis.KeyInsights) > 0 {
		sb.WriteString("\n📌 **Technical Insights:**\n")
		for _, insight := range analysis.TechnicalAnalysis.KeyInsights {
			sb.WriteString(fmt.Sprintf("• %s\n", utils.CapitalizeSentence(insight)))
		}
		sb.WriteString("\n")
	}

	// News Summary
	sb.WriteString("📰 **News Analysis:**\n")
	if analysis.NewsSummary.ConfidenceScore > 0 {
		sb.WriteString(fmt.Sprintf("Confidence Score: %.2f\n", analysis.NewsSummary.ConfidenceScore))
		sb.WriteString(fmt.Sprintf("Sentiment: %s\n", analysis.NewsSummary.Sentiment))
		sb.WriteString(fmt.Sprintf("Impact: %s\n\n", analysis.NewsSummary.Impact))
		sb.WriteString(fmt.Sprintf("🧠 News Insight: \n%s\n\n", analysis.NewsSummary.Reasoning))
	} else {
		sb.WriteString("_Belum ada data berita terbaru yang tersedia untuk saham ini._\n\n")
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("📅 _Terakhir dianalisis: %s_\n", analysis.AnalysisDate.Format("2006-01-02 15:04:05")))

	return sb.String()
}

func FormatPositionMonitoringMessage(position *dto.PositionMonitoringResponse) string {
	var sb strings.Builder

	unrealizedPnLPercentage := ((position.MarketPrice - position.BuyPrice) / position.BuyPrice) * 100
	unrealizedPnLPercentageStr := fmt.Sprintf("(+%.2f)", unrealizedPnLPercentage)

	if unrealizedPnLPercentage < 0 {
		unrealizedPnLPercentageStr = fmt.Sprintf("(%.2f)", unrealizedPnLPercentage)
	}

	daysRemaining := utils.RemainingDays(position.MaxHoldingPeriodDays, position.BuyDate)
	ageDays := int(time.Since(position.BuyDate).Hours() / 24)

	sb.WriteString(fmt.Sprintf("📊 **Position Update: %s**\n", position.Symbol))
	sb.WriteString(fmt.Sprintf("💰 Buy: $%.2f | Current: $%.2f %s\n", position.BuyPrice, position.MarketPrice, unrealizedPnLPercentageStr))
	sb.WriteString(fmt.Sprintf("📈 Age: %d days | Remaining: %d days\n\n", ageDays, daysRemaining))

	// Recommendation
	sb.WriteString("💡 **Recommendation:**\n")
	sb.WriteString(fmt.Sprintf("• 🎯 Target Price: $%.2f\n", position.Recommendation.TargetPrice))
	sb.WriteString(fmt.Sprintf("• 🛡 Stop Loss: $%.2f\n", position.Recommendation.CutLoss))
	sb.WriteString(fmt.Sprintf("• 🔁 Risk/Reward Ratio: %.2f\n", position.Recommendation.RiskRewardRatio))
	sb.WriteString(fmt.Sprintf("• 📊 Confidence: %d%%\n\n", position.Recommendation.ConfidenceLevel))
	// Reasoning
	sb.WriteString(fmt.Sprintf("🧠 **Reasoning:**\n %s\n\n", position.Recommendation.ExitReasoning))
	if len(position.Recommendation.ExitConditions) > 0 {
		sb.WriteString("💡 **Exit Conditions:**\n")
		for _, condition := range position.Recommendation.ExitConditions {
			sb.WriteString(fmt.Sprintf("• %s\n", condition))
		}
	}

	// Technical Analysis
	sb.WriteString("\n🔧 **Technical Analysis:**\n")
	sb.WriteString(fmt.Sprintf("• Trend: %s \n", position.TechnicalAnalysis.Trend))
	sb.WriteString(fmt.Sprintf("• EMA Signal: %s\n", position.TechnicalAnalysis.EMASignal))
	sb.WriteString(fmt.Sprintf("• RSI: %s\n", position.TechnicalAnalysis.RSISignal))
	sb.WriteString(fmt.Sprintf("• MACD: %s\n", position.TechnicalAnalysis.MACDSignal))
	sb.WriteString(fmt.Sprintf("• Momentum: %s\n", position.TechnicalAnalysis.Momentum))
	sb.WriteString(fmt.Sprintf("• Bollinger Bands Position: %s\n", position.TechnicalAnalysis.BollingerBandsPosition))
	sb.WriteString(fmt.Sprintf("• Support Level: $%.2f\n", position.TechnicalAnalysis.SupportLevel))
	sb.WriteString(fmt.Sprintf("• Resistance Level: $%.2f\n", position.TechnicalAnalysis.ResistanceLevel))
	sb.WriteString(fmt.Sprintf("• Technical Score: %d/100\n", position.TechnicalAnalysis.TechnicalScore))
	if len(position.TechnicalAnalysis.KeyInsights) > 0 {
		sb.WriteString("\n📌 **Technical Insights:**\n")
		for _, insight := range position.TechnicalAnalysis.KeyInsights {
			sb.WriteString(fmt.Sprintf("• %s\n", utils.CapitalizeSentence(insight)))
		}
		sb.WriteString("\n")
	}

	// News Summary
	sb.WriteString("📰 **News Analysis:**\n")
	if position.NewsSummary.ConfidenceScore > 0 {
		sb.WriteString(fmt.Sprintf("Confidence Score: %.2f\n", position.NewsSummary.ConfidenceScore))
		sb.WriteString(fmt.Sprintf("Sentiment: %s\n", position.NewsSummary.Sentiment))
		sb.WriteString(fmt.Sprintf("Impact: %s\n\n", position.NewsSummary.Impact))
		sb.WriteString(fmt.Sprintf("🧠 News Insight: \n%s\n\n", position.NewsSummary.Reasoning))
	} else {
		sb.WriteString("_Belum ada data berita terbaru yang tersedia untuk saham ini._\n\n")
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("📅 _Terakhir dianalisis: %s_\n", position.AnalysisDate.Format("2006-01-02 15:04:05")))

	return sb.String()
}

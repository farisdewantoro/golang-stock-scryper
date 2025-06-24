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
🕒 %s
🔧 %s
⚠️ %s	

📄 %s
`, utils.PrettyDate(time), errType, errMsg, data)
}

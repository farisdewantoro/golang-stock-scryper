package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Notifier defines the interface for a Telegram notifier.
type Notifier interface {
	SendMessage(text string) error
}

// client is an implementation of Notifier.
type client struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

// NewClient creates a new Telegram notifier client.
func NewClient(botToken string, chatID int64) (Notifier, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, err
	}
	return &client{
		bot:    bot,
		chatID: chatID,
	}, nil
}

// SendMessage sends a message to the configured Telegram chat.
func (c *client) SendMessage(text string) error {
	msg := tgbotapi.NewMessage(c.chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown // Using Markdown for formatting
	_, err := c.bot.Send(msg)
	return err
}

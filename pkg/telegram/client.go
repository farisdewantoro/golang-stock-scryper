package telegram

import (
	"golang-stock-scryper/pkg/utils"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Notifier defines the interface for a Telegram notifier.
type Notifier interface {
	SendMessage(text string, msgConfig ...tgbotapi.MessageConfig) error
	SendMessageUser(text string, chatID int64, msgConfig ...tgbotapi.MessageConfig) error
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
func (c *client) SendMessage(text string, msgConfig ...tgbotapi.MessageConfig) error {
	parseMode := tgbotapi.ModeMarkdownV2

	if len(msgConfig) > 0 {
		parseMode = msgConfig[0].ParseMode
	}
	if parseMode == tgbotapi.ModeMarkdownV2 || parseMode == tgbotapi.ModeMarkdown {
		text = utils.EscapeMarkdownV2(text)
	}

	msg := tgbotapi.NewMessage(c.chatID, text)
	msg.ParseMode = parseMode
	_, err := c.bot.Send(msg)
	return err
}

// SendMessageUser sends a message to user
func (c *client) SendMessageUser(text string, chatID int64, msgConfig ...tgbotapi.MessageConfig) error {
	parseMode := tgbotapi.ModeMarkdownV2

	if len(msgConfig) > 0 {
		parseMode = msgConfig[0].ParseMode
	}
	if parseMode == tgbotapi.ModeMarkdownV2 || parseMode == tgbotapi.ModeMarkdown {
		text = utils.EscapeMarkdownV2(text)
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMode
	_, err := c.bot.Send(msg)
	return err
}

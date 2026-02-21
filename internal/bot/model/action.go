package model

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// Action — действие пользователя для обработки воркером.
type Action struct {
	ChatID  int64
	Command string
	Text    string
	Message *tgbotapi.Message
}

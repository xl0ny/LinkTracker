package domain

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type Command interface {
	Name() string
	Description() string
	Handle(action Action, api *tgbotapi.BotAPI) (response string, send bool)
}

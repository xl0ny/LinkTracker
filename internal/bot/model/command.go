package model

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// Command — контракт обработчика команды. Description используется для меню (setMyCommands).
type Command interface {
	Name() string
	Description() string // краткое описание для меню команд в Telegram (3–256 символов)
	Handle(action Action, api *tgbotapi.BotAPI) (response string, send bool)
}

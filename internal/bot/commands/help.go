package commands

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

type Help struct{}

func (Help) Name() string { return "help" }

func (Help) Description() string { return "Список доступных команд" }

func (Help) Handle(action model.Action, api *tgbotapi.BotAPI) (string, bool) {
	return "/start — начало работы\n/help — список команд", true
}

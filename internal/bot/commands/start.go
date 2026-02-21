package commands

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

type Start struct{}

func (Start) Name() string { return "start" }

func (Start) Description() string { return "Начать работу с ботом" }

func (Start) Handle(action model.Action, api *tgbotapi.BotAPI) (string, bool) {
	return "Добро пожаловать! Используйте /help для списка команд.", true
}

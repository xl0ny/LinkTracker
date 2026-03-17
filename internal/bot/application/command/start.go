package command

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Start struct{}

func (Start) Name() string { return "start" }

func (Start) Description() string { return "Начать работу с ботом" }

func (Start) Handle(_ domain.Action) (string, bool) {
	return "Добро пожаловать! Используйте /help для списка команд.", true
}

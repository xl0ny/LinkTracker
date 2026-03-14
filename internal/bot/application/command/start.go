package command

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Start struct {
	tracker application.LinkTracker
}

func NewStart(tracker application.LinkTracker) *Start {
	return &Start{tracker: tracker}
}

func (s *Start) Name() string { return "start" }

func (s *Start) Description() string { return "Начать работу с ботом" }

func (s *Start) Handle(action domain.Action) (string, bool) {
	if s.tracker != nil {
		_ = s.tracker.RegisterChat(context.Background(), action.ChatID)
	}
	return "Добро пожаловать! Используйте /help для списка команд.", true
}

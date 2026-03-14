package command

import (
	"context"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Untrack struct {
	tracker application.LinkTracker
}

func NewUntrack(tracker application.LinkTracker) *Untrack {
	return &Untrack{tracker: tracker}
}

func (u *Untrack) Name() string { return "untrack" }

func (u *Untrack) Description() string {
	return "Прекратить отслеживание ссылки"
}

func (u *Untrack) Handle(action domain.Action) (string, bool) {
	link := strings.TrimSpace(action.Args)
	if link == "" {
		return "Укажите ссылку: /untrack <ссылка>", true
	}
	if err := u.tracker.RemoveLink(context.Background(), action.ChatID, link); err != nil {
		if err.Error() == "link not found" {
			return "Ссылка не найдена в отслеживаемых", true
		}
		if err.Error() == errChatNotFound {
			return msgUseStart, true
		}
		return "Ошибка удаления ссылки", true
	}
	return "Ссылка удалена из отслеживания", true
}

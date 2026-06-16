package command

import (
	"context"
	"errors"
	"fmt"
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

func (u *Untrack) Name() string { return commandUntrack }

func (u *Untrack) Description() string {
	return "Прекратить отслеживание ссылки"
}

func (u *Untrack) Handle(action domain.Action) (string, error) {
	link := strings.TrimSpace(action.Args)
	if link == "" {
		return "Укажите ссылку: /untrack <ссылка>", nil
	}
	if err := u.tracker.RemoveLink(context.Background(), action.ChatID, link); err != nil {
		if errors.Is(err, application.ErrLinkNotFound) {
			return "Ссылка не найдена в отслеживаемых", nil
		}
		if errors.Is(err, application.ErrChatNotFound) {
			return msgUseStart, nil
		}
		return "", fmt.Errorf("remove link: %w", err)
	}
	return "Ссылка удалена из отслеживания", nil
}

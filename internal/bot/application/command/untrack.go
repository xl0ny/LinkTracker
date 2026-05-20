package command

import (
	"context"
	"fmt"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type linkRemover interface {
	RemoveLink(ctx context.Context, chatID int64, link string) error
}

type Untrack struct {
	tracker linkRemover
}

func NewUntrack(tracker linkRemover) *Untrack {
	return &Untrack{tracker: tracker}
}

func (u *Untrack) Name() string { return "untrack" }

func (u *Untrack) Description() string {
	return "Прекратить отслеживание ссылки"
}

func (u *Untrack) Handle(action domain.Action) (string, error) {
	link := strings.TrimSpace(action.Args)
	if link == "" {
		return "Укажите ссылку: /untrack <ссылка>", nil
	}
	if err := u.tracker.RemoveLink(context.Background(), action.ChatID, link); err != nil {
		if err.Error() == "link not found" {
			return "Ссылка не найдена в отслеживаемых", nil
		}
		if err.Error() == errChatNotFound {
			return msgUseStart, nil
		}
		return "", fmt.Errorf("remove link: %w", err)
	}
	return "Ссылка удалена из отслеживания", nil
}

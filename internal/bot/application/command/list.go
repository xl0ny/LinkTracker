package command

import (
	"context"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const errChatNotFound = "chat not found"
const msgUseStart = "Сначала используйте /start"

type List struct {
	tracker application.LinkTracker
}

func NewList(tracker application.LinkTracker) *List {
	return &List{tracker: tracker}
}

func (l *List) Name() string { return "list" }

func (l *List) Description() string { return "Список отслеживаемых ссылок" }

func (l *List) Handle(action domain.Action) (string, bool) {
	tagFilter := strings.TrimSpace(action.Args)
	links, err := l.tracker.ListLinks(context.Background(), action.ChatID, tagFilter)
	if err != nil {
		if err.Error() == errChatNotFound {
			return msgUseStart, true
		}
		return "Ошибка получения списка", true
	}
	if len(links) == 0 {
		return "Нет отслеживаемых ссылок", true
	}
	var b strings.Builder
	for i, link := range links {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("• ")
		b.WriteString(link.URL)
		if len(link.Tags) > 0 {
			b.WriteString(" [")
			b.WriteString(strings.Join(link.Tags, ", "))
			b.WriteString("]")
		}
	}
	return b.String(), true
}

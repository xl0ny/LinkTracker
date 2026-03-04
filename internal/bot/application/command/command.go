package command

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Command interface {
	Name() string
	Description() string
	Handle(action domain.Action) (response string, send bool)
}

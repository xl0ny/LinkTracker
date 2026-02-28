package application

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

// Dispatcher routes actions to registered commands.
type Dispatcher struct {
	handlers map[string]domain.Command
}

// NewDispatcher builds a dispatcher from the given commands.
func NewDispatcher(cmds []domain.Command) *Dispatcher {
	h := make(map[string]domain.Command, len(cmds))
	for _, c := range cmds {
		h[c.Name()] = c
	}
	return &Dispatcher{handlers: h}
}

// Dispatch runs the command handler or returns an error message for unknown commands.
func (d *Dispatcher) Dispatch(action domain.Action, api *tgbotapi.BotAPI) (text string, send bool) {
	if action.Command == "" {
		return "", false
	}
	if cmd, ok := d.handlers[action.Command]; ok {
		return cmd.Handle(action, api)
	}
	return "Неизвестная команда. Используйте /help для списка команд", true
}

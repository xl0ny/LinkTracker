package application

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Dispatcher struct {
	handlers map[string]commands.Command
}

func NewDispatcher(cmds []commands.Command) *Dispatcher {
	h := make(map[string]commands.Command, len(cmds))
	for _, c := range cmds {
		h[c.Name()] = c
	}
	return &Dispatcher{handlers: h}
}

func (d *Dispatcher) Dispatch(action domain.Action) (text string, send bool) {
	if action.Command == "" {
		return "", false
	}
	if cmd, ok := d.handlers[action.Command]; ok {
		return cmd.Handle(action)
	}
	return "Неизвестная команда. Используйте /help для списка команд", true
}

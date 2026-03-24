package application

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Command interface {
	Name() string
	Description() string
	Handle(action domain.Action) (response string, err error)
}

type PlainMessageHandler interface {
	HandlePlainMessage(action domain.Action) (response string, err error)
}

type Dispatcher struct {
	handlers map[string]Command
	plain    PlainMessageHandler
	state    *TrackStateStore
}

func NewDispatcher(cmds []Command, plain PlainMessageHandler, state *TrackStateStore) *Dispatcher {
	h := make(map[string]Command, len(cmds))
	for _, c := range cmds {
		h[c.Name()] = c
	}
	return &Dispatcher{handlers: h, plain: plain, state: state}
}

func (d *Dispatcher) Dispatch(action domain.Action) (text string, err error) {
	if action.Command != "" {
		if action.Command == "cancel" {
			d.state.Clear(action.ChatID)
			return "Отменено.", nil
		}
		if cmd, ok := d.handlers[action.Command]; ok {
			if action.Command != "track" {
				d.state.Clear(action.ChatID)
			}
			return cmd.Handle(action)
		}
		d.state.Clear(action.ChatID)
		return "Неизвестная команда. Используйте /help для списка команд", nil
	}
	if d.plain != nil {
		return d.plain.HandlePlainMessage(action)
	}
	return "", nil
}

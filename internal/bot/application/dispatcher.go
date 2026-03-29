package application

import (
	"fmt"

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

func (d *Dispatcher) Dispatch(action domain.Action) (string, error) {
	if action.Command != "" {
		return d.dispatchNamedCommand(action)
	}
	if d.plain != nil {
		msg, handleErr := d.plain.HandlePlainMessage(action)
		if handleErr != nil {
			return "", fmt.Errorf("plain message: %w", handleErr)
		}
		return msg, nil
	}
	return "", nil
}

func (d *Dispatcher) dispatchNamedCommand(action domain.Action) (string, error) {
	if action.Command == "cancel" {
		d.state.Clear(action.ChatID)
		return "Отменено.", nil
	}
	if cmd, ok := d.handlers[action.Command]; ok {
		if action.Command != "track" {
			d.state.Clear(action.ChatID)
		}
		msg, handleErr := cmd.Handle(action)
		if handleErr != nil {
			return "", fmt.Errorf("command %q: %w", action.Command, handleErr)
		}
		return msg, nil
	}
	d.state.Clear(action.ChatID)
	return "Неизвестная команда. Используйте /help для списка команд", nil
}

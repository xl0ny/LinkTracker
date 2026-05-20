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
	state    TrackStateStore
}

func NewDispatcher(cmds []Command, plain PlainMessageHandler, state TrackStateStore) *Dispatcher {
	h := make(map[string]Command, len(cmds))
	for _, c := range cmds {
		h[c.Name()] = c
	}
	return &Dispatcher{handlers: h, plain: plain, state: state}
}

func (d *Dispatcher) Dispatch(action domain.Action) (string, error) {
	if action.Command != "" {
		return d.dispatchSlashCommand(action)
	}
	return d.dispatchPlain(action)
}

func (d *Dispatcher) dispatchSlashCommand(action domain.Action) (string, error) {
	if action.Command == "cancel" {
		d.state.Clear(action.ChatID)
		return "Отменено.", nil
	}
	cmd, ok := d.handlers[action.Command]
	if !ok {
		d.state.Clear(action.ChatID)
		return "Неизвестная команда. Используйте /help для списка команд", nil
	}
	if action.Command != "track" {
		d.state.Clear(action.ChatID)
	}
	resp, handleErr := cmd.Handle(action)
	if handleErr != nil {
		return resp, fmt.Errorf("command %s: %w", action.Command, handleErr)
	}
	return resp, nil
}

func (d *Dispatcher) dispatchPlain(action domain.Action) (string, error) {
	if d.plain == nil {
		return "", nil
	}
	resp, plainErr := d.plain.HandlePlainMessage(action)
	if plainErr != nil {
		return resp, fmt.Errorf("plain message: %w", plainErr)
	}
	return resp, nil
}

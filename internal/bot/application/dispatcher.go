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
	metrics  commandMetrics
}

type commandMetrics interface {
	IncCommand(command string)
}

func NewDispatcher(cmds []Command, plain PlainMessageHandler, state *TrackStateStore, m commandMetrics) *Dispatcher {
	h := make(map[string]Command, len(cmds))
	for _, c := range cmds {
		h[c.Name()] = c
	}
	return &Dispatcher{handlers: h, plain: plain, state: state, metrics: m}
}

func (d *Dispatcher) Dispatch(action domain.Action) (text string, err error) {
	switch {
	case action.Command != "":
		return d.dispatchCommand(action)
	default:
		return d.dispatchPlain(action)
	}
}

func (d *Dispatcher) dispatchCommand(action domain.Action) (string, error) {
	if d.metrics != nil {
		d.metrics.IncCommand(action.Command)
	}
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
	resp, hErr := cmd.Handle(action)
	if hErr != nil {
		return "", fmt.Errorf("dispatcher: command %s: %w", action.Command, hErr)
	}
	return resp, nil
}

func (d *Dispatcher) dispatchPlain(action domain.Action) (string, error) {
	if d.plain == nil {
		return "", nil
	}
	resp, pErr := d.plain.HandlePlainMessage(action)
	if pErr != nil {
		return "", fmt.Errorf("dispatcher: plain message: %w", pErr)
	}
	return resp, nil
}

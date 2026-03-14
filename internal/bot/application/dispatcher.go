package application

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Command interface {
	Name() string
	Description() string
	Handle(action domain.Action) (response string, send bool)
}

type TrackContinuationHandler interface {
	HandleContinuation(ctx context.Context, action domain.Action, state *TrackState) (response string, send bool, newState *TrackState)
}

type Dispatcher struct {
	handlers map[string]Command
	track    TrackContinuationHandler
	state    *TrackStateStore
}

func NewDispatcher(cmds []Command, track TrackContinuationHandler, state *TrackStateStore) *Dispatcher {
	h := make(map[string]Command, len(cmds))
	for _, c := range cmds {
		h[c.Name()] = c
	}
	return &Dispatcher{handlers: h, track: track, state: state}
}

func (d *Dispatcher) Dispatch(action domain.Action) (text string, send bool) {
	if action.Command != "" {
		d.state.Clear(action.ChatID)
		if action.Command == "cancel" {
			return "Отменено.", true
		}
		if cmd, ok := d.handlers[action.Command]; ok {
			return cmd.Handle(action)
		}
		return "Неизвестная команда. Используйте /help для списка команд", true
	}
	st := d.state.Get(action.ChatID)
	if st != nil && d.track != nil {
		resp, doSend, newSt := d.track.HandleContinuation(context.Background(), action, st)
		d.state.Set(action.ChatID, newSt)
		return resp, doSend
	}
	return "", false
}

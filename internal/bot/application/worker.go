package application

import (
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type sender interface {
	SendMessage(chatid int64, message string) error
}

func Worker(actions <-chan domain.Action, d *Dispatcher, s sender) {
	for action := range actions {
		text, err := d.Dispatch(action)
		if err != nil {
			slog.Error("worker: dispatch error", slog.String("error", err.Error()), slog.String("command", action.Command), slog.Int64("chat_id", action.ChatID))
		}
		if text == "" {
			continue
		}
		err = s.SendMessage(action.ChatID, text)
		if err != nil {
			slog.Warn("worker: send message error", slog.String("error", err.Error()), slog.String("command", action.Command), slog.Int64("chat_id", action.ChatID))
		}
		slog.Info("worker: command processed", slog.String("command", action.Command), slog.Int64("chat_id", action.ChatID))
	}
}

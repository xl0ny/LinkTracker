package application

import (
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func Worker(actions <-chan domain.Action, d *Dispatcher, s sender) {
	for action := range actions {
		text, send := d.Dispatch(action)
		if !send {
			continue
		}
		err := s.SendMessage(action.ChatID, text)
		if err != nil {
			slog.Warn("failed to send message", slog.String("error", err.Error()), slog.String("command", action.Command), slog.Int64("chat_id", action.ChatID), slog.String("event", "send_message"))
		}
		slog.Info("command processed", slog.String("command", action.Command), slog.Int64("chat_id", action.ChatID), slog.String("event", "command_handled"))
	}
}

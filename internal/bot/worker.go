package bot

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

// Worker обрабатывает действия из канала через диспетчер. Блокирующая горутина.
func Worker(actions <-chan model.Action, api *tgbotapi.BotAPI, d *Dispatcher) {
	for action := range actions {
		text, send := d.Dispatch(action, api)
		if !send {
			continue
		}
		msg := tgbotapi.NewMessage(action.ChatID, text)
		_, err := api.Send(msg)
		if err != nil {
			slog.Warn("ошибка отправки сообщения", slog.String("error", err.Error()), slog.String("command", action.Command), slog.Int64("chat_id", action.ChatID), slog.String("event", "send_message"))
			continue
		}
		slog.Info("команда обработана", slog.String("command", action.Command), slog.Int64("chat_id", action.ChatID), slog.String("event", "command_handled"))
	}
}

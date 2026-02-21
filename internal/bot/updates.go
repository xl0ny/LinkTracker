package bot

import (
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

// ReceiveUpdates получает апдейты из API и отправляет действия в канал. Блокирующая горутина.
func ReceiveUpdates(api *tgbotapi.BotAPI, actions chan<- model.Action) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := api.GetUpdatesChan(u)
	slog.Info("приём обновлений запущен", slog.String("event", "updates_started"))

	for update := range updates {
		if update.Message == nil {
			continue
		}
		actions <- model.Action{
			ChatID:  update.Message.Chat.ID,
			Command: update.Message.Command(),
			Text:    update.Message.Text,
			Message: update.Message,
		}
	}
}

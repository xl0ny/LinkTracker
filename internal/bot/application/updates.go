package application

import (
	"context"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

// ReceiveUpdates receives updates from the API and sends actions to the channel.
func ReceiveUpdates(ctx context.Context, api *tgbotapi.BotAPI) <-chan domain.Action {
	ch := make(chan domain.Action, actionBuf)

	u := tgbotapi.NewUpdate(0)
	updates := api.GetUpdatesChan(u)

	go func() {
		slog.Info("updates receiving started", slog.String("event", "updates_started"))
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case update := <-updates:
				if update.Message == nil {
					continue
				}
				ch <- domain.Action{
					ChatID:  update.Message.Chat.ID,
					Command: update.Message.Command(),
					Text:    update.Message.Text,
				}
			}
		}
	}()
	return ch
}

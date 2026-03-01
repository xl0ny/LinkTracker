package telegram

import (
	"context"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const actionBuf = 100

type bot struct {
	api *tgbotapi.BotAPI
}

func NewBot(api *tgbotapi.BotAPI) *bot {
	return &bot{
		api: api,
	}
}

func (bot *bot) SendMessage(chatid int64, message string) error {
	msg := tgbotapi.NewMessage(chatid, message)
	_, err := bot.api.Send(msg)
	return err
}

func (bot *bot) ReceiveUpdates(ctx context.Context) <-chan domain.Action {
	ch := make(chan domain.Action, actionBuf)

	u := tgbotapi.NewUpdate(0)
	apiUpdates := bot.api.GetUpdatesChan(u)

	go func() {
		slog.Info("updates receiving started", slog.String("event", "updates_started"))
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				close(ch)
				return
			case update := <-apiUpdates:
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

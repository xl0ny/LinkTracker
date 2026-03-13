package telegram

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	commands "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

const actionBuf = 100

type Bot struct {
	api *tgbotapi.BotAPI
}

func NewBot(telegramToken string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		return nil, fmt.Errorf("new bot: %w", err)
	}

	return &Bot{
		api: api,
	}, nil
}

func (bot *Bot) SendMessage(chatid int64, message string) error {
	msg := tgbotapi.NewMessage(chatid, message)
	_, err := bot.api.Send(msg)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

func (bot *Bot) ReceiveUpdates(ctx context.Context) <-chan domain.Action {
	ch := make(chan domain.Action, actionBuf)

	u := tgbotapi.NewUpdate(0)
	apiUpdates := bot.api.GetUpdatesChan(u)

	go func(ctx context.Context) {
		slog.Info("updates receiving started", slog.String("event", "updates_started"))
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case update := <-apiUpdates:
				if update.Message == nil {
					continue
				}
				ch <- domain.Action{
					ChatID:  update.Message.Chat.ID,
					Command: update.Message.Command(),
					Text:    update.Message.Text,
					Args:    update.Message.CommandArguments(),
				}
			}
		}
	}(ctx)
	return ch
}

func (bot *Bot) SetMenuCommands(cmds []commands.Command) {
	botCommands := make([]tgbotapi.BotCommand, 0, len(cmds))
	for _, c := range cmds {
		botCommands = append(botCommands, tgbotapi.BotCommand{
			Command:     c.Name(),
			Description: c.Description(),
		})
	}
	resp, err := bot.api.Request(tgbotapi.NewSetMyCommands(botCommands...))
	if err != nil {
		slog.Warn("failed to set commands menu", slog.String("error", err.Error()), slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
		return
	}
	if !resp.Ok {
		slog.Warn("setMyCommands API response not Ok", slog.String("description", resp.Description), slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
		return
	}
	slog.Info("commands menu set", slog.String("event", "set_commands"))
}

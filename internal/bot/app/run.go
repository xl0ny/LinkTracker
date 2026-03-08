package app

import (
	"context"
	"log/slog"
	"os"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
)

const workerCount = 5

func Run(ctx context.Context, cfg *config.Config) {
	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		slog.Error("bot initialization", slog.String("error", err.Error()), slog.String("event", "bot_init"))
		os.Exit(1)
	}

	commands := []application.Command{
		command.Start{},
		command.Help{},
	}
	bot.SetMenuCommands(commands)

	actions := bot.ReceiveUpdates(ctx)
	dispatcher := application.NewDispatcher(commands)

	for range workerCount {
		go application.Worker(actions, dispatcher, bot)
	}
	<-ctx.Done()
}

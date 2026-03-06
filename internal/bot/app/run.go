package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
)

const workerCount = 5

func Run(ctx context.Context, cfg *config.Config) {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bot := telegram.NewBot(cfg.TelegramToken)

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
	slog.Info("shutting down", slog.String("event", "shutdown"))
	os.Exit(0)
}

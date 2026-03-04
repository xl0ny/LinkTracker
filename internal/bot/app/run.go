package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	commands "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/pkg/config"
)

const workerCount = 5

func Run(ctx context.Context, cfg *config.Config) {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	bot := telegram.NewBot(cfg.TelegramToken)

	bot.SetMenuCommands(commands.All())

	actions := bot.ReceiveUpdates(ctx)
	dispatcher := application.NewDispatcher(commands.All())

	for range workerCount {
		go application.Worker(actions, dispatcher, bot)
	}
	<-ctx.Done()
	slog.Info("shutting down", slog.String("event", "shutdown"))
	os.Exit(0)
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/app"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()), slog.String("stage", "config_load"))
		os.Exit(1)
	}
	level := cfg.Logging.GetLevel()
	slog.SetDefault(slog.New(logging.NewHandler(level)))
	slog.Info("level set", "level", level)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := app.Run(ctx, cfg); err != nil {
		slog.Error("bot initialization", slog.String("error", err.Error()), slog.String("event", "bot_init"))
	}

	slog.Info("shutting down", slog.String("event", "shutdown"))
	os.Exit(0)
}

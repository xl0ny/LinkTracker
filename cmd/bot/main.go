package main

import (
	"context"
	"log/slog"
	"os"

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
	level := cfg.GetLevel()
	slog.SetDefault(slog.New(logging.NewHandler(level)))
	slog.Info("level set", "level", level)

	ctx := context.Background()
	app.Run(ctx, cfg)
}

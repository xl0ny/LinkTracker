package main

import (
	"context"
	"log/slog"
	"os"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/app"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("bot main: config load error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	level := cfg.GetLevel()
	slog.SetDefault(slog.New(logging.NewHandler(level)))
	slog.Info("bot main: log level set", slog.String("level", level.String()))

	ctx := context.Background()
	if runErr := app.Run(ctx, cfg); runErr != nil {
		slog.Error("bot main: run error", slog.String("error", runErr.Error()))
		os.Exit(1)
	}
}

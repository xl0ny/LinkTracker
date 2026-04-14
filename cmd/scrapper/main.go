package main

import (
	"context"
	"log/slog"
	"os"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/logging"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/app"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("scrapper main: config load error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	level := cfg.GetLevel()
	slog.SetDefault(slog.New(logging.NewHandler(level)))
	slog.Info("scrapper main: log level set", slog.String("level", level.String()))

	ctx := context.Background()
	if runErr := app.Run(ctx, cfg); runErr != nil {
		slog.Error("scrapper main: run error", slog.String("error", runErr.Error()))
		os.Exit(1)
	}
}

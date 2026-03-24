package main

import (
	"context"
	"log/slog"
	"os"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/app"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx := context.Background()
	if runErr := app.Run(ctx, cfg); runErr != nil {
		slog.Error("scrapper run failed", slog.String("error", runErr.Error()))
		os.Exit(1)
	}
}

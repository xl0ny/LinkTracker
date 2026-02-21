package main

import (
	"log/slog"
	"os"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot"
)

func main() {
	var level slog.Level

	if os.Getenv("DEBUG") == "1" {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	cfg, err := bot.Load()
	if err != nil {
		slog.Error("не удалось загрузить конфиг", slog.String("error", err.Error()), slog.String("stage", "config_load"))
		os.Exit(1)
	}
	bot.Run(cfg)
}

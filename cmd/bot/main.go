package main

import (
	"log/slog"
	"os"

	"github.com/goccy/go-yaml"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/infrastructure/logging"
)

func main() {
	var level slog.Level
	var loggingConfig config.LoggingConfig
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()), slog.String("stage", "config_load"))
	}
	
	err = yaml.Unmarshal(data, &loggingConfig)
	if os.Getenv("DEBUG") == "1" {	
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(logging.NewHandler(level)))

	cfg, err := bot.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()), slog.String("stage", "config_load"))
		os.Exit(1)
	}
	bot.Run(cfg)
}

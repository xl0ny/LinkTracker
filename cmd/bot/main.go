package main

import (
	"log/slog"
	"os"

	"github.com/goccy/go-yaml"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/logging"
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

	var logger logging.LoggerI
	logger = logging.NewLogger(level)
	slog.SetDefault(slog.New(logger))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()), slog.String("stage", "config_load"))
		os.Exit(1)
	}
	application.Run(cfg)
}

package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	TelegramToken string `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`
	LoggingLevel  slog.Level
}

type LoggingConfig struct {
	Logging struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("config loading error:")
	}

	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	config.TelegramToken = strings.TrimSpace(config.TelegramToken)
	if !isValidTelegramTokenFormat(config.TelegramToken) {
		return nil, fmt.Errorf("APP_TELEGRAM_TOKEN invalid format (expected <number>:<string>)")
	}

	//logginпg level
	var loggingConfig LoggingConfig
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		slog.Error("failed to load config, setted level INFO", slog.String("error", err.Error()), slog.String("stage", "config_load"))
	}

	err = yaml.Unmarshal(data, &loggingConfig)
	if err != nil {
		slog.Error("logging format in confilg.yaml incorrect, setted level INFO", slog.String("error", err.Error()), slog.String("stage", "config_load"))
	}
	if strings.EqualFold(loggingConfig.Logging.Mode, "DEBUG") {
		config.LoggingLevel = slog.LevelDebug
		slog.Info("setted DEBUG level")
	} else {
		config.LoggingLevel = slog.LevelInfo
		slog.Info("setted INFO level")
	}

	return &config, nil
}

func isValidTelegramTokenFormat(token string) bool {
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return false
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(parts[0]) > 0 && len(parts[1]) > 0
}

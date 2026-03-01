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

type configRaw struct {
	TelegramToken string `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`
}

type Config struct {
	TelegramToken string
	LoggingLevel  slog.Level
}

type LoggingConfig struct {
	Mode string `yaml:"mode"`
}

func Load() (*Config, error) {
	//telegram token
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("config loading error:")
	}

	var raw configRaw
	if err := envconfig.Process("", &raw); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	token := strings.TrimSpace(raw.TelegramToken)
	if !isValidTelegramTokenFormat(token) {
		return nil, fmt.Errorf("APP_TELEGRAM_TOKEN invalid format (expected <number>:<string>)")
	}
	//loggin level
	var level slog.Level
	var loggingConfig LoggingConfig
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

	return &Config{
		TelegramToken: token,
		LoggingLevel:  level,
	}, nil
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

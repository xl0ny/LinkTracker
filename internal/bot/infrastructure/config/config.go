package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/logging"
)

type Config struct {
	TelegramToken string `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`
	ScrapperURL   string `yaml:"scrapper_url"`
	BotPort       string `yaml:"bot_port"`
	Logging       struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("config loading: %w", err)
	}

	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	config.TelegramToken = strings.TrimSpace(config.TelegramToken)
	if !isValidTelegramTokenFormat(config.TelegramToken) {
		return nil, errors.New("invalid telegram token format (expected <number>:<string>)")
	}

	data, err := os.ReadFile("cmd/bot/config.yaml")
	if err != nil {
		slog.Error("bot config: read cmd/bot/config.yaml error", slog.String("error", err.Error()))
	} else if err = yaml.Unmarshal(data, &config); err != nil {
		slog.Error("bot config: parse config.yaml error", slog.String("error", err.Error()))
	}

	config.ScrapperURL = strings.TrimSpace(config.ScrapperURL)
	if config.ScrapperURL == "" {
		config.ScrapperURL = "http://localhost:8080"
		slog.Info("bot config: scrapper_url default", slog.String("scrapper_url", config.ScrapperURL))
	}
	config.BotPort = strings.TrimSpace(config.BotPort)
	if config.BotPort == "" {
		config.BotPort = "8081"
		slog.Info("bot config: bot_port default", slog.String("port", config.BotPort))
	}

	return &config, nil
}

const tokenPartsCount = 2

func isValidTelegramTokenFormat(token string) bool {
	parts := strings.SplitN(token, ":", tokenPartsCount)
	if len(parts) != tokenPartsCount {
		return false
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(parts[0]) > 0 && len(parts[1]) > 0
}

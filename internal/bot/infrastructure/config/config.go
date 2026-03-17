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
)

type Config struct {
	TelegramToken string        `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`
	Logging       LoggingConfig `yaml:"logging"`
}

type LoggingConfig struct {
	Mode string `yaml:"mode"`
}

func (c *LoggingConfig) GetLevel() slog.Level {
	if strings.EqualFold(c.Mode, "DEBUG") {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	config.TelegramToken = strings.TrimSpace(config.TelegramToken)
	if !isValidTelegramTokenFormat(config.TelegramToken) {
		return nil, errors.New("APP_TELEGRAM_TOKEN invalid format (expected <number>:<string>)")
	}

	// logginпg level
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	return &config, nil
}

func isValidTelegramTokenFormat(token string) bool {
	const tokenParts = 2
	parts := strings.SplitN(token, ":", tokenParts)
	if len(parts) != tokenParts {
		return false
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(parts[0]) > 0 && len(parts[1]) > 0
}

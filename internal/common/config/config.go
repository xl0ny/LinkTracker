package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type configRaw struct {
	TelegramToken string `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`
}

type Config struct {
	TelegramToken string
}

// LoggingConfig is used for YAML config (e.g. config.yaml).
type LoggingConfig struct {
	Mode string `yaml:"mode"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("config loading error:")
	}

	var raw configRaw
	if err := envconfig.Process("", &raw); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	token := strings.TrimSpace(raw.TelegramToken)
	if !hasValidTelegramTokenFormat(token) {
		return nil, fmt.Errorf("APP_TELEGRAM_TOKEN invalid format (expected <number>:<string>)")
	}

	return &Config{
		TelegramToken: token,
	}, nil
}

func hasValidTelegramTokenFormat(s string) bool {
	parts := strings.SplitN(s, ":", 2)
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

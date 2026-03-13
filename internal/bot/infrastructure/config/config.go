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
	ScrapperURL   string `envconfig:"APP_SCRAPPER_URL"`
	BotPort       string `envconfig:"APP_BOT_PORT"`
	Logging       struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`
}

func (c *Config) GetLevel() slog.Level {
	if strings.EqualFold(c.Logging.Mode, "DEBUG") {
		return slog.LevelDebug
	} else {
		return slog.LevelInfo
	}
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
	config.ScrapperURL = strings.TrimSpace(config.ScrapperURL)
	if config.ScrapperURL == "" {
		config.ScrapperURL = "http://localhost:8080"
		slog.Info("APP_SCRAPPER_URL not set, using default", slog.String("scrapper_url", config.ScrapperURL), slog.String("event", "config_default"))
	}
	config.BotPort = strings.TrimSpace(config.BotPort)
	if config.BotPort == "" {
		config.BotPort = "8081"
		slog.Info("APP_BOT_PORT not set, using default", slog.String("port", config.BotPort), slog.String("event", "config_default"))
	}
	if !isValidTelegramTokenFormat(config.TelegramToken) {
		return nil, fmt.Errorf("APP_TELEGRAM_TOKEN invalid format (expected <number>:<string>)")
	}

	//logginпg level
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		slog.Error("failed to load config, setted level INFO", slog.String("error", err.Error()), slog.String("stage", "config_load"))
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		slog.Error("logging format in confilg.yaml incorrect, setted level INFO", slog.String("error", err.Error()), slog.String("stage", "config_load"))
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

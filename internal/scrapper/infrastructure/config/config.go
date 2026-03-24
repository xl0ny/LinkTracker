package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/logging"
)

type Config struct {
	BotURL string `envconfig:"APP_BOT_URL"`
	Port   string `envconfig:"APP_SCRAPPER_PORT"`
	Logging struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	c.BotURL = strings.TrimSpace(c.BotURL)
	if c.BotURL == "" {
		c.BotURL = "http://localhost:8081"
		slog.Info("scrapper config: APP_BOT_URL default", slog.String("bot_url", c.BotURL))
	}
	if err := validateBotURL(c.BotURL); err != nil {
		return nil, fmt.Errorf("config: APP_BOT_URL: %w", err)
	}
	c.Port = strings.TrimSpace(c.Port)
	if c.Port == "" {
		c.Port = "8080"
		slog.Info("scrapper config: APP_SCRAPPER_PORT default", slog.String("port", c.Port))
	}

	data, err := os.ReadFile("cmd/scrapper/config.yaml")
	if err != nil {
		slog.Error("scrapper config: read cmd/scrapper/config.yaml error", slog.String("error", err.Error()))
		return &c, nil
	}
	if err = yaml.Unmarshal(data, &c); err != nil {
		slog.Error("scrapper config: parse config.yaml error", slog.String("error", err.Error()))
	}
	return &c, nil
}

func validateBotURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("host is required")
	}
	return nil
}

package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	BotURL string `envconfig:"APP_BOT_URL"`
	Port   string `envconfig:"APP_SCRAPPER_PORT"`
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
		slog.Info("APP_BOT_URL not set, using default", slog.String("bot_url", c.BotURL), slog.String("event", "config_default"))
	}
	c.Port = strings.TrimSpace(c.Port)
	if c.Port == "" {
		c.Port = "8080"
		slog.Info("APP_SCRAPPER_PORT not set, using default", slog.String("port", c.Port), slog.String("event", "config_default"))
	}
	return &c, nil
}

package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/logging"
)

type Config struct {
	TelegramToken string `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`

	ScrapperURL string `yaml:"scrapper_url"`
	BotPort     string `yaml:"bot_port"`

	Logging struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`

	Kafka struct {
		config.Kafka `yaml:",inline"`
		Consumer     struct {
			config.KafkaConsumer `yaml:",inline"`
		} `yaml:"consumer"`
	} `yaml:"kafka"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	data, err := os.ReadFile("cmd/bot/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("bot config: read yaml: %w", err)
	}
	var c Config
	if err = yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("bot config: parse yaml: %w", err)
	}
	if err = envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("bot config: env: %w", err)
	}
	return &c, nil
}

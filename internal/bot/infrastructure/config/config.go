package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/logging"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/resilience"
)

type Config struct {
	TelegramToken string `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`

	ScrapperURL  string `yaml:"scrapper_url"`
	BotPort      string `yaml:"bot_port"`
	MetricsPort  string `yaml:"metrics_port"`

	Metrics struct {
		Pushgateway struct {
			Enabled  bool          `yaml:"enabled"`
			URL      string        `yaml:"url"`
			Job      string        `yaml:"job"`
			Interval time.Duration `yaml:"interval"`
		} `yaml:"pushgateway"`
	} `yaml:"metrics"`

	Logging struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`

	Kafka struct {
		config.Kafka `yaml:",inline"`
		Consumer     struct {
			config.KafkaConsumer `yaml:",inline"`
		} `yaml:"consumer"`
	} `yaml:"kafka"`

	Resilience resilience.Config `yaml:"resilience"`

	Redis struct {
		Enabled   bool          `yaml:"enabled"`
		Addr      string        `yaml:"addr"`
		Password  string        `envconfig:"REDIS_PASSWORD"`
		DB        int           `yaml:"db"`
		KeyPrefix string        `yaml:"key_prefix"`
		TTL       time.Duration `yaml:"ttl"`
	} `yaml:"redis"`
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

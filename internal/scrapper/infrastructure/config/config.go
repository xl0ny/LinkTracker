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
	BotURL string `yaml:"bot_url"`
	Port   string `yaml:"port"`

	AccessType string `yaml:"access_type"`

	Postgres struct {
		User     string `yaml:"user"`
		Password string `envconfig:"POSTGRES_PASSWORD" required:"true"`
		DB       string `yaml:"db"`
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		SSLMode  string `yaml:"ssl_mode"`
	} `yaml:"postgres"`

	Batch struct {
		Size int `yaml:"size"`
	} `yaml:"batch"`

	Scheduler struct {
		Interval time.Duration `yaml:"interval"`
		Workers  int           `yaml:"workers"`
	} `yaml:"scheduler"`

	Logging struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`

	Kafka KafkaSettings `yaml:"kafka"`

	Resilience resilience.Config `yaml:"resilience"`

	Valkey struct {
		Enabled     bool          `yaml:"enabled"`
		Addrs       []string      `yaml:"addrs"`
		Password    string        `envconfig:"VALKEY_PASSWORD"`
		KeyPrefix   string        `yaml:"key_prefix"`
		TTL         time.Duration `yaml:"ttl"`
		ClientCache struct {
			Enabled bool          `yaml:"enabled"`
			TTL     time.Duration `yaml:"ttl"`
		} `yaml:"client_cache"`
	} `yaml:"valkey"`
}

type KafkaSettings struct {
	Cluster  config.Kafka `yaml:",inline"`
	Producer struct {
		config.KafkaProducer `yaml:",inline"`
		Outbox               config.KafkaOutbox `yaml:"outbox"`
	} `yaml:"producer"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func (c *Config) PostgresDSN() string {
	return config.BuildPostgresDSN(
		c.Postgres.User,
		c.Postgres.Password,
		c.Postgres.Host,
		c.Postgres.Port,
		c.Postgres.DB,
		c.Postgres.SSLMode,
	)
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	data, err := os.ReadFile("cmd/scrapper/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("scrapper config: read yaml: %w", err)
	}
	var c Config
	if err = yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("scrapper config: parse yaml: %w", err)
	}
	if err = envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("scrapper config: env: %w", err)
	}
	return &c, nil
}

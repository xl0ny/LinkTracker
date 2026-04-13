package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	commondb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/db"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/logging"
)

type Config struct {
	BotURL string `envconfig:"APP_BOT_URL"`
	Port   string `envconfig:"APP_SCRAPPER_PORT"`
	// db
	AccessType       string `envconfig:"APP_SCRAPPER_ACCESS_TYPE"`
	PostgresUser     string `envconfig:"POSTGRES_USER"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDB       string `envconfig:"POSTGRES_DB"`
	PostgresHost     string `envconfig:"POSTGRES_HOST"`
	PostgresPort     string `envconfig:"POSTGRES_PORT"`
	PostgresSSLMode  string `envconfig:"POSTGRES_SSL_MODE"`
	// scrapper (из config.yaml через scrapperFileConfig)
	Batch struct {
		Size int
	}
	Scheduler struct {
		Interval string
		Workers  int
	}
	Logging struct {
		Mode string
	}
}

// scrapperFileConfig — только нечувствительные к репозиторию поля из cmd/scrapper/config.yaml.
type scrapperFileConfig struct {
	AccessType string `yaml:"access_type"`
	Batch      struct {
		Size int `yaml:"size"`
	} `yaml:"batch"`
	Scheduler struct {
		Interval string `yaml:"interval"`
		Workers  int    `yaml:"workers"`
	} `yaml:"scheduler"`
	Logging struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func (c *Config) PostgresDSN() string {
	return commondb.BuildPostgresDSN(
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
		c.PostgresSSLMode,
	)
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		slog.Info("scrapper config: .env not loaded (optional)", slog.String("error", err.Error()))
	}
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
	var fc scrapperFileConfig
	if err = yaml.Unmarshal(data, &fc); err != nil {
		slog.Error("scrapper config: parse config.yaml error", slog.String("error", err.Error()))
	} else {
		if fc.AccessType != "" {
			c.AccessType = fc.AccessType
		}
		c.Batch.Size = fc.Batch.Size
		c.Scheduler.Interval = fc.Scheduler.Interval
		c.Scheduler.Workers = fc.Scheduler.Workers
		c.Logging.Mode = fc.Logging.Mode
	}
	if c.Batch.Size < 50 || c.Batch.Size > 500 {
		slog.Warn("scrapper config: invalid batch.size, using default 100", slog.Int("batch_size", c.Batch.Size))
		c.Batch.Size = 100
	}
	if c.Scheduler.Workers < 1 {
		slog.Warn("scrapper config: invalid scheduler.workers, using default 4", slog.Int("workers", c.Scheduler.Workers))
		c.Scheduler.Workers = 4
	}
	if strings.TrimSpace(c.Scheduler.Interval) == "" {
		c.Scheduler.Interval = "1m"
	}
	if _, parseErr := parseSchedulerInterval(c.Scheduler.Interval); parseErr != nil {
		slog.Warn(
			"scrapper config: invalid scheduler.interval, using default 1m",
			slog.String("scheduler_interval", c.Scheduler.Interval),
			slog.String("error", parseErr.Error()),
		)
		c.Scheduler.Interval = "1m"
	}
	return &c, nil
}

func (c *Config) SchedulerInterval() time.Duration {
	d, err := parseSchedulerInterval(c.Scheduler.Interval)
	if err != nil {
		return time.Minute
	}
	return d
}

func parseSchedulerInterval(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty interval")
	}
	if value, atoiErr := strconv.Atoi(raw); atoiErr == nil {
		if value <= 0 {
			return 0, errors.New("interval must be > 0")
		}
		return time.Duration(value) * time.Second, nil
	}
	d, durErr := time.ParseDuration(raw)
	if durErr != nil {
		return 0, fmt.Errorf("parse duration: %w", durErr)
	}
	if d <= 0 {
		return 0, errors.New("interval must be > 0")
	}
	return d, nil
}

func validateBotURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("scheme must be http or https")
	}
	if u.Host == "" {
		return errors.New("host is required")
	}
	return nil
}

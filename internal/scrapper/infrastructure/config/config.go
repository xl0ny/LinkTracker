package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/logging"
)

type AccessMode string

const (
	AccessSQL AccessMode = "SQL"
	AccessORM AccessMode = "ORM"
)

func (m *AccessMode) Decode(value string) error {
	v := strings.ToUpper(strings.TrimSpace(value))
	if v == "" {
		*m = AccessSQL
		return nil
	}
	switch v {
	case string(AccessSQL):
		*m = AccessSQL
		return nil
	case string(AccessORM):
		*m = AccessORM
		return nil
	default:
		return fmt.Errorf("unknown access_type %q (use SQL or ORM)", value)
	}
}

type PostgresSettings struct {
	DB struct {
		PostgresUser    string `yaml:"postgres_user"`
		PostgresDB      string `yaml:"postgres_db"`
		PostgresHost    string `yaml:"postgres_host"`
		PostgresPort    string `yaml:"postgres_port"`
		PostgresSSLMode string `yaml:"postgres_ssl_mode"`
	} `yaml:"db"`

	PostgresPassword string `envconfig:"POSTGRES_PASSWORD" required:"true"`
}

type SchedulerSettings struct {
	Interval time.Duration `yaml:"interval"`
	Workers  int           `yaml:"workers"`
}

type Config struct {
	PostgresSettings `yaml:",inline"`
	BotURL           string     `yaml:"bot_url"`
	Port             string     `envconfig:"APP_SCRAPPER_PORT"`
	AccessType       AccessMode `yaml:"access_type" envconfig:"APP_SCRAPPER_ACCESS_TYPE"`
	Batch            struct {
		Size int `yaml:"size"`
	} `yaml:"batch"`
	Scheduler SchedulerSettings `yaml:"scheduler"`
	Logging   struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func (c *Config) PostgresDSN() string {
	return BuildPostgresDSN(
		c.DB.PostgresUser,
		c.PostgresPassword,
		c.DB.PostgresHost,
		c.DB.PostgresPort,
		c.DB.PostgresDB,
		c.DB.PostgresSSLMode,
	)
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		slog.Info("scrapper config: .env not loaded (optional)", slog.String("error", err.Error()))
	}
	data, err := os.ReadFile("cmd/scrapper/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("config: read cmd/scrapper/config.yaml: %w", err)
	}
	var c Config
	if err = yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: parse cmd/scrapper/config.yaml: %w", err)
	}
	if err = envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("config: env: %w", err)
	}
	// Переопределение из окружения (Docker Compose: POSTGRES_HOST=db и т.д.)
	if v := strings.TrimSpace(os.Getenv("POSTGRES_HOST")); v != "" {
		c.DB.PostgresHost = v
	}
	if v := strings.TrimSpace(os.Getenv("POSTGRES_PORT")); v != "" {
		c.DB.PostgresPort = v
	}
	if v := strings.TrimSpace(os.Getenv("POSTGRES_USER")); v != "" {
		c.DB.PostgresUser = v
	}
	if v := strings.TrimSpace(os.Getenv("POSTGRES_DB")); v != "" {
		c.DB.PostgresDB = v
	}
	if v := strings.TrimSpace(os.Getenv("POSTGRES_SSL_MODE")); v != "" {
		c.DB.PostgresSSLMode = v
	}
	c.BotURL = strings.TrimSpace(c.BotURL)
	if v := strings.TrimSpace(os.Getenv("APP_BOT_URL")); v != "" {
		c.BotURL = v
	}
	c.BotURL = strings.TrimSpace(c.BotURL)
	if c.BotURL == "" {
		c.BotURL = "http://localhost:8081"
	}
	if err = validateBotURL(c.BotURL); err != nil {
		return nil, fmt.Errorf("config: bot_url: %w", err)
	}
	c.Port = strings.TrimSpace(c.Port)
	if c.Port == "" {
		c.Port = "8080"
	}
	var access AccessMode
	if err = access.Decode(string(c.AccessType)); err != nil {
		return nil, fmt.Errorf("config: access_type: %w", err)
	}
	c.AccessType = access
	return &c, nil
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

func BuildPostgresDSN(user, pass, host, port, dbname, sslmode string) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   dbname,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

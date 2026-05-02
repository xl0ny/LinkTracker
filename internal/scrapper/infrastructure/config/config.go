package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	commondb "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/db"
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

type Config struct {
	DB               commondb.Config `yaml:"db"`
	PostgresPassword string          `yaml:"-" envconfig:"POSTGRES_PASSWORD" required:"true"`
	BotURL           string          `envconfig:"APP_BOT_URL"`
	Port             string          `envconfig:"APP_SCRAPPER_PORT"`
	AccessType       AccessMode      `yaml:"access_type" envconfig:"APP_SCRAPPER_ACCESS_TYPE"`
	Logging          struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func (c *Config) PostgresDSN() string {
	return commondb.BuildPostgresDSN(
		c.DB.User,
		c.PostgresPassword,
		c.DB.Host,
		c.DB.Port,
		c.DB.Database,
		c.DB.SSLMode,
	)
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		slog.Info("scrapper config: .env not loaded (optional)", slog.String("error", err.Error()))
	}
	var c Config
	data, err := os.ReadFile("cmd/scrapper/config.yaml")
	if err != nil {
		slog.Error("scrapper config: read cmd/scrapper/config.yaml error", slog.String("error", err.Error()))
		return nil, fmt.Errorf("config: read cmd/scrapper/config.yaml: %w", err)
	}
	if err = yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: parse cmd/scrapper/config.yaml: %w", err)
	}
	if envErr := envconfig.Process("", &c); envErr != nil {
		return nil, fmt.Errorf("config: %w", envErr)
	}
	c.BotURL = strings.TrimSpace(c.BotURL)
	if c.BotURL == "" {
		c.BotURL = "http://localhost:8081"
		slog.Info("scrapper config: APP_BOT_URL default", slog.String("bot_url", c.BotURL))
	}
	if validateErr := validateBotURL(c.BotURL); validateErr != nil {
		return nil, fmt.Errorf("config: APP_BOT_URL: %w", validateErr)
	}
	c.Port = strings.TrimSpace(c.Port)
	if c.Port == "" {
		c.Port = "8080"
		slog.Info("scrapper config: APP_SCRAPPER_PORT default", slog.String("port", c.Port))
	}
	rawAccess := string(c.AccessType)
	var access AccessMode
	if err := access.Decode(rawAccess); err != nil {
		return nil, fmt.Errorf("config: access_type: %w", err)
	}
	c.AccessType = access
	if rawAccess == "" {
		slog.Info("scrapper config: access_type default", slog.String("access_type", string(c.AccessType)))
	}
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

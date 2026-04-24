package db

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

func GetConfig() (*Config, error) {
	data, err := os.ReadFile("cmd/migrator/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("migrator: read config: %w", err)
	}
	var config Config
	if yamlErr := yaml.Unmarshal(data, &config); yamlErr != nil {
		return nil, fmt.Errorf("migrator: parse yaml: %w", yamlErr)
	}

	if dotenvErr := godotenv.Load(); dotenvErr != nil {
		log.Println("migrator: env variables loading failed:", dotenvErr)
	}

	if envErr := envconfig.Process("", &config); envErr != nil {
		return nil, fmt.Errorf("migrator: env config: %w", envErr)
	}
	return &config, nil
}

type Config struct {
	MigrationsConfig
	DB struct {
		PostgresUser    string `yaml:"postgres_user"`
		PostgresDB      string `yaml:"postgres_db"`
		PostgresHost    string `yaml:"postgres_host"`
		PostgresPort    string `yaml:"postgres_port"`
		PostgresSSLMode string `yaml:"postgres_ssl_mode"`
	} `yaml:"db"`

	PostgresPassword string `envconfig:"POSTGRES_PASSWORD" required:"true"`
}

type MigrationsConfig struct {
	Migrations struct {
		Dir       string `yaml:"dir"`
		Direction string `yaml:"direction"`
		Steps     int    `yaml:"steps"`
	} `yaml:"migrations"`
}

func BuildPostgresDSN(user, pass, host, port, db, sslmode string) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   db,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

package config

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

func GetConfig() (*DB, error) {
	data, err := os.ReadFile("migrations/migrator/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("migrator: read config: %w", err)
	}
	var config DB
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

type DB struct {
	Migrations struct {
		Dir       string `yaml:"dir"`
		Direction string `yaml:"direction"`
		Steps     int    `yaml:"steps"`
	} `yaml:"migrations"`
	PostgresUser     string `yaml:"postgres_user" envconfig:"POSTGRES_USER" default:"postgres"`
	PostgresPassword string `envconfig:"POSTGRES_PASSWORD"`
	PostgresDB       string `yaml:"postgres_db" envconfig:"POSTGRES_DB" default:"linktracker"`
	PostgresHost     string `yaml:"postgres_host" envconfig:"POSTGRES_HOST" default:"localhost"`
	PostgresPort     string `yaml:"postgres_port" envconfig:"POSTGRES_PORT" default:"5432"`
	PostgresSSLMode  string `yaml:"postgres_ssl_mode" envconfig:"POSTGRES_SSL_MODE" default:"disable"`
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

package db

import (
	"fmt"
	"net/url"
)

type Config struct {
	DB struct {
		PostgresUser    string `yaml:"postgres_user"`
		PostgresDB      string `yaml:"postgres_db"`
		PostgresHost    string `yaml:"postgres_host"`
		PostgresPort    string `yaml:"postgres_port"`
		PostgresSSLMode string `yaml:"postgres_ssl_mode"`
	} `yaml:"db"`

	PostgresPassword string `envconfig:"POSTGRES_PASSWORD" required:"true"`
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

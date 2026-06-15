package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPostgresDSN(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		pass     string
		host     string
		port     string
		db       string
		sslmode  string
		expected string
	}{
		{
			name:     "builds escaped postgres dsn",
			user:     "app",
			pass:     "p@ss word",
			host:     "localhost",
			port:     "5432",
			db:       "linktracker",
			sslmode:  "disable",
			expected: "postgres://app:p%40ss%20word@localhost:5432/linktracker?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPostgresDSN(tt.user, tt.pass, tt.host, tt.port, tt.db, tt.sslmode)

			assert.Equal(t, tt.expected, got)
		})
	}
}

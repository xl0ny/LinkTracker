package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"

	scrapcfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/config"
)

type Config struct {
	scrapcfg.PostgresSettings `yaml:",inline"`
	Migrations      struct {
		Dir           string `yaml:"dir"`
		TargetVersion *int   `yaml:"target_version,omitempty"`
	} `yaml:"migrations"`
}

func loadConfig() (*Config, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("migrator: runtime.Caller failed")
	}
	cfgPath := filepath.Join(filepath.Dir(thisFile), "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("migrator: read config %s: %w", cfgPath, err)
	}
	var c Config
	if yamlErr := yaml.Unmarshal(data, &c); yamlErr != nil {
		return nil, fmt.Errorf("migrator: parse yaml: %w", yamlErr)
	}

	if dotenvErr := godotenv.Load(); dotenvErr != nil {
		log.Println("migrator: env variables loading failed:", dotenvErr)
	}

	if envErr := envconfig.Process("", &c); envErr != nil {
		return nil, fmt.Errorf("migrator: env config: %w", envErr)
	}
	return &c, nil
}

package config

import (
	"fmt"
	"os"
	"time"

	"github.com/goccy/go-yaml"
)

const DefaultPath = "loadtest/config.yaml"

type Config struct {
	Run  Run  `yaml:"run"`
	Seed Seed `yaml:"seed"`
}

type Run struct {
	Target            string        `yaml:"target"`
	VUs               int           `yaml:"vus"`
	RampUp            time.Duration `yaml:"ramp_up"`
	Duration          time.Duration `yaml:"duration"`
	Ratio             int           `yaml:"ratio"`
	Chats             int           `yaml:"chats"`
	Scenario          string        `yaml:"scenario"`
	Output            string        `yaml:"output"`
	ProgressInterval  time.Duration `yaml:"progress_interval"`
	HTTPClientTimeout time.Duration `yaml:"http_client_timeout"`
	ChatIDBase        int64         `yaml:"chat_id_base"`
}

type Seed struct {
	DSN          string        `yaml:"dsn"`
	Chats        int           `yaml:"chats"`
	LinksPerChat int           `yaml:"links_per_chat"`
	Batch        int           `yaml:"batch"`
	Timeout      time.Duration `yaml:"timeout"`
	ChatIDBase   int64         `yaml:"chat_id_base"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loadtest config: read %s: %w", path, err)
	}
	var c Config
	if err = yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("loadtest config: parse yaml: %w", err)
	}
	return &c, nil
}

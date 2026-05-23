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
)

type Filtering struct {
	StopWords       []string `yaml:"stop-words"`
	ExcludedAuthors []string `yaml:"excluded-authors"`
	MinLength       int      `yaml:"min-length"`
}

type HuggingFace struct {
	APIURL    string        `yaml:"api_url"`
	Token     string        `yaml:"token" envconfig:"HUGGINGFACE_TOKEN"`
	MaxLength int           `yaml:"max_length"`
	MinLength int           `yaml:"min_length"`
	Timeout   time.Duration `yaml:"timeout"`
}

type Summarization struct {
	Mode        string      `yaml:"mode" validate:"oneof=stub huggingface"`
	Threshold   int         `yaml:"threshold"`
	HuggingFace HuggingFace `yaml:"huggingface"`
}

type AIAgent struct {
	Filtering     Filtering     `yaml:"filtering"`
	Summarization Summarization `yaml:"summarization"`
}

type Config struct {
	Port string `yaml:"port"`

	Logging struct {
		Mode string `yaml:"mode"`
	} `yaml:"logging"`

	Kafka struct {
		config.Kafka `yaml:",inline"`
		Consumer     struct {
			config.KafkaConsumer `yaml:",inline"`
		} `yaml:"consumer"`
		Producer struct {
			config.KafkaProducer `yaml:",inline"`
		} `yaml:"producer"`
	} `yaml:"kafka"`

	AIAgent AIAgent `yaml:"ai-agent"`
}

func (c *Config) GetLevel() slog.Level {
	return logging.LevelFromMode(c.Logging.Mode)
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	data, err := os.ReadFile("cmd/agent/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("agent config: read yaml: %w", err)
	}
	var c Config
	if err = yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("agent config: parse yaml: %w", err)
	}
	if err = envconfig.Process("", &c); err != nil {
		return nil, fmt.Errorf("agent config: env: %w", err)
	}
	return &c, nil
}

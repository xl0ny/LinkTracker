package bot

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type TelegramToken string

func (t TelegramToken) String() string { return string(t) }

type configRaw struct {
	TelegramToken string `envconfig:"APP_TELEGRAM_TOKEN" required:"true"`
}

type Config struct {
	TelegramToken TelegramToken
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	var raw configRaw
	if err := envconfig.Process("", &raw); err != nil {
		return nil, fmt.Errorf("конфиг: %w", err)
	}

	token := strings.TrimSpace(raw.TelegramToken)
	if !isValidTelegramTokenFormat(token) {
		return nil, fmt.Errorf("APP_TELEGRAM_TOKEN неверный формат (ожидается <число>:<строка>)")
	}

	return &Config{
		TelegramToken: TelegramToken(token),
	}, nil
}

func isValidTelegramTokenFormat(s string) bool {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return false
	}
	for _, r := range parts[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(parts[0]) > 0 && len(parts[1]) > 0
}

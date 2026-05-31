package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	botcfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
)

// idempotencyClaimed просто метка того, что ключ eventID уже обработан.
const idempotencyClaimed = true

type Idempotency struct {
	client    *goredis.Client
	keyPrefix string
	ttl       time.Duration
}

func New(cfg botcfg.RedisSettings) *Idempotency {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &Idempotency{client: client, keyPrefix: cfg.KeyPrefix, ttl: cfg.TTL}
}

func (i *Idempotency) Ping(ctx context.Context) error {
	if err := i.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("bot-redis: ping: %w", err)
	}
	return nil
}

func (i *Idempotency) Close() error {
	if err := i.client.Close(); err != nil {
		return fmt.Errorf("bot-redis: close: %w", err)
	}
	return nil
}

// Acquire помечает eventID как обработанный в Redis.
func (i *Idempotency) Acquire(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return true, nil
	}
	ok, err := i.client.SetNX(ctx, i.keyPrefix+eventID, idempotencyClaimed, i.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("bot-redis: setnx: %w", err)
	}
	return ok, nil
}

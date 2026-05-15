package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Config struct {
	Addrs       []string
	Password    string
	KeyPrefix   string
	TTL         time.Duration
	ClientCache bool
	CSCTTL      time.Duration
}

type LinksCache struct {
	client    valkey.Client
	keyPrefix string
	ttl       time.Duration
	useCSC    bool
	cscTTL    time.Duration
}

func New(ctx context.Context, cfg Config) (*LinksCache, error) {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: cfg.Addrs,
		Password:    cfg.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("valkey: New: %w", err)
	}

	c := &LinksCache{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
		ttl:       cfg.TTL,
		useCSC:    cfg.ClientCache,
		cscTTL:    cfg.CSCTTL,
	}

	if err = c.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}
	return c, nil
}

func (c *LinksCache) Ping(ctx context.Context) error {
	if err := c.client.Do(ctx, c.client.B().Ping().Build()).Error(); err != nil {
		return fmt.Errorf("valkey: ping: %w", err)
	}
	return nil
}

func (c *LinksCache) Close() {
	c.client.Close()
}

func (c *LinksCache) Get(ctx context.Context, chatID int64) ([]domain.Link, bool, error) {
	key := c.key(chatID)

	var raw string
	var err error
	if c.useCSC {
		raw, err = c.client.DoCache(ctx, c.client.B().Get().Key(key).Cache(), c.cscTTL).ToString()
	} else {
		raw, err = c.client.Do(ctx, c.client.B().Get().Key(key).Build()).ToString()
	}
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("valkey: get: %w", err)
	}

	var links []domain.Link
	if err = json.Unmarshal([]byte(raw), &links); err != nil {
		return nil, false, fmt.Errorf("valkey: get: unmarshal: %w", err)
	}
	return links, true, nil
}

func (c *LinksCache) Set(ctx context.Context, chatID int64, links []domain.Link) error {
	payload, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("valkey: set: marshal: %w", err)
	}

	cmd := c.client.B().Set().Key(c.key(chatID)).Value(string(payload)).Ex(c.ttl).Build()
	if err = c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("valkey: set: %w", err)
	}
	return nil
}

func (c *LinksCache) Invalidate(ctx context.Context, chatID int64) error {
	cmd := c.client.B().Del().Key(c.key(chatID)).Build()
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("valkey: invalidate: %w", err)
	}
	return nil
}

func (c *LinksCache) key(chatID int64) string {
	return c.keyPrefix + strconv.FormatInt(chatID, 10)
}

//go:build integration

package valkey_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	cacheimpl "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/valkey"
)

const (
	valkeyImage = "valkey/valkey:8"
	valkeyPort  = "6379/tcp"
)

func startValkey(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        valkeyImage,
			ExposedPorts: []string{valkeyPort},
			WaitingFor:   wait.ForListeningPort(valkeyPort).WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	}
	container, err := tc.GenericContainer(ctx, req)
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = tc.TerminateContainer(container)
	})

	host, err := container.Host(ctx)
	assert.NoError(t, err)
	port, err := container.MappedPort(ctx, valkeyPort)
	assert.NoError(t, err)

	return host + ":" + port.Port()
}

func newCache(t *testing.T, addr string, csc bool) *cacheimpl.LinksCache {
	t.Helper()
	cache, err := cacheimpl.New(context.Background(), cacheimpl.Config{
		Addrs:       []string{addr},
		KeyPrefix:   "test:links:",
		TTL:         2 * time.Second,
		ClientCache: csc,
		CSCTTL:      time.Second,
	})
	assert.NoError(t, err)
	t.Cleanup(cache.Close)
	return cache
}

func TestValkeyCache_SetGetInvalidate(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "valkey cache set get invalidate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			addr := startValkey(t)
			cache := newCache(t, addr, false)
			ctx := context.Background()

			links := []domain.Link{
				{URL: "https://example.com", Tags: []string{"go"}, Filters: []string{"prio"}, LastUpdated: time.Unix(1700000000, 0).UTC()},
				{URL: "https://example.org"},
			}

			got, ok, err := cache.Get(ctx, 1)
			assert.NoError(t, err)
			assert.False(t, ok)
			assert.Nil(t, got)

			assert.NoError(t, cache.Set(ctx, 1, links))

			got, ok, err = cache.Get(ctx, 1)
			assert.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, links, got)

			assert.NoError(t, cache.Invalidate(ctx, 1))

			got, ok, err = cache.Get(ctx, 1)
			assert.NoError(t, err)
			assert.False(t, ok)
			assert.Nil(t, got)
		})
	}
}

func TestValkeyCache_TTLExpires(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "valkey cache ttlexpires"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			addr := startValkey(t)
			cache := newCache(t, addr, false)
			ctx := context.Background()

			links := []domain.Link{{URL: "https://ttl"}}
			assert.NoError(t, cache.Set(ctx, 42, links))

			_, ok, err := cache.Get(ctx, 42)
			assert.NoError(t, err)
			assert.True(t, ok)

			assert.Eventually(t, func() bool {
				_, found, gErr := cache.Get(ctx, 42)
				return gErr == nil && !found
			}, 6*time.Second, 200*time.Millisecond)
		})
	}
}

func TestValkeyCache_ClientSideCaching(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "valkey cache client side caching"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			addr := startValkey(t)
			cache := newCache(t, addr, true)
			ctx := context.Background()

			links := []domain.Link{{URL: "https://csc"}}
			assert.NoError(t, cache.Set(ctx, 100, links))

			for range 5 {
				got, ok, err := cache.Get(ctx, 100)
				assert.NoError(t, err)
				assert.True(t, ok)
				assert.Equal(t, links, got)
			}

			assert.NoError(t, cache.Invalidate(ctx, 100))

			assert.Eventually(t, func() bool {
				_, ok, err := cache.Get(ctx, 100)
				return err == nil && !ok
			}, 3*time.Second, 100*time.Millisecond)
		})
	}
}

func TestValkeyCache_Ping(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "valkey cache ping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			addr := startValkey(t)
			cache := newCache(t, addr, false)
			assert.NoError(t, cache.Ping(context.Background()))
		})
	}
}

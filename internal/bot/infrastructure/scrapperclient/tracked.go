package scrapperclient

import (
	"context"
	"fmt"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/metrics"
)

type tracked struct {
	inner application.LinkTracker
	m     *metrics.Bot
}

func Track(inner application.LinkTracker, m *metrics.Bot) application.LinkTracker {
	if m == nil {
		return inner
	}
	return &tracked{inner: inner, m: m}
}

func (t *tracked) RegisterChat(ctx context.Context, chatID int64) error {
	if err := metrics.Timed(t.m.CommandDuration, metrics.ScopeScrapperSync, "PostTgChatId", func() error {
		return t.inner.RegisterChat(ctx, chatID)
	}); err != nil {
		return fmt.Errorf("register chat: %w", err)
	}
	return nil
}

func (t *tracked) AddLink(ctx context.Context, chatID int64, link string, tags []string) error {
	if err := metrics.Timed(t.m.CommandDuration, metrics.ScopeScrapperSync, "PostLinks", func() error {
		return t.inner.AddLink(ctx, chatID, link, tags)
	}); err != nil {
		return fmt.Errorf("add link: %w", err)
	}
	return nil
}

func (t *tracked) RemoveLink(ctx context.Context, chatID int64, link string) error {
	if err := metrics.Timed(t.m.CommandDuration, metrics.ScopeScrapperSync, "DeleteLinks", func() error {
		return t.inner.RemoveLink(ctx, chatID, link)
	}); err != nil {
		return fmt.Errorf("remove link: %w", err)
	}
	return nil
}

func (t *tracked) ListLinks(ctx context.Context, chatID int64, tagFilter string) ([]application.LinkInfo, error) {
	start := time.Now()
	out, err := t.inner.ListLinks(ctx, chatID, tagFilter)
	metrics.ObserveDuration(t.m.CommandDuration, metrics.ScopeScrapperSync, "GetLinks", start)
	if err != nil {
		return nil, fmt.Errorf("list links: %w", err)
	}
	return out, nil
}

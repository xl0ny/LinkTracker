package scrapperclient

import (
	"context"
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
	return metrics.Timed(t.m.CommandDuration, metrics.ScopeScrapperSync, "PostTgChatId", func() error {
		return t.inner.RegisterChat(ctx, chatID)
	})
}

func (t *tracked) AddLink(ctx context.Context, chatID int64, link string, tags []string) error {
	return metrics.Timed(t.m.CommandDuration, metrics.ScopeScrapperSync, "PostLinks", func() error {
		return t.inner.AddLink(ctx, chatID, link, tags)
	})
}

func (t *tracked) RemoveLink(ctx context.Context, chatID int64, link string) error {
	return metrics.Timed(t.m.CommandDuration, metrics.ScopeScrapperSync, "DeleteLinks", func() error {
		return t.inner.RemoveLink(ctx, chatID, link)
	})
}

func (t *tracked) ListLinks(ctx context.Context, chatID int64, tagFilter string) ([]application.LinkInfo, error) {
	start := time.Now()
	out, err := t.inner.ListLinks(ctx, chatID, tagFilter)
	metrics.ObserveDuration(t.m.CommandDuration, metrics.ScopeScrapperSync, "GetLinks", start)
	return out, err
}

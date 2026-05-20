package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type Fallback struct {
	primary  application.BotNotifier
	fallback application.BotNotifier
}

func NewFallback(primary, fallback application.BotNotifier) *Fallback {
	return &Fallback{primary: primary, fallback: fallback}
}

func (f *Fallback) Notify(ctx context.Context, chatID int64, link domain.Link, description string) error {
	err := f.primary.Notify(ctx, chatID, link, description)
	if err == nil {
		return nil
	}
	slog.Warn("notifier: http failed, kafka fallback",
		slog.Int64("chat_id", chatID),
		slog.String("url", link.URL),
		slog.String("error", err.Error()))
	fbErr := f.fallback.Notify(ctx, chatID, link, description)
	if fbErr != nil {
		return fmt.Errorf("notify: http: %w; kafka: %w", err, fbErr)
	}
	return nil
}

func (f *Fallback) NotifyFailedLinks(ctx context.Context, chatID int64, links []string) error {
	err := f.primary.NotifyFailedLinks(ctx, chatID, links)
	if err == nil {
		return nil
	}
	slog.Warn("notifier: http failed links, kafka fallback",
		slog.Int64("chat_id", chatID),
		slog.String("error", err.Error()))
	fbErr := f.fallback.NotifyFailedLinks(ctx, chatID, links)
	if fbErr != nil {
		return errors.Join(err, fbErr)
	}
	return nil
}

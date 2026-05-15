package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type LinksCache interface {
	Get(ctx context.Context, chatID int64) ([]domain.Link, bool, error)
	Set(ctx context.Context, chatID int64, links []domain.Link) error
	Invalidate(ctx context.Context, chatID int64) error
}

type ChatUseCase interface {
	ChatRegistration(ctx context.Context, id int64) error
	ChatDelition(ctx context.Context, id int64) error
	LinkAddment(ctx context.Context, chatID int64, link string, tags, filters *[]string) error
	GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error)
	DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error)
}

type CachedChatUC struct {
	inner ChatUseCase
	cache LinksCache
}

func NewCachedChatUC(inner ChatUseCase, cache LinksCache) *CachedChatUC {
	return &CachedChatUC{inner: inner, cache: cache}
}

func (uc *CachedChatUC) ChatRegistration(ctx context.Context, id int64) error {
	if err := uc.inner.ChatRegistration(ctx, id); err != nil {
		return fmt.Errorf("cached chat: chat registration: %w", err)
	}
	return nil
}

func (uc *CachedChatUC) ChatDelition(ctx context.Context, id int64) error {
	if err := uc.inner.ChatDelition(ctx, id); err != nil {
		return fmt.Errorf("cached chat: chat delition: %w", err)
	}
	uc.invalidate(ctx, id, "ChatDelition")
	return nil
}

func (uc *CachedChatUC) LinkAddment(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	if err := uc.inner.LinkAddment(ctx, chatID, link, tags, filters); err != nil {
		return fmt.Errorf("cached chat: link addment: %w", err)
	}
	uc.invalidate(ctx, chatID, "LinkAddment")
	return nil
}

func (uc *CachedChatUC) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	if links, ok, err := uc.cache.Get(ctx, chatID); err != nil {
		slog.Warn("application: GetLinks: cache get failed, falling back to source",
			slog.Int64("chat_id", chatID),
			slog.String("error", err.Error()))
	} else if ok {
		slog.Debug("application: GetLinks: cache hit", slog.Int64("chat_id", chatID))
		return links, nil
	}

	links, err := uc.inner.GetLinks(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("cached chat: get links: %w", err)
	}

	if setErr := uc.cache.Set(ctx, chatID, links); setErr != nil {
		slog.Warn("application: GetLinks: cache set failed",
			slog.Int64("chat_id", chatID),
			slog.String("error", setErr.Error()))
	}
	return links, nil
}

func (uc *CachedChatUC) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	link, err := uc.inner.DeleteLink(ctx, chatID, linkURL)
	if err != nil {
		return domain.Link{}, fmt.Errorf("cached chat: delete link: %w", err)
	}
	uc.invalidate(ctx, chatID, "DeleteLink")
	return link, nil
}

func (uc *CachedChatUC) invalidate(ctx context.Context, chatID int64, op string) {
	if err := uc.cache.Invalidate(ctx, chatID); err != nil && !errors.Is(err, context.Canceled) {
		slog.Warn("application: "+op+": cache invalidate failed",
			slog.Int64("chat_id", chatID),
			slog.String("error", err.Error()))
	}
}

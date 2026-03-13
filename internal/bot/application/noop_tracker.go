package application

import (
	"context"
	"fmt"
)

type noopTracker struct{}

func (noopTracker) RegisterChat(ctx context.Context, chatID int64) error {
	return nil
}

func (noopTracker) AddLink(ctx context.Context, chatID int64, link string, tags []string) error {
	return fmt.Errorf("scrapper not configured")
}

func (noopTracker) RemoveLink(ctx context.Context, chatID int64, link string) error {
	return fmt.Errorf("scrapper not configured")
}

func (noopTracker) ListLinks(ctx context.Context, chatID int64, tagFilter string) ([]LinkInfo, error) {
	return nil, fmt.Errorf("scrapper not configured")
}

func NewNoopTracker() LinkTracker {
	return noopTracker{}
}

package application

import "context"

type LinkTracker interface {
	RegisterChat(ctx context.Context, chatID int64) error
	AddLink(ctx context.Context, chatID int64, link string, tags []string) error
	RemoveLink(ctx context.Context, chatID int64, link string) error
	ListLinks(ctx context.Context, chatID int64, tagFilter string) ([]LinkInfo, error)
}

type LinkInfo struct {
	URL  string
	Tags []string
}

package application

import "context"

// Tracker — клиент Scrapper API для регистрации чата и ссылок (реализация в scrapperclient).
type Tracker interface {
	RegisterChat(ctx context.Context, chatID int64) error
	AddLink(ctx context.Context, chatID int64, link string, tags []string) error
	RemoveLink(ctx context.Context, chatID int64, link string) error
	ListLinks(ctx context.Context, chatID int64, tagFilter string) ([]LinkInfo, error)
}

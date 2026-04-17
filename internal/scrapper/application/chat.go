package application

import (
	"context"
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ChatRepository interface {
	AddChat(ctx context.Context, id int64) error
	DeleteChat(ctx context.Context, id int64) error
	GetChats(ctx context.Context, limit, offset int) (map[int64]domain.Chat, error)
}

type LinkRepository interface {
	AddLink(ctx context.Context, chatID int64, link string, tags, filters *[]string) error
	GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error)
	DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error)
}

type ChatUC struct {
	chats ChatRepository
	links LinkRepository
}

func NewChatUC(chats ChatRepository, links LinkRepository) *ChatUC {
	return &ChatUC{
		chats: chats,
		links: links,
	}
}

func (uc *ChatUC) ChatRegistration(ctx context.Context, id int64) error {
	if err := uc.chats.AddChat(ctx, id); err != nil {
		return fmt.Errorf("add chat: %w", err)
	}
	return nil
}

func (uc *ChatUC) ChatDelition(ctx context.Context, id int64) error {
	if err := uc.chats.DeleteChat(ctx, id); err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	return nil
}

func (uc *ChatUC) LinkAddment(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	if err := uc.links.AddLink(ctx, chatID, link, tags, filters); err != nil {
		return fmt.Errorf("add link: %w", err)
	}
	return nil
}

func (uc *ChatUC) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	links, err := uc.links.GetLinks(ctx, chatID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}
	return links, nil
}

func (uc *ChatUC) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	link, err := uc.links.DeleteLink(ctx, chatID, linkURL)
	if err != nil {
		return domain.Link{}, fmt.Errorf("delete link: %w", err)
	}
	return link, nil
}

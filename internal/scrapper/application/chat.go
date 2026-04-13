package application

import (
	"context"
	"fmt"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ChatRepository interface {
	AddChat(ctx context.Context, id int64) error
	DeleteChat(ctx context.Context, id int64) error
	AddLink(ctx context.Context, chatID int64, link string, tags, filters *[]string) error
	GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error)
	GetChats(ctx context.Context, limit, offset int) (map[int64]domain.Chat, error)
	ListSubscribedLinks(ctx context.Context, limit, offset int) ([]domain.SubscribedLink, error)
	DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error)
	UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error
	CreateTag(ctx context.Context, value string) (int64, error)
	ListTags(ctx context.Context, limit, offset int) ([]domain.Tag, error)
	UpdateTag(ctx context.Context, id int64, value string) error
	DeleteTag(ctx context.Context, id int64) error
	Close()
}

type chatUC struct {
	repo ChatRepository
}

//revive:disable-next-line:unexported-return returning *chatUC is intentional (internal impl)
func NewChatUC(repo ChatRepository) *chatUC {
	return &chatUC{
		repo: repo,
	}
}

func (uc *chatUC) ChatRegistration(ctx context.Context, id int64) error {
	if err := uc.repo.AddChat(ctx, id); err != nil {
		return fmt.Errorf("add chat: %w", err)
	}
	return nil
}

func (uc *chatUC) ChatDelition(ctx context.Context, id int64) error {
	if err := uc.repo.DeleteChat(ctx, id); err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}
	return nil
}

func (uc *chatUC) LinkAddment(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	if err := uc.repo.AddLink(ctx, chatID, link, tags, filters); err != nil {
		return fmt.Errorf("add link: %w", err)
	}
	return nil
}

func (uc *chatUC) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	links, err := uc.repo.GetLinks(ctx, chatID, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}
	return links, nil
}

func (uc *chatUC) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	link, err := uc.repo.DeleteLink(ctx, chatID, linkURL)
	if err != nil {
		return domain.Link{}, fmt.Errorf("delete link: %w", err)
	}
	return link, nil
}

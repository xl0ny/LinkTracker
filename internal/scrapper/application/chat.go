package application

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ChatRepository interface {
	AddChat(ctx context.Context, id int64) error
	DeleteChat(ctx context.Context, id int64) error
	AddLink(ctx context.Context, chatid int64, link string, tags, filters *[]string) error
	GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error)
	GetChats(ctx context.Context) (map[int64]domain.Chat, error)
	DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error)
}

type chatUC struct {
	repo ChatRepository
}

func NewChatUC(repo ChatRepository) *chatUC {
	return &chatUC{
		repo: repo,
	}
}

func (uc *chatUC) ChatRegistration(ctx context.Context, id int64) error {
	return uc.repo.AddChat(ctx, id)
}

func (uc *chatUC) ChatDelition(ctx context.Context, id int64) error {
	return uc.repo.DeleteChat(ctx, id)
}

func (uc *chatUC) LinkAddment(ctx context.Context, chatId int64, link string, tags, filters *[]string) error {
	return uc.repo.AddLink(ctx, chatId, link, tags, filters)
}

func (uc *chatUC) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	return uc.repo.GetLinks(ctx, chatID)
}

func (uc *chatUC) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	return uc.repo.DeleteLink(ctx, chatID, linkURL)
}

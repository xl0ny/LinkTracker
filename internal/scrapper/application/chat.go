package application

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

// ChatRepository — интерфейс репозитория чатов и ссылок. Все типы из domain.
type ChatRepository interface {
	AddChat(ctx context.Context, id int64) error
	DeleteChat(ctx context.Context, id int64) error
	AddLink(ctx context.Context, chatid int64, link string, tags, filters *[]string) error
	GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error)
	GetAllLinks(ctx context.Context) ([]domain.Link, error)
	DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error)
}

type chatUC struct {
	repo ChatRepository
}

func (uc *chatUC) chatRegistration(ctx context.Context, id int64) error {
	return uc.repo.AddChat(ctx, id)
}

func (uc *chatUC) chatDelition(ctx context.Context, id int64) error {
	return uc.repo.DeleteChat(ctx, id)
}

func (uc *chatUC) linkAddment(ctx context.Context, chatId int64, link string, tags, filters *[]string) error {
	return uc.repo.AddLink(ctx, chatId, link, tags, filters)
}

func (uc *chatUC) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	return uc.repo.GetLinks(ctx, chatID)
}

func (uc *chatUC) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	return uc.repo.DeleteLink(ctx, chatID, linkURL)
}

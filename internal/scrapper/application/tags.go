package application

import (
	"context"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type TagRepository interface {
	CreateTag(ctx context.Context, value string) (int64, error)
	ListTags(ctx context.Context, limit, offset int) ([]domain.Tag, error)
	UpdateTag(ctx context.Context, id int64, value string) error
	DeleteTag(ctx context.Context, id int64) error
}

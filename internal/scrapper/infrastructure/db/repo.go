package db

import (
	"context"
	"fmt"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/model"
)

type repository struct {
	Chats map[int64]model.Chat
}

func NewRepository() *repository {
	return &repository{Chats: make(map[int64]model.Chat)}
}

func (r *repository) AddChat(ctx context.Context, id int64) error {
	if !r.exists(id) {
		r.Chats[id] = model.Chat{Id: &id}
		return nil
	}
	return fmt.Errorf("chat already exists")

}

func (r *repository) DeleteChat(ctx context.Context, id int64) error {
	if r.exists(id) {
		delete(r.Chats, id)
		return nil
	}
	return fmt.Errorf("chat not found")
}

func (r *repository) AddLink(ctx context.Context, chatid int64, link string, tags, filters *[]string) error {
	chat, ok := r.Chats[chatid]
	if !ok {
		return fmt.Errorf("chat not found")
	}
	for _, l := range chat.Links {
		if l.URL == link {
			return fmt.Errorf("link already exists")
		}
	}
	tagsVal := []string{}
	if tags != nil {
		tagsVal = *tags
	}
	filtersVal := []string{}
	if filters != nil {
		filtersVal = *filters
	}
	chat.Links = append(chat.Links, model.Link{
		URL:     link,
		Tags:    tagsVal,
		Filters: filtersVal,
	})
	r.Chats[chatid] = chat
	return nil
}

func (r *repository) GetLinks(ctx context.Context, chatID int64) ([]domain.Link, error) {
	chat, ok := r.Chats[chatID]
	if !ok {
		return nil, fmt.Errorf("chat not found")
	}
	return toDomainLinks(chat.Links), nil
}

func (r *repository) GetChats(ctx context.Context) (map[int64]domain.Chat, error) {
	chats := make(map[int64]domain.Chat)
	for id, chat := range r.Chats {
		chats[id] = domain.Chat{
			Id:    chat.Id,
			Links: toDomainLinks(chat.Links),
		}
	}
	return chats, nil
}

func (r *repository) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	chat, ok := r.Chats[chatID]
	if !ok {
		return domain.Link{}, fmt.Errorf("chat not found")
	}
	for i, l := range chat.Links {
		if l.URL == linkURL {
			removed := chat.Links[i]
			chat.Links = append(chat.Links[:i], chat.Links[i+1:]...)
			r.Chats[chatID] = chat
			return toDomainLink(removed), nil
		}
	}
	return domain.Link{}, fmt.Errorf("link not found")
}

func (r *repository) exists(id int64) bool {
	_, ok := r.Chats[id]
	return ok
}

func (r *repository) UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error {
	chat, ok := r.Chats[chatID]
	if !ok {
		return fmt.Errorf("chat not found")
	}
	for i := range chat.Links {
		if chat.Links[i].URL == linkURL {
			chat.Links[i].LastUpdated = t
			r.Chats[chatID] = chat
			return nil
		}
	}
	return fmt.Errorf("link not found")
}

func toDomainLink(l model.Link) domain.Link {
	return domain.Link{URL: l.URL, Tags: l.Tags, Filters: l.Filters, LastUpdated: l.LastUpdated}
}

func toDomainLinks(links []model.Link) []domain.Link {
	out := make([]domain.Link, len(links))
	for i := range links {
		out[i] = toDomainLink(links[i])
	}
	return out
}

package inmemory

import (
	"context"
	"errors"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/db/inmemory/model"
)

type repository struct {
	Chats map[int64]model.Chat
}

//revive:disable-next-line:unexported-return returning *repository is intentional (internal impl)
func NewRepository() *repository {
	return &repository{Chats: make(map[int64]model.Chat)}
}

func (r *repository) AddChat(_ context.Context, id int64) error {
	if !r.exists(id) {
		r.Chats[id] = model.Chat{ID: &id}
		return nil
	}
	return errors.New("chat already exists")

}

func (r *repository) DeleteChat(_ context.Context, id int64) error {
	if r.exists(id) {
		delete(r.Chats, id)
		return nil
	}
	return errors.New("chat not found")
}

func (r *repository) AddLink(_ context.Context, chatID int64, link string, tags, filters *[]string) error {
	chat, ok := r.Chats[chatID]
	if !ok {
		return errors.New("chat not found")
	}
	for _, l := range chat.Links {
		if l.URL == link {
			return errors.New("link already exists")
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
	r.Chats[chatID] = chat
	return nil
}

func (r *repository) GetLinks(_ context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	chat, ok := r.Chats[chatID]
	if !ok {
		return nil, errors.New("chat not found")
	}
	all := toDomainLinks(chat.Links)
	if limit <= 0 {
		return all, nil
	}
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (r *repository) GetChats(_ context.Context, limit, offset int) (map[int64]domain.Chat, error) {
	chats := make(map[int64]domain.Chat)
	for id, chat := range r.Chats {
		chats[id] = domain.Chat{
			ID:    chat.ID,
			Links: toDomainLinks(chat.Links),
		}
	}
	return chats, nil
}

func (r *repository) Close() {}

func (r *repository) CreateTag(_ context.Context, value string) (int64, error) {
	if value == "" {
		return 0, errors.New("empty tag")
	}
	return 1, nil
}

func (r *repository) ListTags(_ context.Context, _, _ int) ([]domain.Tag, error) {
	return nil, nil
}

func (r *repository) UpdateTag(_ context.Context, _ int64, _ string) error {
	return errors.New("inmemory: tags not supported")
}

func (r *repository) DeleteTag(_ context.Context, _ int64) error {
	return errors.New("inmemory: tags not supported")
}

func (r *repository) DeleteLink(_ context.Context, chatID int64, linkURL string) (domain.Link, error) {
	chat, ok := r.Chats[chatID]
	if !ok {
		return domain.Link{}, errors.New("chat not found")
	}
	for i, l := range chat.Links {
		if l.URL == linkURL {
			removed := chat.Links[i]
			chat.Links = append(chat.Links[:i], chat.Links[i+1:]...)
			r.Chats[chatID] = chat
			return toDomainLink(removed), nil
		}
	}
	return domain.Link{}, errors.New("link not found")
}

func (r *repository) UpdateLinkUpdatedAt(_ context.Context, chatID int64, linkURL string, t time.Time) error {
	chat, ok := r.Chats[chatID]
	if !ok {
		return errors.New("chat not found")
	}
	for i := range chat.Links {
		if chat.Links[i].URL == linkURL {
			chat.Links[i].LastUpdated = t
			r.Chats[chatID] = chat
			return nil
		}
	}
	return errors.New("link not found")
}

func (r *repository) exists(id int64) bool {
	_, ok := r.Chats[id]
	return ok
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

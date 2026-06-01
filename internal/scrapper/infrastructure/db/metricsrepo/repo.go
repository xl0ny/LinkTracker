package metricsrepo

import (
	"context"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/outbox"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/metrics"
)

type LinkCounter interface {
	CountTrackedLinksBySource(ctx context.Context) (map[string]int, error)
}

type inner interface {
	application.ChatRepository
	application.LinkRepository
	application.TagRepository
	application.SchedulerLinks
	Close()
}

type Repository struct {
	inner inner
	m     *metrics.Scrapper
}

func Wrap(r inner, m *metrics.Scrapper) *Repository {
	return &Repository{inner: r, m: m}
}

func (r *Repository) Close() {
	r.inner.Close()
}

func (r *Repository) CountTrackedLinksBySource(ctx context.Context) (map[string]int, error) {
	if c, ok := r.inner.(LinkCounter); ok {
		return timedMap(r.m, metrics.ScopeDatabase, "links", func() (map[string]int, error) {
			return c.CountTrackedLinksBySource(ctx)
		})
	}
	return map[string]int{}, nil
}

func (r *Repository) AddChat(ctx context.Context, id int64) error {
	return timedErr(r.m, metrics.ScopeDatabase, "chats", func() error {
		return r.inner.AddChat(ctx, id)
	})
}

func (r *Repository) DeleteChat(ctx context.Context, id int64) error {
	return timedErr(r.m, metrics.ScopeDatabase, "chats", func() error {
		return r.inner.DeleteChat(ctx, id)
	})
}

func (r *Repository) GetChats(ctx context.Context, limit, offset int) (map[int64]domain.Chat, error) {
	return timedVal(r.m, metrics.ScopeDatabase, "chats", func() (map[int64]domain.Chat, error) {
		return r.inner.GetChats(ctx, limit, offset)
	})
}

func (r *Repository) AddLink(ctx context.Context, chatID int64, link string, tags, filters *[]string) error {
	return timedErr(r.m, metrics.ScopeDatabase, "links", func() error {
		return r.inner.AddLink(ctx, chatID, link, tags, filters)
	})
}

func (r *Repository) GetLinks(ctx context.Context, chatID int64, limit, offset int) ([]domain.Link, error) {
	return timedVal(r.m, metrics.ScopeDatabase, "links", func() ([]domain.Link, error) {
		return r.inner.GetLinks(ctx, chatID, limit, offset)
	})
}

func (r *Repository) DeleteLink(ctx context.Context, chatID int64, linkURL string) (domain.Link, error) {
	return timedVal(r.m, metrics.ScopeDatabase, "links", func() (domain.Link, error) {
		return r.inner.DeleteLink(ctx, chatID, linkURL)
	})
}

func (r *Repository) ListSubscribedLinks(ctx context.Context, limit, offset int) ([]domain.SubscribedLink, error) {
	return timedVal(r.m, metrics.ScopeDatabase, "subscriptions", func() ([]domain.SubscribedLink, error) {
		return r.inner.ListSubscribedLinks(ctx, limit, offset)
	})
}

func (r *Repository) UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error {
	return timedErr(r.m, metrics.ScopeDatabase, "subscriptions", func() error {
		return r.inner.UpdateLinkUpdatedAt(ctx, chatID, linkURL, t)
	})
}

func (r *Repository) CreateTag(ctx context.Context, value string) (int64, error) {
	return timedVal(r.m, metrics.ScopeDatabase, "tag", func() (int64, error) {
		return r.inner.CreateTag(ctx, value)
	})
}

func (r *Repository) ListTags(ctx context.Context, limit, offset int) ([]domain.Tag, error) {
	return timedVal(r.m, metrics.ScopeDatabase, "tag", func() ([]domain.Tag, error) {
		return r.inner.ListTags(ctx, limit, offset)
	})
}

func (r *Repository) UpdateTag(ctx context.Context, id int64, value string) error {
	return timedErr(r.m, metrics.ScopeDatabase, "tag", func() error {
		return r.inner.UpdateTag(ctx, id, value)
	})
}

func (r *Repository) DeleteTag(ctx context.Context, id int64) error {
	return timedErr(r.m, metrics.ScopeDatabase, "tag", func() error {
		return r.inner.DeleteTag(ctx, id)
	})
}

func (r *Repository) SaveOutbox(ctx context.Context, e domain.OutboxEvent) error {
	x, ok := r.inner.(outbox.NotifierRepository)
	if !ok {
		return nil
	}
	return timedErr(r.m, metrics.ScopeDatabase, "outbox", func() error {
		return x.SaveOutbox(ctx, e)
	})
}

func (r *Repository) FetchAndLockOutbox(ctx context.Context, batchSize int, lockFor time.Duration) ([]domain.OutboxEvent, error) {
	x, ok := r.inner.(outbox.PublisherRepository)
	if !ok {
		return nil, nil
	}
	return timedVal(r.m, metrics.ScopeDatabase, "outbox", func() ([]domain.OutboxEvent, error) {
		return x.FetchAndLockOutbox(ctx, batchSize, lockFor)
	})
}

func (r *Repository) MarkOutboxSent(ctx context.Context, id int64) error {
	x, ok := r.inner.(outbox.PublisherRepository)
	if !ok {
		return nil
	}
	return timedErr(r.m, metrics.ScopeDatabase, "outbox", func() error {
		return x.MarkOutboxSent(ctx, id)
	})
}

func (r *Repository) IncrementOutboxAttempts(
	ctx context.Context, id int64, lastErr string, nextAvailableAt time.Time, maxAttempts int,
) error {
	x, ok := r.inner.(outbox.PublisherRepository)
	if !ok {
		return nil
	}
	return timedErr(r.m, metrics.ScopeDatabase, "outbox", func() error {
		return x.IncrementOutboxAttempts(ctx, id, lastErr, nextAvailableAt, maxAttempts)
	})
}

func (r *Repository) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	x, ok := r.inner.(application.TxRunner)
	if !ok {
		return fn(ctx)
	}
	return timedErr(r.m, metrics.ScopeDatabase, "tx", func() error {
		return x.WithTx(ctx, fn)
	})
}

func timedErr(m *metrics.Scrapper, scope, table string, fn func() error) error {
	start := time.Now()
	err := fn()
	metrics.ObserveDuration(m.RequestDuration, scope, table, start)
	return err
}

func timedVal[T any](m *metrics.Scrapper, scope, table string, fn func() (T, error)) (T, error) {
	start := time.Now()
	v, err := fn()
	metrics.ObserveDuration(m.RequestDuration, scope, table, start)
	return v, err
}

func timedMap(m *metrics.Scrapper, scope, table string, fn func() (map[string]int, error)) (map[string]int, error) {
	start := time.Now()
	v, err := fn()
	metrics.ObserveDuration(m.RequestDuration, scope, table, start)
	return v, err
}

package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const subscriptionsPageSize = 100

type linkChecker interface {
	Check(ctx context.Context, link domain.Link) (changed bool, latest time.Time, err error)
}

type botNotifier interface {
	Notify(ctx context.Context, chatID int64, link domain.Link) error
}

type Scheduler struct {
	repo        ChatRepository
	linkChecker linkChecker
	botNotifier botNotifier
}

func NewScheduler(repo ChatRepository, lc linkChecker, bn botNotifier) *Scheduler {
	return &Scheduler{repo: repo, linkChecker: lc, botNotifier: bn}
}

func (s *Scheduler) Run(ctx context.Context) {
	sch := gocron.NewScheduler(time.UTC)
	_, err := sch.Every(1).Minutes().Do(func() { s.checkAllLinks(ctx) })
	if err != nil {
		slog.Error("scheduler: register periodic job", slog.String("error", err.Error()))
		return
	}
	sch.StartAsync()
	<-ctx.Done()
	sch.Stop()
}

func (s *Scheduler) checkAllLinks(ctx context.Context) {
	slog.Info("scheduler: check all links start", slog.Int("page_size", subscriptionsPageSize))
	var processed int
	for offset := 0; ; offset += subscriptionsPageSize {
		subs, err := s.repo.ListSubscriptions(ctx, subscriptionsPageSize, offset)
		if err != nil {
			slog.Error("scheduler: list subscriptions error", slog.Int("offset", offset), slog.String("error", err.Error()))
			return
		}
		if len(subs) == 0 {
			break
		}
		for _, sub := range subs {
			s.processSubscription(ctx, sub)
		}
		processed += len(subs)
		if len(subs) < subscriptionsPageSize {
			break
		}
	}
	slog.Info("scheduler: check all links done", slog.Int("processed", processed))
}

func (s *Scheduler) processSubscription(ctx context.Context, sub domain.Subscription) {
	changed, latest, errCheck := s.linkChecker.Check(ctx, sub.Link)
	if errCheck != nil {
		slog.Warn("scheduler: check link error", slog.String("url", sub.Link.URL), slog.String("error", errCheck.Error()))
		return
	}
	if !changed {
		return
	}
	slog.Info("scheduler: link changed, notifying", slog.Int64("chat_id", sub.ChatID), slog.String("url", sub.Link.URL))
	if errNotify := s.botNotifier.Notify(ctx, sub.ChatID, sub.Link); errNotify != nil {
		slog.Warn("scheduler: notify error", slog.Int64("chat_id", sub.ChatID), slog.String("url", sub.Link.URL), slog.String("error", errNotify.Error()))
	} else {
		slog.Info("scheduler: notify ok", slog.Int64("chat_id", sub.ChatID))
	}
	if err := s.repo.UpdateLinkUpdatedAt(ctx, sub.ChatID, sub.Link.URL, latest); err != nil {
		slog.Warn("scheduler: db new link date updation failed", slog.String("error", err.Error()))
	}
}

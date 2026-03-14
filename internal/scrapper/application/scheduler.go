package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type linksRepo interface {
	GetChats(ctx context.Context) (map[int64]domain.Chat, error)
	UpdateLinkUpdatedAt(ctx context.Context, chatID int64, linkURL string, t time.Time) error
}

type linkChecker interface {
	Check(ctx context.Context, link domain.Link) (changed bool, latest time.Time, err error)
}

type botNotifier interface {
	Notify(ctx context.Context, chatID int64, link domain.Link) error
}

type Scheduler struct {
	repo        linksRepo
	linkChecker linkChecker
	botNotifier botNotifier
}

func NewScheduler(repo linksRepo, lc linkChecker, bn botNotifier) *Scheduler {
	return &Scheduler{repo: repo, linkChecker: lc, botNotifier: bn}
}

func (s *Scheduler) Run(ctx context.Context) {
	sch := gocron.NewScheduler(time.UTC)
	_, _ = sch.Every(1).Minutes().Do(func() { s.checkAllLinks(ctx) })
	sch.StartAsync()
	<-ctx.Done()
	sch.Stop()
}

func (s *Scheduler) checkAllLinks(ctx context.Context) {
	chats, err := s.repo.GetChats(ctx)
	if err != nil {
		slog.Error("get chats failed", slog.String("error", err.Error()), slog.String("event", "checkAllLinks"))
		return
	}
	var totalLinks int
	for _, chat := range chats {
		if chat.ID == nil {
			continue
		}
		totalLinks += len(chat.Links)
	}
	slog.Info("check all links start", slog.Int("chats", len(chats)), slog.Int("links", totalLinks), slog.String("event", "checkAllLinks"))

	for _, chat := range chats {
		if chat.ID == nil {
			continue
		}
		chatID := *chat.ID
		for _, link := range chat.Links {
			changed, latest, errCheck := s.linkChecker.Check(ctx, link)
			if errCheck != nil {
				slog.Warn("check link failed", slog.String("url", link.URL), slog.String("error", errCheck.Error()), slog.String("event", "checkAllLinks"))
				continue
			}
			if changed {
				slog.Info("link changed, notifying", slog.Int64("chat_id", chatID), slog.String("url", link.URL), slog.String("event", "checkAllLinks"))
				if errNotify := s.botNotifier.Notify(ctx, chatID, link); errNotify != nil {
					slog.Warn("notify failed", slog.Int64("chat_id", chatID), slog.String("url", link.URL), slog.String("error", errNotify.Error()))
				} else {
					slog.Info("notify ok", slog.Int64("chat_id", chatID), slog.String("event", "checkAllLinks"))
				}
				_ = s.repo.UpdateLinkUpdatedAt(ctx, chatID, link.URL, latest)
			}
		}
	}
}

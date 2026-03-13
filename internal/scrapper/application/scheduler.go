package application

import (
	"context"
	"log/slog"

	"github.com/go-co-op/gocron"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type linksRepo interface {
	GetChats(ctx context.Context) (map[int64]domain.Chat, error)
}

type linkChecker interface {
	Check(ctx context.Context, link domain.Link) (changed bool, err error)
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
	gocron.Every(5).Minutes().Do(func() { s.checkAllLinks(ctx) })
	<-ctx.Done()
}

func (s *Scheduler) checkAllLinks(ctx context.Context) {
	chats, err := s.repo.GetChats(ctx)
	if err != nil {
		slog.Error("get chats failed", slog.String("error", err.Error()), slog.String("stage", "checkAllLinks"))
		return
	}
	for _, chat := range chats {
		for _, link := range chat.Links {
			changed, err := s.linkChecker.Check(ctx, link)
			if err != nil {
				slog.Warn("check link failed", slog.String("url", link.URL), slog.String("error", err.Error()))
				continue
			}
			if changed {
				s.botNotifier.Notify(ctx, *chat.Id, link)
			}
		}
	}
}

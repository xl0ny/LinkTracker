package application

import (
	"context"
	"log/slog"

	"github.com/go-co-op/gocron"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type linksRepo interface {
	GetAllLinks(ctx context.Context) ([]domain.Link, error)
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
	links, err := s.repo.GetAllLinks(ctx)
	if err != nil {
		slog.Error("get links failed", slog.String("error", err.Error()), slog.String("stage", "checkAllLinks"))
		return
	}
	for _, link := range links {
		changed, err := s.linkChecker.Check(ctx, link)
		if err != nil {
			slog.Warn("check link failed", slog.String("url", link.URL), slog.String("error", err.Error()))
			continue
		}
		if changed {
			// TODO: определить chatID по ссылке и вызвать s.botNotifier.Notify(ctx, chatID, link)
			_ = s.botNotifier
		}
	}
}

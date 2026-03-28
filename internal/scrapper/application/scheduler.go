package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

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
	chats, err := s.repo.GetChats(ctx, 0, 0)
	if err != nil {
		slog.Error("scheduler: get chats error", slog.String("error", err.Error()))
		return
	}
	var totalLinks int
	for _, chat := range chats {
		if chat.ID == nil {
			continue
		}
		totalLinks += len(chat.Links)
	}
	slog.Info("scheduler: check all links start", slog.Int("chats", len(chats)), slog.Int("links", totalLinks))

	for _, chat := range chats {
		if chat.ID == nil {
			continue
		}
		chatID := *chat.ID
		for _, link := range chat.Links {
			changed, latest, errCheck := s.linkChecker.Check(ctx, link)
			if errCheck != nil {
				slog.Warn("scheduler: check link error", slog.String("url", link.URL), slog.String("error", errCheck.Error()))
				continue
			}
			if changed {
				slog.Info("scheduler: link changed, notifying", slog.Int64("chat_id", chatID), slog.String("url", link.URL))
				if errNotify := s.botNotifier.Notify(ctx, chatID, link); errNotify != nil {
					slog.Warn("scheduler: notify error", slog.Int64("chat_id", chatID), slog.String("url", link.URL), slog.String("error", errNotify.Error()))
				} else {
					slog.Info("scheduler: notify ok", slog.Int64("chat_id", chatID))
				}
				err = s.repo.UpdateLinkUpdatedAt(ctx, chatID, link.URL, latest)
				if err != nil {
					slog.Warn("scheduler: db new link date updation failed", slog.String("error", err.Error()))
				}
			}
		}
	}
}

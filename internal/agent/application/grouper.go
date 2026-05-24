package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type UpdatePublisher interface {
	Publish(ctx context.Context, u domain.ProcessedUpdate) error
}

type GrouperConfig struct {
	Window time.Duration
}

type chatGroup struct {
	items []domain.ProcessedUpdate
	timer *time.Timer
}

type Grouper struct {
	publisher UpdatePublisher
	window    time.Duration

	mu     sync.Mutex
	groups map[int64]*chatGroup
}

func NewGrouper(publisher UpdatePublisher, cfg GrouperConfig) *Grouper {
	return &Grouper{
		publisher: publisher,
		window:    cfg.Window,
		groups:    make(map[int64]*chatGroup),
	}
}

// Publish ставит обновление в буфер группировки по каждому tgChatId.
func (g *Grouper) Publish(_ context.Context, update domain.ProcessedUpdate) error {
	for _, chatID := range update.TgChatIDs {
		single := update
		single.TgChatIDs = []int64{chatID}
		g.enqueue(chatID, single)
	}
	return nil
}

// Run ожидает отмены контекста и сбрасывает все ожидающие группы.
func (g *Grouper) Run(ctx context.Context) {
	<-ctx.Done()
	g.flushAll(ctx)
}

func (g *Grouper) enqueue(chatID int64, update domain.ProcessedUpdate) {
	g.mu.Lock()
	defer g.mu.Unlock()

	grp, ok := g.groups[chatID]
	if !ok {
		grp = &chatGroup{}
		g.groups[chatID] = grp
		grp.timer = time.AfterFunc(g.window, func() {
			g.flushChat(chatID)
		})
	}
	grp.items = append(grp.items, update)
}

func (g *Grouper) flushChat(chatID int64) {
	g.mu.Lock()
	grp, ok := g.groups[chatID]
	if !ok {
		g.mu.Unlock()
		return
	}
	items := grp.items
	if grp.timer != nil {
		grp.timer.Stop()
	}
	delete(g.groups, chatID)
	g.mu.Unlock()

	g.publishGroup(context.Background(), items)
}

func (g *Grouper) flushAll(ctx context.Context) {
	g.mu.Lock()
	pending := make(map[int64]*chatGroup, len(g.groups))
	for chatID, grp := range g.groups {
		pending[chatID] = grp
	}
	g.groups = make(map[int64]*chatGroup)
	g.mu.Unlock()

	for _, grp := range pending {
		if grp.timer != nil {
			grp.timer.Stop()
		}
		g.publishGroup(ctx, grp.items)
	}
}

func (g *Grouper) publishGroup(ctx context.Context, items []domain.ProcessedUpdate) {
	if len(items) == 0 {
		return
	}
	out := items[0]
	if len(items) > 1 {
		out = mergeGroupedUpdates(items)
	}
	if err := g.publisher.Publish(ctx, out); err != nil {
		slog.Error("agent-grouper: publish grouped update",
			slog.String("event_id", out.EventID),
			slog.String("error", err.Error()),
		)
	}
}

func mergeGroupedUpdates(items []domain.ProcessedUpdate) domain.ProcessedUpdate {
	first := items[0]
	var b strings.Builder
	priority := first.Priority
	for i, item := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d. %s", i+1, item.Description)
		priority = maxPriority(priority, item.Priority)
	}
	return domain.ProcessedUpdate{
		EventID:     first.EventID,
		OccurredAt:  first.OccurredAt,
		URL:         first.URL,
		Description: b.String(),
		TgChatIDs:   first.TgChatIDs,
		Priority:    priority,
	}
}

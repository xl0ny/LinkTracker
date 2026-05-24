package application

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type recordingPublisher struct {
	mu      sync.Mutex
	updates []domain.ProcessedUpdate
}

func (r *recordingPublisher) Publish(_ context.Context, u domain.ProcessedUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, u)
	return nil
}

func (r *recordingPublisher) snapshot() []domain.ProcessedUpdate {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.ProcessedUpdate, len(r.updates))
	copy(out, r.updates)
	return out
}

func TestGrouper_MultipleUpdatesGrouped(t *testing.T) {
	pub := &recordingPublisher{}
	g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	chatID := int64(42)
	require.NoError(t, g.Publish(context.Background(), domain.ProcessedUpdate{
		EventID: "e1", Description: "first update", TgChatIDs: []int64{chatID}, Priority: PriorityLow,
	}))
	require.NoError(t, g.Publish(context.Background(), domain.ProcessedUpdate{
		EventID: "e2", Description: "second update", TgChatIDs: []int64{chatID}, Priority: PriorityHigh,
	}))

	require.Eventually(t, func() bool {
		return len(pub.snapshot()) == 1
	}, time.Second, 10*time.Millisecond)

	updates := pub.snapshot()
	require.Len(t, updates, 1)
	require.Equal(t, "1. first update\n2. second update", updates[0].Description)
	require.Equal(t, PriorityHigh, updates[0].Priority)
	require.Equal(t, []int64{chatID}, updates[0].TgChatIDs)
}

func TestGrouper_SingleUpdateUnchanged(t *testing.T) {
	pub := &recordingPublisher{}
	g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	original := domain.ProcessedUpdate{
		EventID:     "e1",
		Description: "only one update",
		TgChatIDs:   []int64{99},
		Priority:    PriorityMedium,
	}
	require.NoError(t, g.Publish(context.Background(), original))

	require.Eventually(t, func() bool {
		return len(pub.snapshot()) == 1
	}, time.Second, 10*time.Millisecond)

	updates := pub.snapshot()
	require.Len(t, updates, 1)
	require.Equal(t, original.Description, updates[0].Description)
	require.Equal(t, original.Priority, updates[0].Priority)
	require.Equal(t, original.TgChatIDs, updates[0].TgChatIDs)
}

func TestGrouper_FanOutMultipleChats(t *testing.T) {
	pub := &recordingPublisher{}
	g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	require.NoError(t, g.Publish(context.Background(), domain.ProcessedUpdate{
		EventID: "e1", Description: "shared update", TgChatIDs: []int64{1, 2}, Priority: PriorityMedium,
	}))

	require.Eventually(t, func() bool {
		return len(pub.snapshot()) == 2
	}, time.Second, 10*time.Millisecond)

	updates := pub.snapshot()
	require.Len(t, updates, 2)
	for _, u := range updates {
		require.Equal(t, "shared update", u.Description)
		require.Len(t, u.TgChatIDs, 1)
	}
}

package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type contextKey string

const testContextKey contextKey = "test-key"

func expectPublishes(pub *mocks.MockUpdatePublisher, count int) (<-chan domain.ProcessedUpdate, <-chan context.Context) {
	updates := make(chan domain.ProcessedUpdate, count)
	contexts := make(chan context.Context, count)
	pub.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, u domain.ProcessedUpdate) error {
			updates <- u
			contexts <- ctx
			return nil
		}).
		Times(count)
	return updates, contexts
}

func TestGrouper_MultipleUpdatesGrouped(t *testing.T) {
	ctrl := gomock.NewController(t)
	pub := mocks.NewMockUpdatePublisher(ctrl)
	published, _ := expectPublishes(pub, 1)
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

	got := requirePublished(t, published)
	require.Equal(t, "1. first update\n2. second update", got.Description)
	require.Equal(t, PriorityHigh, got.Priority)
	require.Equal(t, []int64{chatID}, got.TgChatIDs)
}

func TestGrouper_SingleUpdateUnchanged(t *testing.T) {
	ctrl := gomock.NewController(t)
	pub := mocks.NewMockUpdatePublisher(ctrl)
	published, _ := expectPublishes(pub, 1)
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

	got := requirePublished(t, published)
	require.Equal(t, original.Description, got.Description)
	require.Equal(t, original.Priority, got.Priority)
	require.Equal(t, original.TgChatIDs, got.TgChatIDs)
}

func TestGrouper_FanOutMultipleChats(t *testing.T) {
	ctrl := gomock.NewController(t)
	pub := mocks.NewMockUpdatePublisher(ctrl)
	published, _ := expectPublishes(pub, 2)
	g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	require.NoError(t, g.Publish(context.Background(), domain.ProcessedUpdate{
		EventID: "e1", Description: "shared update", TgChatIDs: []int64{1, 2}, Priority: PriorityMedium,
	}))

	updates := []domain.ProcessedUpdate{
		requirePublished(t, published),
		requirePublished(t, published),
	}
	for _, u := range updates {
		require.Equal(t, "shared update", u.Description)
		require.Len(t, u.TgChatIDs, 1)
	}
}

func TestGrouper_UsesPublishContextOnTimedFlush(t *testing.T) {
	ctrl := gomock.NewController(t)
	pub := mocks.NewMockUpdatePublisher(ctrl)
	_, contexts := expectPublishes(pub, 1)
	g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go g.Run(ctx)

	publishCtx := context.WithValue(context.Background(), testContextKey, "publish-context")
	require.NoError(t, g.Publish(publishCtx, domain.ProcessedUpdate{
		EventID: "e1", Description: "ctx update", TgChatIDs: []int64{1}, Priority: PriorityMedium,
	}))

	got := requirePublishContext(t, contexts)
	require.Equal(t, "publish-context", got.Value(testContextKey))
}

func requirePublished(t *testing.T, published <-chan domain.ProcessedUpdate) domain.ProcessedUpdate {
	t.Helper()
	select {
	case got := <-published:
		return got
	case <-time.After(time.Second):
		t.Fatal("publish was not called")
		return domain.ProcessedUpdate{}
	}
}

func requirePublishContext(t *testing.T, published <-chan context.Context) context.Context {
	t.Helper()
	select {
	case got := <-published:
		return got
	case <-time.After(time.Second):
		t.Fatal("publish was not called")
		return context.Background()
	}
}

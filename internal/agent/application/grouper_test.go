package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	chatID := int64(42)
	tests := []struct {
		name                string
		updates             []domain.ProcessedUpdate
		expectedDescription string
		expectedPriority    string
		expectedChatIDs     []int64
		wantErr             bool
	}{
		{
			name: "groups multiple updates",
			updates: []domain.ProcessedUpdate{
				{EventID: "e1", Description: "first update", TgChatIDs: []int64{chatID}, Priority: PriorityLow},
				{EventID: "e2", Description: "second update", TgChatIDs: []int64{chatID}, Priority: PriorityHigh},
			},
			expectedDescription: "1. first update\n2. second update",
			expectedPriority:    PriorityHigh,
			expectedChatIDs:     []int64{chatID},
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			pub := mocks.NewMockUpdatePublisher(ctrl)
			published, _ := expectPublishes(pub, 1)
			g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go g.Run(ctx)

			for _, update := range tt.updates {
				err := g.Publish(context.Background(), update)
				if tt.wantErr {
					assert.Error(t, err)
				} else if !assert.NoError(t, err) {
					return
				}
			}

			got := requirePublished(t, published)
			assert.Equal(t, tt.expectedDescription, got.Description)
			assert.Equal(t, tt.expectedPriority, got.Priority)
			assert.Equal(t, tt.expectedChatIDs, got.TgChatIDs)
		})
	}
}

func TestGrouper_SingleUpdateUnchanged(t *testing.T) {
	tests := []struct {
		name    string
		update  domain.ProcessedUpdate
		wantErr bool
	}{
		{
			name: "keeps single update unchanged",
			update: domain.ProcessedUpdate{
				EventID:     "e1",
				Description: "only one update",
				TgChatIDs:   []int64{99},
				Priority:    PriorityMedium,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			pub := mocks.NewMockUpdatePublisher(ctrl)
			published, _ := expectPublishes(pub, 1)
			g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go g.Run(ctx)

			err := g.Publish(context.Background(), tt.update)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			if !assert.NoError(t, err) {
				return
			}

			got := requirePublished(t, published)
			assert.Equal(t, tt.update.Description, got.Description)
			assert.Equal(t, tt.update.Priority, got.Priority)
			assert.Equal(t, tt.update.TgChatIDs, got.TgChatIDs)
		})
	}
}

func TestGrouper_FanOutMultipleChats(t *testing.T) {
	tests := []struct {
		name                string
		update              domain.ProcessedUpdate
		expectedPublishes   int
		expectedDescription string
		expectedChatIDCount int
		wantErr             bool
	}{
		{
			name: "fans out multiple chats",
			update: domain.ProcessedUpdate{
				EventID: "e1", Description: "shared update", TgChatIDs: []int64{1, 2}, Priority: PriorityMedium,
			},
			expectedPublishes:   2,
			expectedDescription: "shared update",
			expectedChatIDCount: 1,
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			pub := mocks.NewMockUpdatePublisher(ctrl)
			published, _ := expectPublishes(pub, tt.expectedPublishes)
			g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go g.Run(ctx)

			err := g.Publish(context.Background(), tt.update)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			if !assert.NoError(t, err) {
				return
			}

			updates := make([]domain.ProcessedUpdate, 0, tt.expectedPublishes)
			for range tt.expectedPublishes {
				updates = append(updates, requirePublished(t, published))
			}
			for _, u := range updates {
				assert.Equal(t, tt.expectedDescription, u.Description)
				assert.Len(t, u.TgChatIDs, tt.expectedChatIDCount)
			}
		})
	}
}

func TestGrouper_UsesPublishContextOnTimedFlush(t *testing.T) {
	tests := []struct {
		name          string
		contextValue  string
		update        domain.ProcessedUpdate
		expectedValue string
		wantErr       bool
	}{
		{
			name:         "uses publish context on timed flush",
			contextValue: "publish-context",
			update: domain.ProcessedUpdate{
				EventID: "e1", Description: "ctx update", TgChatIDs: []int64{1}, Priority: PriorityMedium,
			},
			expectedValue: "publish-context",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			pub := mocks.NewMockUpdatePublisher(ctrl)
			_, contexts := expectPublishes(pub, 1)
			g := NewGrouper(pub, GrouperConfig{Window: 50 * time.Millisecond})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go g.Run(ctx)

			publishCtx := context.WithValue(context.Background(), testContextKey, tt.contextValue)
			err := g.Publish(publishCtx, tt.update)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			if !assert.NoError(t, err) {
				return
			}

			got := requirePublishContext(t, contexts)
			assert.Equal(t, tt.expectedValue, got.Value(testContextKey))
		})
	}
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

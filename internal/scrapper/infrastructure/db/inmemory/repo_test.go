package inmemory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepository_ChatLifecycle(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "adds lists and deletes chat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := NewRepository()

			require.NoError(t, repo.AddChat(ctx, 1))
			require.Error(t, repo.AddChat(ctx, 1))

			chats, err := repo.GetChats(ctx)
			require.NoError(t, err)
			require.Contains(t, chats, int64(1))
			assert.Equal(t, int64(1), *chats[1].ID)

			require.NoError(t, repo.DeleteChat(ctx, 1))
			assert.Error(t, repo.DeleteChat(ctx, 1))
		})
	}
}

func TestRepository_LinkLifecycle(t *testing.T) {
	tests := []struct {
		name    string
		tags    *[]string
		filters *[]string
	}{
		{
			name:    "adds gets updates and deletes link with metadata",
			tags:    &[]string{"go", "backend"},
			filters: &[]string{"priority=high"},
		},
		{
			name:    "adds link with nil metadata",
			tags:    nil,
			filters: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := NewRepository()
			linkURL := "https://github.com/a/b"
			updatedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)

			require.NoError(t, repo.AddChat(ctx, 1))
			require.NoError(t, repo.AddLink(ctx, 1, linkURL, tt.tags, tt.filters))
			require.Error(t, repo.AddLink(ctx, 1, linkURL, tt.tags, tt.filters))

			links, err := repo.GetLinks(ctx, 1)
			require.NoError(t, err)
			require.Len(t, links, 1)
			assert.Equal(t, linkURL, links[0].URL)
			if tt.tags != nil {
				assert.Equal(t, *tt.tags, links[0].Tags)
			} else {
				assert.Empty(t, links[0].Tags)
			}
			if tt.filters != nil {
				assert.Equal(t, *tt.filters, links[0].Filters)
			} else {
				assert.Empty(t, links[0].Filters)
			}

			require.NoError(t, repo.UpdateLinkUpdatedAt(ctx, 1, linkURL, updatedAt))
			links, err = repo.GetLinks(ctx, 1)
			require.NoError(t, err)
			assert.Equal(t, updatedAt, links[0].LastUpdated)

			removed, err := repo.DeleteLink(ctx, 1, linkURL)
			require.NoError(t, err)
			assert.Equal(t, linkURL, removed.URL)
			_, err = repo.DeleteLink(ctx, 1, linkURL)
			assert.Error(t, err)
		})
	}
}

func TestRepository_NotFoundErrors(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *repository) error
	}{
		{
			name: "add link missing chat",
			run: func(ctx context.Context, repo *repository) error {
				return repo.AddLink(ctx, 404, "https://github.com/a/b", nil, nil)
			},
		},
		{
			name: "get links missing chat",
			run: func(ctx context.Context, repo *repository) error {
				_, err := repo.GetLinks(ctx, 404)
				return err
			},
		},
		{
			name: "delete link missing chat",
			run: func(ctx context.Context, repo *repository) error {
				_, err := repo.DeleteLink(ctx, 404, "https://github.com/a/b")
				return err
			},
		},
		{
			name: "update link missing chat",
			run: func(ctx context.Context, repo *repository) error {
				return repo.UpdateLinkUpdatedAt(ctx, 404, "https://github.com/a/b", time.Now())
			},
		},
		{
			name: "update link missing link",
			run: func(ctx context.Context, repo *repository) error {
				require.NoError(t, repo.AddChat(ctx, 1))
				return repo.UpdateLinkUpdatedAt(ctx, 1, "https://github.com/a/b", time.Now())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(context.Background(), NewRepository())

			assert.Error(t, err)
		})
	}
}

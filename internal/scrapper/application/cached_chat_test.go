package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func newCachedUC(t *testing.T) (*mocks.MockChatUseCase, *mocks.MockLinksCache, *application.CachedChatUC) {
	t.Helper()
	ctrl := gomock.NewController(t)
	inner := mocks.NewMockChatUseCase(ctrl)
	cache := mocks.NewMockLinksCache(ctrl)
	return inner, cache, application.NewCachedChatUC(inner, cache)
}

func TestCachedChatUC_GetLinks_Miss_FetchesAndStores(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chatID   int64
		expected []domain.Link
		wantErr  bool
	}{
		{
			name:     "miss fetches and stores",
			chatID:   42,
			expected: []domain.Link{{URL: "https://example.com"}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, cache, uc := newCachedUC(t)

			cache.EXPECT().Get(gomock.Any(), tt.chatID).Return(nil, false, nil)
			inner.EXPECT().GetLinks(gomock.Any(), tt.chatID).Return(tt.expected, nil)
			cache.EXPECT().Set(gomock.Any(), tt.chatID, tt.expected).Return(nil)

			out, err := uc.GetLinks(context.Background(), tt.chatID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestCachedChatUC_GetLinks_Hit_SkipsInner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chatID   int64
		cached   []domain.Link
		expected []domain.Link
		wantErr  bool
	}{
		{
			name:     "hit skips inner use case",
			chatID:   7,
			cached:   []domain.Link{{URL: "https://cached.example"}},
			expected: []domain.Link{{URL: "https://cached.example"}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cache, uc := newCachedUC(t)

			cache.EXPECT().Get(gomock.Any(), tt.chatID).Return(tt.cached, true, nil)

			out, err := uc.GetLinks(context.Background(), tt.chatID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestCachedChatUC_GetLinks_CacheGetError_FallsBackToInner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chatID   int64
		getErr   error
		expected []domain.Link
		wantErr  bool
	}{
		{
			name:     "cache get error falls back to inner",
			chatID:   9,
			getErr:   errors.New("boom"),
			expected: []domain.Link{{URL: "https://fallback.example"}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, cache, uc := newCachedUC(t)

			cache.EXPECT().Get(gomock.Any(), tt.chatID).Return(nil, false, tt.getErr)
			inner.EXPECT().GetLinks(gomock.Any(), tt.chatID).Return(tt.expected, nil)
			cache.EXPECT().Set(gomock.Any(), tt.chatID, tt.expected).Return(nil)

			out, err := uc.GetLinks(context.Background(), tt.chatID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestCachedChatUC_GetLinks_CacheSetError_DoesNotBreakResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chatID   int64
		setErr   error
		expected []domain.Link
		wantErr  bool
	}{
		{
			name:     "cache set error does not break result",
			chatID:   11,
			setErr:   errors.New("set boom"),
			expected: []domain.Link{{URL: "https://x"}},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, cache, uc := newCachedUC(t)

			cache.EXPECT().Get(gomock.Any(), tt.chatID).Return(nil, false, nil)
			inner.EXPECT().GetLinks(gomock.Any(), tt.chatID).Return(tt.expected, nil)
			cache.EXPECT().Set(gomock.Any(), tt.chatID, tt.expected).Return(tt.setErr)

			out, err := uc.GetLinks(context.Background(), tt.chatID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestCachedChatUC_LinkAddment_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chatID  int64
		url     string
		wantErr bool
	}{
		{
			name:    "invalidates on add success",
			chatID:  3,
			url:     "https://new",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, cache, uc := newCachedUC(t)

			inner.EXPECT().LinkAddment(gomock.Any(), tt.chatID, tt.url, nil, nil).Return(nil)
			cache.EXPECT().Invalidate(gomock.Any(), tt.chatID).Return(nil)

			err := uc.LinkAddment(context.Background(), tt.chatID, tt.url, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCachedChatUC_LinkAddment_InnerError_NoInvalidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chatID   int64
		url      string
		innerErr error
		wantErr  bool
	}{
		{
			name:     "inner add error skips invalidation",
			chatID:   3,
			url:      "https://new",
			innerErr: errors.New("add boom"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, _, uc := newCachedUC(t)

			inner.EXPECT().LinkAddment(gomock.Any(), tt.chatID, tt.url, nil, nil).Return(tt.innerErr)

			err := uc.LinkAddment(context.Background(), tt.chatID, tt.url, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCachedChatUC_DeleteLink_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		chatID      int64
		url         string
		deleted     domain.Link
		expectedURL string
		wantErr     bool
	}{
		{
			name:        "invalidates on delete success",
			chatID:      5,
			url:         "https://removed",
			deleted:     domain.Link{URL: "https://removed"},
			expectedURL: "https://removed",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, cache, uc := newCachedUC(t)

			inner.EXPECT().DeleteLink(gomock.Any(), tt.chatID, tt.url).Return(tt.deleted, nil)
			cache.EXPECT().Invalidate(gomock.Any(), tt.chatID).Return(nil)

			got, err := uc.DeleteLink(context.Background(), tt.chatID, tt.url)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedURL, got.URL)
		})
	}
}

func TestCachedChatUC_ChatDelition_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chatID  int64
		wantErr bool
	}{
		{
			name:    "invalidates on chat deletion success",
			chatID:  8,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, cache, uc := newCachedUC(t)

			inner.EXPECT().ChatDelition(gomock.Any(), tt.chatID).Return(nil)
			cache.EXPECT().Invalidate(gomock.Any(), tt.chatID).Return(nil)

			err := uc.ChatDelition(context.Background(), tt.chatID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCachedChatUC_ChatRegistration_NoCacheTouch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		chatID  int64
		wantErr bool
	}{
		{
			name:    "chat registration does not touch cache",
			chatID:  1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, _, uc := newCachedUC(t)

			inner.EXPECT().ChatRegistration(gomock.Any(), tt.chatID).Return(nil)

			err := uc.ChatRegistration(context.Background(), tt.chatID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCachedChatUC_InvalidateError_DoesNotBreakResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		chatID        int64
		url           string
		invalidateErr error
		wantErr       bool
	}{
		{
			name:          "invalidate error does not break add result",
			chatID:        1,
			url:           "x",
			invalidateErr: errors.New("inv boom"),
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner, cache, uc := newCachedUC(t)

			inner.EXPECT().LinkAddment(gomock.Any(), tt.chatID, tt.url, nil, nil).Return(nil)
			cache.EXPECT().Invalidate(gomock.Any(), tt.chatID).Return(tt.invalidateErr)

			err := uc.LinkAddment(context.Background(), tt.chatID, tt.url, nil, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

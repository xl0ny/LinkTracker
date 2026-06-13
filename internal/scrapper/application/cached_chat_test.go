package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
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

	links := []domain.Link{{URL: "https://example.com"}}
	inner, cache, uc := newCachedUC(t)

	cache.EXPECT().Get(gomock.Any(), int64(42)).Return(nil, false, nil)
	inner.EXPECT().GetLinks(gomock.Any(), int64(42)).Return(links, nil)
	cache.EXPECT().Set(gomock.Any(), int64(42), links).Return(nil)

	out, err := uc.GetLinks(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, links, out)
}

func TestCachedChatUC_GetLinks_Hit_SkipsInner(t *testing.T) {
	t.Parallel()

	cached := []domain.Link{{URL: "https://cached.example"}}
	_, cache, uc := newCachedUC(t)

	cache.EXPECT().Get(gomock.Any(), int64(7)).Return(cached, true, nil)

	out, err := uc.GetLinks(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, cached, out)
}

func TestCachedChatUC_GetLinks_CacheGetError_FallsBackToInner(t *testing.T) {
	t.Parallel()

	links := []domain.Link{{URL: "https://fallback.example"}}
	inner, cache, uc := newCachedUC(t)

	cache.EXPECT().Get(gomock.Any(), int64(9)).Return(nil, false, errors.New("boom"))
	inner.EXPECT().GetLinks(gomock.Any(), int64(9)).Return(links, nil)
	cache.EXPECT().Set(gomock.Any(), int64(9), links).Return(nil)

	out, err := uc.GetLinks(context.Background(), 9)
	require.NoError(t, err)
	require.Equal(t, links, out)
}

func TestCachedChatUC_GetLinks_CacheSetError_DoesNotBreakResult(t *testing.T) {
	t.Parallel()

	links := []domain.Link{{URL: "https://x"}}
	inner, cache, uc := newCachedUC(t)

	cache.EXPECT().Get(gomock.Any(), int64(11)).Return(nil, false, nil)
	inner.EXPECT().GetLinks(gomock.Any(), int64(11)).Return(links, nil)
	cache.EXPECT().Set(gomock.Any(), int64(11), links).Return(errors.New("set boom"))

	out, err := uc.GetLinks(context.Background(), 11)
	require.NoError(t, err)
	require.Equal(t, links, out)
}

func TestCachedChatUC_LinkAddment_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	inner, cache, uc := newCachedUC(t)

	inner.EXPECT().LinkAddment(gomock.Any(), int64(3), "https://new", nil, nil).Return(nil)
	cache.EXPECT().Invalidate(gomock.Any(), int64(3)).Return(nil)

	require.NoError(t, uc.LinkAddment(context.Background(), 3, "https://new", nil, nil))
}

func TestCachedChatUC_LinkAddment_InnerError_NoInvalidation(t *testing.T) {
	t.Parallel()

	inner, _, uc := newCachedUC(t)

	inner.EXPECT().LinkAddment(gomock.Any(), int64(3), "https://new", nil, nil).
		Return(errors.New("add boom"))

	err := uc.LinkAddment(context.Background(), 3, "https://new", nil, nil)
	require.Error(t, err)
}

func TestCachedChatUC_DeleteLink_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	deleted := domain.Link{URL: "https://removed"}
	inner, cache, uc := newCachedUC(t)

	inner.EXPECT().DeleteLink(gomock.Any(), int64(5), "https://removed").Return(deleted, nil)
	cache.EXPECT().Invalidate(gomock.Any(), int64(5)).Return(nil)

	got, err := uc.DeleteLink(context.Background(), 5, "https://removed")
	require.NoError(t, err)
	require.Equal(t, "https://removed", got.URL)
}

func TestCachedChatUC_ChatDelition_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	inner, cache, uc := newCachedUC(t)

	inner.EXPECT().ChatDelition(gomock.Any(), int64(8)).Return(nil)
	cache.EXPECT().Invalidate(gomock.Any(), int64(8)).Return(nil)

	require.NoError(t, uc.ChatDelition(context.Background(), 8))
}

func TestCachedChatUC_ChatRegistration_NoCacheTouch(t *testing.T) {
	t.Parallel()

	inner, _, uc := newCachedUC(t)

	inner.EXPECT().ChatRegistration(gomock.Any(), int64(1)).Return(nil)

	require.NoError(t, uc.ChatRegistration(context.Background(), 1))
}

func TestCachedChatUC_InvalidateError_DoesNotBreakResult(t *testing.T) {
	t.Parallel()

	inner, cache, uc := newCachedUC(t)

	inner.EXPECT().LinkAddment(gomock.Any(), int64(1), "x", nil, nil).Return(nil)
	cache.EXPECT().Invalidate(gomock.Any(), int64(1)).Return(errors.New("inv boom"))

	require.NoError(t, uc.LinkAddment(context.Background(), 1, "x", nil, nil))
}

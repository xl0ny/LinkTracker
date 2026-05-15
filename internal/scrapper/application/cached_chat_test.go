package application_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type fakeInner struct {
	registrationCalls int32
	delitionCalls     int32
	linkAddCalls      int32
	getLinksCalls     int32
	deleteLinkCalls   int32

	links     []domain.Link
	getErr    error
	delErr    error
	addErr    error
	regErr    error
	rmChatErr error

	deleted domain.Link
}

func (f *fakeInner) ChatRegistration(_ context.Context, _ int64) error {
	atomic.AddInt32(&f.registrationCalls, 1)
	return f.regErr
}

func (f *fakeInner) ChatDelition(_ context.Context, _ int64) error {
	atomic.AddInt32(&f.delitionCalls, 1)
	return f.rmChatErr
}

func (f *fakeInner) LinkAddment(_ context.Context, _ int64, _ string, _, _ *[]string) error {
	atomic.AddInt32(&f.linkAddCalls, 1)
	return f.addErr
}

func (f *fakeInner) GetLinks(_ context.Context, _ int64) ([]domain.Link, error) {
	atomic.AddInt32(&f.getLinksCalls, 1)
	return f.links, f.getErr
}

func (f *fakeInner) DeleteLink(_ context.Context, _ int64, _ string) (domain.Link, error) {
	atomic.AddInt32(&f.deleteLinkCalls, 1)
	return f.deleted, f.delErr
}

type fakeCache struct {
	getCalls        int32
	setCalls        int32
	invalidateCalls int32

	stored map[int64][]domain.Link

	getErr     error
	setErr     error
	invErr     error
	forceMiss  bool
	forceFound []domain.Link
}

func newFakeCache() *fakeCache {
	return &fakeCache{stored: make(map[int64][]domain.Link)}
}

func (f *fakeCache) Get(_ context.Context, chatID int64) ([]domain.Link, bool, error) {
	atomic.AddInt32(&f.getCalls, 1)
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	if f.forceMiss {
		return nil, false, nil
	}
	if f.forceFound != nil {
		return f.forceFound, true, nil
	}
	v, ok := f.stored[chatID]
	return v, ok, nil
}

func (f *fakeCache) Set(_ context.Context, chatID int64, links []domain.Link) error {
	atomic.AddInt32(&f.setCalls, 1)
	if f.setErr != nil {
		return f.setErr
	}
	f.stored[chatID] = links
	return nil
}

func (f *fakeCache) Invalidate(_ context.Context, chatID int64) error {
	atomic.AddInt32(&f.invalidateCalls, 1)
	if f.invErr != nil {
		return f.invErr
	}
	delete(f.stored, chatID)
	return nil
}

func TestCachedChatUC_GetLinks_Miss_FetchesAndStores(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{links: []domain.Link{{URL: "https://example.com"}}}
	cache := newFakeCache()
	cache.forceMiss = true

	uc := application.NewCachedChatUC(inner, cache)

	out, err := uc.GetLinks(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, inner.links, out)
	require.Equal(t, int32(1), inner.getLinksCalls)
	require.Equal(t, int32(1), cache.getCalls)
	require.Equal(t, int32(1), cache.setCalls)
}

func TestCachedChatUC_GetLinks_Hit_SkipsInner(t *testing.T) {
	t.Parallel()

	cached := []domain.Link{{URL: "https://cached.example"}}
	inner := &fakeInner{}
	cache := newFakeCache()
	cache.forceFound = cached

	uc := application.NewCachedChatUC(inner, cache)

	out, err := uc.GetLinks(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, cached, out)
	require.Equal(t, int32(0), inner.getLinksCalls)
	require.Equal(t, int32(1), cache.getCalls)
	require.Equal(t, int32(0), cache.setCalls)
}

func TestCachedChatUC_GetLinks_CacheGetError_FallsBackToInner(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{links: []domain.Link{{URL: "https://fallback.example"}}}
	cache := newFakeCache()
	cache.getErr = errors.New("boom")

	uc := application.NewCachedChatUC(inner, cache)

	out, err := uc.GetLinks(context.Background(), 9)
	require.NoError(t, err)
	require.Equal(t, inner.links, out)
	require.Equal(t, int32(1), inner.getLinksCalls)
}

func TestCachedChatUC_GetLinks_CacheSetError_DoesNotBreakResult(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{links: []domain.Link{{URL: "https://x"}}}
	cache := newFakeCache()
	cache.forceMiss = true
	cache.setErr = errors.New("set boom")

	uc := application.NewCachedChatUC(inner, cache)

	out, err := uc.GetLinks(context.Background(), 11)
	require.NoError(t, err)
	require.Equal(t, inner.links, out)
}

func TestCachedChatUC_LinkAddment_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{}
	cache := newFakeCache()
	cache.stored[3] = []domain.Link{{URL: "stale"}}

	uc := application.NewCachedChatUC(inner, cache)

	require.NoError(t, uc.LinkAddment(context.Background(), 3, "https://new", nil, nil))
	require.Equal(t, int32(1), cache.invalidateCalls)
	_, ok := cache.stored[3]
	require.False(t, ok)
}

func TestCachedChatUC_LinkAddment_InnerError_NoInvalidation(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{addErr: errors.New("add boom")}
	cache := newFakeCache()
	cache.stored[3] = []domain.Link{{URL: "stale"}}

	uc := application.NewCachedChatUC(inner, cache)

	err := uc.LinkAddment(context.Background(), 3, "https://new", nil, nil)
	require.Error(t, err)
	require.Equal(t, int32(0), cache.invalidateCalls)
	require.Contains(t, cache.stored, int64(3))
}

func TestCachedChatUC_DeleteLink_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{deleted: domain.Link{URL: "https://removed"}}
	cache := newFakeCache()
	cache.stored[5] = []domain.Link{{URL: "stale"}}

	uc := application.NewCachedChatUC(inner, cache)

	got, err := uc.DeleteLink(context.Background(), 5, "https://removed")
	require.NoError(t, err)
	require.Equal(t, "https://removed", got.URL)
	require.Equal(t, int32(1), cache.invalidateCalls)
}

func TestCachedChatUC_ChatDelition_InvalidatesOnSuccess(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{}
	cache := newFakeCache()
	cache.stored[8] = []domain.Link{{URL: "stale"}}

	uc := application.NewCachedChatUC(inner, cache)

	require.NoError(t, uc.ChatDelition(context.Background(), 8))
	require.Equal(t, int32(1), cache.invalidateCalls)
}

func TestCachedChatUC_ChatRegistration_NoCacheTouch(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{}
	cache := newFakeCache()

	uc := application.NewCachedChatUC(inner, cache)

	require.NoError(t, uc.ChatRegistration(context.Background(), 1))
	require.Equal(t, int32(0), cache.getCalls)
	require.Equal(t, int32(0), cache.setCalls)
	require.Equal(t, int32(0), cache.invalidateCalls)
}

func TestCachedChatUC_InvalidateError_DoesNotBreakResult(t *testing.T) {
	t.Parallel()

	inner := &fakeInner{}
	cache := newFakeCache()
	cache.invErr = errors.New("inv boom")

	uc := application.NewCachedChatUC(inner, cache)

	require.NoError(t, uc.LinkAddment(context.Background(), 1, "x", nil, nil))
	require.Equal(t, int32(1), cache.invalidateCalls)
}

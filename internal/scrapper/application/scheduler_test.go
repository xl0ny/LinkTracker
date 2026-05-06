package application

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type schedulerRepoStub struct {
	mu             sync.Mutex
	links          []domain.SubscribedLink
	updateCalls    int
	updateErrByURL map[string]error
}

func (r *schedulerRepoStub) ListSubscribedLinks(_ context.Context, limit, offset int) ([]domain.SubscribedLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if offset >= len(r.links) {
		return nil, nil
	}
	end := offset + limit
	if end > len(r.links) {
		end = len(r.links)
	}
	out := make([]domain.SubscribedLink, end-offset)
	copy(out, r.links[offset:end])
	return out, nil
}

func (r *schedulerRepoStub) UpdateLinkUpdatedAt(_ context.Context, _ int64, linkURL string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updateCalls++
	if r.updateErrByURL != nil {
		if err, ok := r.updateErrByURL[linkURL]; ok {
			return err
		}
	}
	return nil
}

type schedulerCheckerStub struct {
	mu            sync.Mutex
	checkCalls    int
	changedByURL  map[string]bool
	errByURL      map[string]error
	notifyAtCount map[int]chan struct{}
}

func (c *schedulerCheckerStub) Check(_ context.Context, link domain.Link) (domain.LinkCheckOutcome, error) {
	c.mu.Lock()
	c.checkCalls++
	call := c.checkCalls
	err := error(nil)
	changed := false
	if c.errByURL != nil {
		err = c.errByURL[link.URL]
	}
	if c.changedByURL != nil {
		changed = c.changedByURL[link.URL]
	}
	ch := c.notifyAtCount[call]
	c.mu.Unlock()
	if ch != nil {
		close(ch)
	}
	if err != nil {
		return domain.LinkCheckOutcome{}, err
	}
	return domain.LinkCheckOutcome{
		Changed:     changed,
		Latest:      time.Now(),
		Description: "test update",
	}, nil
}

type schedulerNotifierStub struct {
	mu                    sync.Mutex
	notifyCalls           int
	notifyFailedLinksCall int
	notifyErrByURL        map[string]error
}

func (n *schedulerNotifierStub) Notify(_ context.Context, _ int64, link domain.Link, _ string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifyCalls++
	if n.notifyErrByURL != nil {
		if err, ok := n.notifyErrByURL[link.URL]; ok {
			return err
		}
	}
	return nil
}

func (n *schedulerNotifierStub) NotifyFailedLinks(_ context.Context, _ int64, _ []string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notifyFailedLinksCall++
	return nil
}

func TestScheduler_Run_StartsWorkersOnce(t *testing.T) {
	repo := &schedulerRepoStub{
		links: []domain.SubscribedLink{
			{ChatID: 1, Link: domain.Link{URL: "https://example.com/1"}},
			{ChatID: 1, Link: domain.Link{URL: "https://example.com/2"}},
		},
	}
	checker := &schedulerCheckerStub{
		changedByURL: map[string]bool{
			"https://example.com/1": false,
			"https://example.com/2": false,
		},
	}
	notifier := &schedulerNotifierStub{}

	s := NewScheduler(repo, checker, notifier, 100, 3, 200*time.Millisecond)
	var started int32
	s.onWorkerStart = func() {
		atomic.AddInt32(&started, 1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()

	time.Sleep(60 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	require.Equal(t, int32(3), atomic.LoadInt32(&started))
}

func TestScheduler_Run_ProcessesTasks(t *testing.T) {
	baseline := time.Unix(1, 0).UTC()
	repo := &schedulerRepoStub{
		links: []domain.SubscribedLink{
			{ChatID: 10, Link: domain.Link{URL: "https://example.com/a", LastUpdated: baseline}},
			{ChatID: 11, Link: domain.Link{URL: "https://example.com/b", LastUpdated: baseline}},
		},
	}
	first := make(chan struct{})
	second := make(chan struct{})
	checker := &schedulerCheckerStub{
		changedByURL: map[string]bool{
			"https://example.com/a": true,
			"https://example.com/b": false,
		},
		notifyAtCount: map[int]chan struct{}{
			1: first,
			2: second,
		},
	}
	notifier := &schedulerNotifierStub{}

	s := NewScheduler(repo, checker, notifier, 100, 2, 24*time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()

	select {
	case <-first:
	case <-time.After(2 * time.Second):
		t.Fatal("first task was not checked")
	}
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("second task was not checked")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	checker.mu.Lock()
	checkCalls := checker.checkCalls
	checker.mu.Unlock()
	notifier.mu.Lock()
	notifyCalls := notifier.notifyCalls
	notifier.mu.Unlock()
	repo.mu.Lock()
	updateCalls := repo.updateCalls
	repo.mu.Unlock()

	require.GreaterOrEqual(t, checkCalls, 2)
	require.Equal(t, 1, notifyCalls)
	require.Equal(t, 1, updateCalls)
}

func TestScheduler_Run_ShutdownOnCancel(t *testing.T) {
	repo := &schedulerRepoStub{}
	checker := &schedulerCheckerStub{}
	notifier := &schedulerNotifierStub{}
	s := NewScheduler(repo, checker, notifier, 100, 2, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop by cancel")
	}
}

func TestScheduler_Run_ErrorsDoNotPanic(t *testing.T) {
	baseline := time.Unix(1, 0).UTC()
	repo := &schedulerRepoStub{
		links: []domain.SubscribedLink{
			{ChatID: 1, Link: domain.Link{URL: "https://example.com/check-err", LastUpdated: baseline}},
			{ChatID: 1, Link: domain.Link{URL: "https://example.com/notify-err", LastUpdated: baseline}},
			{ChatID: 1, Link: domain.Link{URL: "https://example.com/repo-err", LastUpdated: baseline}},
		},
		updateErrByURL: map[string]error{
			"https://example.com/repo-err": errors.New("repo error"),
		},
	}
	checker := &schedulerCheckerStub{
		changedByURL: map[string]bool{
			"https://example.com/check-err":  false,
			"https://example.com/notify-err": true,
			"https://example.com/repo-err":   true,
		},
		errByURL: map[string]error{
			"https://example.com/check-err": errors.New("checker error"),
		},
	}
	notifier := &schedulerNotifierStub{
		notifyErrByURL: map[string]error{
			"https://example.com/notify-err": errors.New("notify error"),
		},
	}
	s := NewScheduler(repo, checker, notifier, 100, 2, 24*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
}

package application

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func expectListLinks(repo *mocks.MockSchedulerLinks, links []domain.SubscribedLink) {
	repo.EXPECT().
		ListSubscribedLinks(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, limit, offset int) ([]domain.SubscribedLink, error) {
			if offset >= len(links) {
				return nil, nil
			}
			end := offset + limit
			if end > len(links) {
				end = len(links)
			}
			out := make([]domain.SubscribedLink, end-offset)
			copy(out, links[offset:end])
			return out, nil
		}).
		AnyTimes()
}

func expectChecker(
	checker *mocks.MockLinkChecker,
	changedByURL map[string]bool,
	errByURL map[string]error,
	notifyAtCount map[int]chan struct{},
) {
	var calls atomic.Int32
	checker.EXPECT().Check(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, link domain.Link) (domain.LinkCheckOutcome, error) {
			n := calls.Add(1)
			if ch := notifyAtCount[int(n)]; ch != nil {
				close(ch)
			}
			if errByURL != nil {
				if e, ok := errByURL[link.URL]; ok {
					return domain.LinkCheckOutcome{}, e
				}
			}
			changed := false
			if changedByURL != nil {
				changed = changedByURL[link.URL]
			}
			return domain.LinkCheckOutcome{
				Changed:     changed,
				Latest:      time.Now(),
				Description: "test update",
			}, nil
		},
	).AnyTimes()
}

func TestScheduler_Run_StartsWorkersOnce(t *testing.T) {
	tests := []struct{ name string }{
		{name: "scheduler run starts workers once"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			baseline := time.Unix(1, 0).UTC()
			links := []domain.SubscribedLink{
				{ChatID: 1, Link: domain.Link{URL: "https://example.com/1", LastUpdated: baseline}},
				{ChatID: 1, Link: domain.Link{URL: "https://example.com/2", LastUpdated: baseline}},
			}
			repo := mocks.NewMockSchedulerLinks(ctrl)
			checker := mocks.NewMockLinkChecker(ctrl)
			notifier := mocks.NewMockBotNotifier(ctrl)

			expectListLinks(repo, links)
			expectChecker(checker, map[string]bool{
				"https://example.com/1": false,
				"https://example.com/2": false,
			}, nil, nil)

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

			assert.Equal(t, int32(3), atomic.LoadInt32(&started))
		})
	}
}

func TestScheduler_Run_ProcessesTasks(t *testing.T) {
	tests := []struct{ name string }{
		{name: "scheduler run processes tasks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			baseline := time.Unix(1, 0).UTC()
			links := []domain.SubscribedLink{
				{ChatID: 10, Link: domain.Link{URL: "https://example.com/a", LastUpdated: baseline}},
				{ChatID: 11, Link: domain.Link{URL: "https://example.com/b", LastUpdated: baseline}},
			}
			repo := mocks.NewMockSchedulerLinks(ctrl)
			checker := mocks.NewMockLinkChecker(ctrl)
			notifier := mocks.NewMockBotNotifier(ctrl)

			expectListLinks(repo, links)
			first := make(chan struct{})
			second := make(chan struct{})
			expectChecker(checker, map[string]bool{
				"https://example.com/a": true,
				"https://example.com/b": false,
			}, nil, map[int]chan struct{}{
				1: first,
				2: second,
			})
			notifier.EXPECT().
				Notify(gomock.Any(), int64(10), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil).
				Times(1)
			repo.EXPECT().
				UpdateLinkUpdatedAt(gomock.Any(), int64(10), "https://example.com/a", gomock.Any()).
				Return(nil).
				Times(1)

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
		})
	}
}

func TestScheduler_Run_ShutdownOnCancel(t *testing.T) {
	tests := []struct{ name string }{
		{name: "scheduler run shutdown on cancel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			repo := mocks.NewMockSchedulerLinks(ctrl)
			checker := mocks.NewMockLinkChecker(ctrl)
			notifier := mocks.NewMockBotNotifier(ctrl)

			expectListLinks(repo, nil)

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
		})
	}
}

func TestScheduler_Run_ErrorsDoNotPanic(t *testing.T) {
	tests := []struct{ name string }{
		{name: "scheduler run errors do not panic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)

			baseline := time.Unix(1, 0).UTC()
			links := []domain.SubscribedLink{
				{ChatID: 1, Link: domain.Link{URL: "https://example.com/check-err", LastUpdated: baseline}},
				{ChatID: 1, Link: domain.Link{URL: "https://example.com/notify-err", LastUpdated: baseline}},
				{ChatID: 1, Link: domain.Link{URL: "https://example.com/repo-err", LastUpdated: baseline}},
			}
			repo := mocks.NewMockSchedulerLinks(ctrl)
			checker := mocks.NewMockLinkChecker(ctrl)
			notifier := mocks.NewMockBotNotifier(ctrl)

			expectListLinks(repo, links)
			expectChecker(checker, map[string]bool{
				"https://example.com/check-err":  false,
				"https://example.com/notify-err": true,
				"https://example.com/repo-err":   true,
			}, map[string]error{
				"https://example.com/check-err": errors.New("checker error"),
			}, nil)
			notifier.EXPECT().
				Notify(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(errors.New("notify error")).
				AnyTimes()
			notifier.EXPECT().
				NotifyFailedLinks(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil).
				AnyTimes()
			repo.EXPECT().
				UpdateLinkUpdatedAt(gomock.Any(), gomock.Any(), "https://example.com/repo-err", gomock.Any()).
				Return(errors.New("repo error")).
				AnyTimes()

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
		})
	}
}

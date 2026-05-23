package notifier

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type stubNotifier struct {
	notifyErr       error
	notifyFailedErr error
	called          string
}

func (s *stubNotifier) Notify(_ context.Context, _ int64, _ domain.Link, _, _ string) error {
	s.called = "notify"
	return s.notifyErr
}

func (s *stubNotifier) NotifyFailedLinks(_ context.Context, _ int64, _ []string) error {
	s.called = "failed"
	return s.notifyFailedErr
}

func TestFallback_httpDownUsesKafka(t *testing.T) {
	primary := &stubNotifier{notifyErr: errors.New("http down")}
	fallback := &stubNotifier{}
	fb := NewFallback(primary, fallback)

	err := fb.Notify(context.Background(), 1, domain.Link{URL: "https://example.com"}, "x", "u")
	require.NoError(t, err)
	require.Equal(t, "notify", fallback.called)
}

func TestFallback_bothFailReturnsError(t *testing.T) {
	primary := &stubNotifier{notifyErr: errors.New("http down")}
	fallback := &stubNotifier{notifyErr: errors.New("kafka down")}
	fb := NewFallback(primary, fallback)

	err := fb.Notify(context.Background(), 1, domain.Link{URL: "https://example.com"}, "x", "u")
	require.Error(t, err)
	require.Contains(t, err.Error(), "http down")
	require.Contains(t, err.Error(), "kafka down")
}

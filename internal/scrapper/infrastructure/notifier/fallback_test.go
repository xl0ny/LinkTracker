package notifier_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/notifier"
)

func TestFallback_httpDownUsesKafka(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "fallback http down uses kafka"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			primary := mocks.NewMockBotNotifier(ctrl)
			fallback := mocks.NewMockBotNotifier(ctrl)

			link := domain.Link{URL: "https://example.com"}
			primary.EXPECT().Notify(gomock.Any(), int64(1), link, "x", "u").Return(errors.New("http down"))
			fallback.EXPECT().Notify(gomock.Any(), int64(1), link, "x", "u").Return(nil)

			fb := notifier.NewFallback(primary, fallback)
			err := fb.Notify(context.Background(), 1, link, "x", "u")
			assert.NoError(t, err)
		})
	}
}

func TestFallback_bothFailReturnsError(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "fallback both fail returns error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			primary := mocks.NewMockBotNotifier(ctrl)
			fallback := mocks.NewMockBotNotifier(ctrl)

			link := domain.Link{URL: "https://example.com"}
			primary.EXPECT().Notify(gomock.Any(), int64(1), link, "x", "u").Return(errors.New("http down"))
			fallback.EXPECT().Notify(gomock.Any(), int64(1), link, "x", "u").Return(errors.New("kafka down"))

			fb := notifier.NewFallback(primary, fallback)
			err := fb.Notify(context.Background(), 1, link, "x", "u")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "http down")
			assert.Contains(t, err.Error(), "kafka down")
		})
	}
}

package command

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type trackStub struct {
	addedTags []string
}

func (s *trackStub) RegisterChat(context.Context, int64) error { return nil }

func (s *trackStub) AddLink(_ context.Context, _ int64, _ string, tags []string) error {
	s.addedTags = tags
	return nil
}

func (s *trackStub) RemoveLink(context.Context, int64, string) error { return nil }

func (s *trackStub) ListLinks(context.Context, int64, string) ([]application.LinkInfo, error) {
	return nil, nil
}

func TestTrack_HandlePlainMessage_skipTagsWithoutSlash(t *testing.T) {
	tracker := &trackStub{}
	state := application.NewTrackStateStore()
	state.Set(42, &application.TrackState{
		Phase: application.TrackPhaseTags,
		Link:  "https://github.com/openai/codex",
	})
	cmd := NewTrack(tracker, state)

	text, err := cmd.HandlePlainMessage(domain.Action{ChatID: 42, Text: " skip "})

	require.NoError(t, err)
	assert.Equal(t, "Ссылка добавлена в отслеживание", text)
	assert.Empty(t, tracker.addedTags)
	assert.Nil(t, state.Get(42))
}

func TestTrack_HandlePlainMessage_parsesTags(t *testing.T) {
	tracker := &trackStub{}
	state := application.NewTrackStateStore()
	state.Set(42, &application.TrackState{
		Phase: application.TrackPhaseTags,
		Link:  "https://github.com/openai/codex",
	})
	cmd := NewTrack(tracker, state)

	_, err := cmd.HandlePlainMessage(domain.Action{ChatID: 42, Text: "go, review"})

	require.NoError(t, err)
	assert.Equal(t, []string{"go", "review"}, tracker.addedTags)
}

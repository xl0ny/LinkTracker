package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	appmocks "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestTrack_HandlePlainMessage_skipTagsWithoutSlash(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedText string
		expectedTags []string
		wantErr      bool
	}{
		{
			name:         "skip tags without slash",
			input:        " skip ",
			expectedText: "Ссылка добавлена в отслеживание",
			expectedTags: nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tracker := appmocks.NewMockLinkTracker(ctrl)
			state := application.NewTrackStateStore()
			state.Set(42, &application.TrackState{
				Phase: application.TrackPhaseTags,
				Link:  "https://github.com/openai/codex",
			})
			tracker.EXPECT().
				AddLink(gomock.Any(), int64(42), "https://github.com/openai/codex", tt.expectedTags).
				Return(nil)
			cmd := NewTrack(tracker, state)

			text, err := cmd.HandlePlainMessage(domain.Action{ChatID: 42, Text: tt.input})

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedText, text)
			assert.Nil(t, state.Get(42))
		})
	}
}

func TestTrack_HandlePlainMessage_parsesTags(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedTags []string
		wantErr      bool
	}{
		{
			name:         "parses comma-separated tags",
			input:        "go, review",
			expectedTags: []string{"go", "review"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tracker := appmocks.NewMockLinkTracker(ctrl)
			state := application.NewTrackStateStore()
			state.Set(42, &application.TrackState{
				Phase: application.TrackPhaseTags,
				Link:  "https://github.com/openai/codex",
			})
			tracker.EXPECT().
				AddLink(gomock.Any(), int64(42), "https://github.com/openai/codex", tt.expectedTags).
				Return(nil)
			cmd := NewTrack(tracker, state)

			_, err := cmd.HandlePlainMessage(domain.Action{ChatID: 42, Text: tt.input})

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

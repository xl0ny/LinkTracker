package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	appmocks "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/mocks"
)

func TestCommands(t *testing.T) {
	tests := []struct {
		name          string
		expectedNames []string
	}{
		{
			name:          "returns all bot commands in menu order",
			expectedNames: []string{"start", "help", "track", "untrack", "list"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tracker := appmocks.NewMockLinkTracker(ctrl)
			track := NewTrack(tracker, application.NewTrackStateStore())

			commands := Commands(tracker, track)
			names := make([]string, 0, len(commands))
			for _, cmd := range commands {
				names = append(names, cmd.Name())
			}

			assert.Equal(t, tt.expectedNames, names)
		})
	}
}

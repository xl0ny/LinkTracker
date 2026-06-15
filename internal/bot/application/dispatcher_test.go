package application_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestDispatcher_Dispatch_start_returnsWelcomeMessage(t *testing.T) {
	tests := []struct {
		name             string
		action           domain.Action
		commandName      string
		commandReply     string
		expectedContains []string
		wantErr          bool
	}{
		{
			name:             "start returns welcome message",
			action:           domain.Action{Command: "start"},
			commandName:      "start",
			commandReply:     "Добро пожаловать! Используйте /help для списка команд.",
			expectedContains: []string{"Добро пожаловать! Используйте /help"},
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			start := mocks.NewMockCommand(ctrl)
			start.EXPECT().Name().Return(tt.commandName).AnyTimes()
			start.EXPECT().Description().Return("").AnyTimes()
			start.EXPECT().Handle(tt.action).Return(tt.commandReply, nil)

			d := application.NewDispatcher([]application.Command{start}, nil, application.NewTrackStateStore(), nil)
			text, err := d.Dispatch(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotEmpty(t, text)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, text, expected)
			}
		})
	}
}

func TestDispatcher_Dispatch_help_returnsCommandList(t *testing.T) {
	tests := []struct {
		name             string
		action           domain.Action
		commandName      string
		commandReply     string
		expectedContains []string
		wantErr          bool
	}{
		{
			name:             "help returns command list",
			action:           domain.Action{Command: "help"},
			commandName:      "help",
			commandReply:     "/start\n/help",
			expectedContains: []string{"/start", "/help"},
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			help := mocks.NewMockCommand(ctrl)
			help.EXPECT().Name().Return(tt.commandName).AnyTimes()
			help.EXPECT().Description().Return("").AnyTimes()
			help.EXPECT().Handle(tt.action).Return(tt.commandReply, nil)

			d := application.NewDispatcher([]application.Command{help}, nil, application.NewTrackStateStore(), nil)
			text, err := d.Dispatch(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotEmpty(t, text)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, text, expected)
			}
		})
	}
}

func TestDispatcher_Dispatch_unknownCommand_returnsErrorMessage(t *testing.T) {
	tests := []struct {
		name             string
		action           domain.Action
		expectedContains []string
		wantErr          bool
	}{
		{
			name:             "unknown command returns error message",
			action:           domain.Action{Command: "unknowncommand"},
			expectedContains: []string{"Неизвестная команда", "/help"},
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := application.NewDispatcher([]application.Command{}, nil, application.NewTrackStateStore(), nil)

			text, err := d.Dispatch(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotEmpty(t, text)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, text, expected)
			}
		})
	}
}

func TestDispatcher_Dispatch_emptyCommand_noResponse(t *testing.T) {
	tests := []struct {
		name     string
		action   domain.Action
		expected string
		wantErr  bool
	}{
		{
			name:     "empty command returns no response",
			action:   domain.Action{Command: ""},
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := application.NewDispatcher([]application.Command{}, nil, application.NewTrackStateStore(), nil)

			text, err := d.Dispatch(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, text)
		})
	}
}

func TestDispatcher_Dispatch_commandDuringTrackFlow_goesToPlainHandler(t *testing.T) {
	tests := []struct {
		name         string
		action       domain.Action
		plainReply   string
		expectedText string
		wantErr      bool
	}{
		{
			name:         "command during track flow goes to plain handler",
			action:       domain.Action{ChatID: 42, Command: "skip", Text: "/skip"},
			plainReply:   "Ссылка добавлена в отслеживание",
			expectedText: "Ссылка добавлена в отслеживание",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			state := application.NewTrackStateStore()
			state.Set(42, &application.TrackState{Phase: application.TrackPhaseTags, Link: "https://github.com/openai/codex"})
			plain := mocks.NewMockPlainMessageHandler(ctrl)
			plain.EXPECT().
				HandlePlainMessage(tt.action).
				Return(tt.plainReply, nil)
			d := application.NewDispatcher(nil, plain, state, nil)

			text, err := d.Dispatch(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedText, text)
		})
	}
}

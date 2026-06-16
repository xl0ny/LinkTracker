package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestStart_Name(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "returns command name",
			expected: commandStart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewStart(nil)

			assert.Equal(t, tt.expected, cmd.Name())
		})
	}
}

func TestStart_Description(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "returns non-empty description"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewStart(nil)

			assert.NotEmpty(t, cmd.Description())
		})
	}
}

func TestStart_Handle_returnsWelcomeMessage(t *testing.T) {
	tests := []struct {
		name     string
		action   domain.Action
		expected string
		wantErr  bool
	}{
		{
			name:     "returns welcome message",
			action:   domain.Action{Command: commandStart},
			expected: "Добро пожаловать! Используйте /help для списка команд.",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewStart(nil)

			text, err := cmd.Handle(tt.action)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, text)
		})
	}
}

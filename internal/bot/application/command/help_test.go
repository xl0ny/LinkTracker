package command

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestHelp_Name(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "returns command name",
			expected: "help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd Help

			assert.Equal(t, tt.expected, cmd.Name())
		})
	}
}

func TestHelp_Description(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "returns non-empty description"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd Help

			assert.NotEmpty(t, cmd.Description())
		})
	}
}

func TestHelp_Handle_returnsCommandList(t *testing.T) {
	tests := []struct {
		name             string
		action           domain.Action
		expectedContains []string
		wantErr          bool
	}{
		{
			name:             "returns command list",
			action:           domain.Action{Command: "help"},
			expectedContains: []string{"/start", "/help"},
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd Help

			text, err := cmd.Handle(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			for _, expected := range tt.expectedContains {
				assert.Contains(t, text, expected)
			}
		})
	}
}

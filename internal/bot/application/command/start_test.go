package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestStart_Name(t *testing.T) {
	cmd := NewStart(nil)
	assert.Equal(t, "start", cmd.Name())
}

func TestStart_Description(t *testing.T) {
	cmd := NewStart(nil)
	require.NotEmpty(t, cmd.Description())
}

func TestStart_Handle_returnsWelcomeMessage(t *testing.T) {
	cmd := NewStart(nil)
	action := domain.Action{Command: "start"}
	text, send := cmd.Handle(action)
	require.True(t, send)
	assert.Equal(t, "Добро пожаловать! Используйте /help для списка команд.", text)
}

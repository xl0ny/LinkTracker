package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestHelp_Name(t *testing.T) {
	var cmd Help
	assert.Equal(t, "help", cmd.Name())
}

func TestHelp_Description(t *testing.T) {
	var cmd Help
	require.NotEmpty(t, cmd.Description())
}

func TestHelp_Handle_returnsCommandList(t *testing.T) {
	var cmd Help
	action := domain.Action{Command: "help"}
	text, err := cmd.Handle(action)
	require.NoError(t, err)
	assert.Contains(t, text, "/start")
	assert.Contains(t, text, "/help")
}

package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestDispatcher_Dispatch_start_returnsWelcomeMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCmd := NewMockCommand(ctrl)
	mockCmd.EXPECT().
		Name().
		Return("start")
	mockCmd.EXPECT().
		Handle(gomock.Any()).
		Return("Добро пожаловать! Используйте /help для списка команд.", nil)

	d := NewDispatcher([]Command{mockCmd}, nil, NewTrackStateStore())
	action := domain.Action{Command: "start"}
	text, err := d.Dispatch(action)
	require.NoError(t, err)
	require.NotEmpty(t, text)
	assert.Contains(t, text, "Добро пожаловать! Используйте /help")
}

func TestDispatcher_Dispatch_help_returnsCommandList(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockCmd := NewMockCommand(ctrl)
	mockCmd.EXPECT().
		Name().
		Return("help")
	mockCmd.EXPECT().
		Handle(gomock.Any()).
		Return("/start\n/help", nil)

	d := NewDispatcher([]Command{mockCmd}, nil, NewTrackStateStore())
	action := domain.Action{Command: "help"}
	text, err := d.Dispatch(action)
	require.NoError(t, err)
	require.NotEmpty(t, text)
	assert.Contains(t, text, "/start")
	assert.Contains(t, text, "/help")
}

func TestDispatcher_Dispatch_unknownCommand_returnsErrorMessage(t *testing.T) {
	d := NewDispatcher([]Command{}, nil, NewTrackStateStore())
	action := domain.Action{Command: "unknowncommand"}
	text, err := d.Dispatch(action)
	require.NoError(t, err)
	require.NotEmpty(t, text)
	assert.Contains(t, text, "Неизвестная команда")
	assert.Contains(t, text, "/help")
}

func TestDispatcher_Dispatch_emptyCommand_noResponse(t *testing.T) {
	d := NewDispatcher([]Command{}, nil, NewTrackStateStore())
	action := domain.Action{Command: ""}
	text, err := d.Dispatch(action)
	require.NoError(t, err)
	assert.Empty(t, text)
}

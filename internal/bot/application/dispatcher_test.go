package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type fakeCmd struct {
	name, desc, reply string
}

func (f fakeCmd) Name() string { return f.name }

func (f fakeCmd) Description() string { return f.desc }

func (f fakeCmd) Handle(domain.Action) (string, error) { return f.reply, nil }

func TestDispatcher_Dispatch_start_returnsWelcomeMessage(t *testing.T) {
	d := NewDispatcher([]Command{fakeCmd{"start", "", "Добро пожаловать! Используйте /help для списка команд."}}, nil, NewTrackStateStore())
	action := domain.Action{Command: "start"}
	text, err := d.Dispatch(action)
	require.NoError(t, err)
	require.NotEmpty(t, text)
	assert.Contains(t, text, "Добро пожаловать! Используйте /help")
}

func TestDispatcher_Dispatch_help_returnsCommandList(t *testing.T) {
	d := NewDispatcher([]Command{fakeCmd{"help", "", "/start\n/help"}}, nil, NewTrackStateStore())
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

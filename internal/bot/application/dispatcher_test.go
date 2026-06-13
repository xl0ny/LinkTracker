package application_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type fakePlain struct {
	action domain.Action
	reply  string
}

func (f *fakePlain) HandlePlainMessage(action domain.Action) (string, error) {
	f.action = action
	return f.reply, nil
}

func TestDispatcher_Dispatch_start_returnsWelcomeMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	start := mocks.NewMockCommand(ctrl)
	start.EXPECT().Name().Return("start").AnyTimes()
	start.EXPECT().Description().Return("").AnyTimes()
	start.EXPECT().Handle(domain.Action{Command: "start"}).
		Return("Добро пожаловать! Используйте /help для списка команд.", nil)

	d := application.NewDispatcher([]application.Command{start}, nil, application.NewTrackStateStore(), nil)
	text, err := d.Dispatch(domain.Action{Command: "start"})

	require.NoError(t, err)
	require.NotEmpty(t, text)
	assert.Contains(t, text, "Добро пожаловать! Используйте /help")
}

func TestDispatcher_Dispatch_help_returnsCommandList(t *testing.T) {
	ctrl := gomock.NewController(t)
	help := mocks.NewMockCommand(ctrl)
	help.EXPECT().Name().Return("help").AnyTimes()
	help.EXPECT().Description().Return("").AnyTimes()
	help.EXPECT().Handle(domain.Action{Command: "help"}).
		Return("/start\n/help", nil)

	d := application.NewDispatcher([]application.Command{help}, nil, application.NewTrackStateStore(), nil)
	text, err := d.Dispatch(domain.Action{Command: "help"})

	require.NoError(t, err)
	require.NotEmpty(t, text)
	assert.Contains(t, text, "/start")
	assert.Contains(t, text, "/help")
}

func TestDispatcher_Dispatch_unknownCommand_returnsErrorMessage(t *testing.T) {
	d := application.NewDispatcher([]application.Command{}, nil, application.NewTrackStateStore(), nil)
	text, err := d.Dispatch(domain.Action{Command: "unknowncommand"})

	require.NoError(t, err)
	require.NotEmpty(t, text)
	assert.Contains(t, text, "Неизвестная команда")
	assert.Contains(t, text, "/help")
}

func TestDispatcher_Dispatch_emptyCommand_noResponse(t *testing.T) {
	d := application.NewDispatcher([]application.Command{}, nil, application.NewTrackStateStore(), nil)
	text, err := d.Dispatch(domain.Action{Command: ""})

	require.NoError(t, err)
	assert.Empty(t, text)
}

func TestDispatcher_Dispatch_commandDuringTrackFlow_goesToPlainHandler(t *testing.T) {
	state := application.NewTrackStateStore()
	state.Set(42, &application.TrackState{Phase: application.TrackPhaseTags, Link: "https://github.com/openai/codex"})
	plain := &fakePlain{reply: "Ссылка добавлена в отслеживание"}
	d := application.NewDispatcher(nil, plain, state, nil)

	text, err := d.Dispatch(domain.Action{ChatID: 42, Command: "skip", Text: "/skip"})

	require.NoError(t, err)
	assert.Equal(t, "Ссылка добавлена в отслеживание", text)
	assert.Equal(t, "/skip", plain.action.Text)
}

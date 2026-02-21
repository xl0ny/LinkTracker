package bot

import (
	"log/slog"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

const (
	workerCount = 5
	actionBuf   = 100
)

func Run(cfg *Config) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken.String())
	if err != nil {
		slog.Error("инициализация бота", slog.String("error", err.Error()), slog.String("event", "bot_init"))
		os.Exit(1)
	}
	// api.Debug = true
	slog.Info("бот авторизован", slog.String("bot_username", api.Self.UserName), slog.String("event", "authorized"))

	setMenuCommands(api, commands.All())

	actions := make(chan model.Action, actionBuf)
	dispatcher := NewDispatcher(commands.All())

	for i := 0; i < workerCount; i++ {
		go Worker(actions, api, dispatcher)
	}
	go ReceiveUpdates(api, actions)

	select {}
}

// setMenuCommands регистрирует команды в меню бота (setMyCommands).
func setMenuCommands(api *tgbotapi.BotAPI, cmds []model.Command) {
	botCommands := make([]tgbotapi.BotCommand, 0, len(cmds))
	for _, c := range cmds {
		botCommands = append(botCommands, tgbotapi.BotCommand{
			Command:     c.Name(),
			Description: c.Description(),
		})
	}
	resp, err := api.Request(tgbotapi.NewSetMyCommands(botCommands...))
	if err != nil {
		slog.Warn("не удалось установить меню команд", slog.String("error", err.Error()), slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
		return
	}
	if !resp.Ok {
		slog.Warn("ответ API setMyCommands не Ok", slog.String("description", resp.Description), slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
		return
	}
	slog.Info("меню команд установлено", slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
}

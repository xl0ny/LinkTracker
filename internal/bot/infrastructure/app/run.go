package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/commands"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
)

const workerCount = 5

func Run(cfg *config.Config) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		slog.Error("bot initialization", slog.String("error", err.Error()), slog.String("event", "bot_init"))
		os.Exit(1)
	}
	slog.Info("bot authorized", slog.String("bot_username", api.Self.UserName), slog.String("event", "authorized"))

	bot := telegram.NewBot(api)

	setMenuCommands(api, commands.All())

	actions := bot.ReceiveUpdates(ctx)
	dispatcher := application.NewDispatcher(commands.All())

	for i := 0; i < workerCount; i++ {
		go application.Worker(actions, dispatcher, bot)
	}
	<-ctx.Done()
	slog.Info("shutting down", slog.String("event", "shutdown"))
	os.Exit(0)
}

func setMenuCommands(api *tgbotapi.BotAPI, cmds []commands.Command) {
	botCommands := make([]tgbotapi.BotCommand, 0, len(cmds))
	for _, c := range cmds {
		botCommands = append(botCommands, tgbotapi.BotCommand{
			Command:     c.Name(),
			Description: c.Description(),
		})
	}
	resp, err := api.Request(tgbotapi.NewSetMyCommands(botCommands...))
	if err != nil {
		slog.Warn("failed to set commands menu", slog.String("error", err.Error()), slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
		return
	}
	if !resp.Ok {
		slog.Warn("setMyCommands API response not Ok", slog.String("description", resp.Description), slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
		return
	}
	slog.Info("commands menu set", slog.Int("commands_count", len(botCommands)), slog.String("event", "set_commands"))
}

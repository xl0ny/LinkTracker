package bot

import (
	"log"

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
		log.Fatal(err)
	}
	api.Debug = true
	log.Printf("Authorized as %s", api.Self.UserName)

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
	_, _ = api.Request(tgbotapi.NewSetMyCommands(botCommands...))
}

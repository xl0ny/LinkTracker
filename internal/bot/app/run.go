package app

import (
	"context"
	"fmt"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/command"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/infrastructure/telegram"
)

const workerCount = 5

func Run(ctx context.Context, cfg *config.Config) error {
	bot, err := telegram.NewBot(cfg.TelegramToken)
	if err != nil {
		return fmt.Errorf("error on running bot stage: %w", err)
	}

	commands := []application.Command{
		command.Start{},
		command.Help{},
	}
	bot.SetMenuCommands(commands)

	actions := bot.ReceiveUpdates(ctx)
	dispatcher := application.NewDispatcher(commands)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			application.Worker(actions, dispatcher, bot)
		})
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}

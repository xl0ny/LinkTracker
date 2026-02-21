package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

// Worker обрабатывает действия из канала через диспетчер. Блокирующая горутина.
func Worker(actions <-chan model.Action, api *tgbotapi.BotAPI, d *Dispatcher) {
	for action := range actions {
		text, send := d.Dispatch(action, api)
		if !send {
			continue
		}
		msg := tgbotapi.NewMessage(action.ChatID, text)
		_, _ = api.Send(msg)
	}
}

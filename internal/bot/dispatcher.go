package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"
)

// Dispatcher маршрутизирует действия по зарегистрированным командам.
type Dispatcher struct {
	handlers map[string]model.Command
}

// NewDispatcher строит диспетчер из списка команд (commands.All()).
func NewDispatcher(cmds []model.Command) *Dispatcher {
	h := make(map[string]model.Command, len(cmds))
	for _, c := range cmds {
		h[c.Name()] = c
	}
	return &Dispatcher{handlers: h}
}

// Dispatch вызывает обработчик команды или возвращает сообщение об ошибке для неизвестной команд
func (d *Dispatcher) Dispatch(action model.Action, api *tgbotapi.BotAPI) (text string, send bool) {
	if action.Command == "" {
		return "", false
	}
	if cmd, ok := d.handlers[action.Command]; ok {
		return cmd.Handle(action, api)
	}
	return "Неизвестная команда. Используйте /help для списка команд", true
}

func (d *Dispatcher) GetComandsStrings() []string {
	commands := make([]string, len(d.handlers))
	for name, _ := range d.handlers {
		commands = append(commands, name)
	}
	return commands
}

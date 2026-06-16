package command

import (
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Help struct{}

func (Help) Name() string { return commandHelp }

func (Help) Description() string { return "Список доступных команд" }

func (Help) Handle(_ domain.Action) (string, error) {
	return "/start — начало работы\n/help — список команд\n/track — добавить ссылку\n/untrack — убрать ссылку\n/list — список ссылок", nil
}

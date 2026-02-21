package commands

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/model"

// чтобы добавить новую команду -> новый файл в commands/ + запись сюда.
func All() []model.Command {
	return []model.Command{
		Start{},
		Help{},
	}
}

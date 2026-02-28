package commands

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"

func All() []domain.Command {
	return []domain.Command{
		Start{},
		Help{},
	}
}

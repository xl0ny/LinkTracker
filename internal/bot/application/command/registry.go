package command

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"

// Commands возвращает все команды бота для меню и диспетчера.
// track должен быть тем же экземпляром, что передаётся в Dispatcher как обработчик продолжения диалога /track.
func Commands(tracker application.LinkTracker, track *Track) []application.Command {
	return []application.Command{
		NewStart(tracker),
		Help{},
		track,
		NewUntrack(tracker),
		NewList(tracker),
	}
}

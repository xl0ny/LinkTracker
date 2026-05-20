package command

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"

// Commands возвращает все команды бота для меню и диспетчера.
// track должен быть тем же экземпляром, что передаётся в Dispatcher как обработчик продолжения диалога /track.
// Один и тот же конкретный клиент можно передать в forStart, forUntrack и forList — типы интерфейсов разные.
func Commands(forStart chatRegistrar, track *Track, forUntrack linkRemover, forList linkLister) []application.Command {
	return []application.Command{
		NewStart(forStart),
		Help{},
		track,
		NewUntrack(forUntrack),
		NewList(forList),
	}
}

package command

import "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"

func All(tracker application.LinkTracker, state *application.TrackStateStore) ([]application.Command, *Track) {
	track := NewTrack(tracker, state)
	cmds := []application.Command{
		NewStart(tracker),
		Help{},
		track,
		NewUntrack(tracker),
		NewList(tracker),
	}
	return cmds, track
}

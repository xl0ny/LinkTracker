package command

import (
	"context"
	"strings"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

type Track struct {
	tracker application.LinkTracker
	state   *application.TrackStateStore
}

func NewTrack(tracker application.LinkTracker, state *application.TrackStateStore) *Track {
	return &Track{tracker: tracker, state: state}
}

func (t *Track) Name() string { return "track" }

func (t *Track) Description() string { return "Начать отслеживание ссылки" }

func (t *Track) Handle(action domain.Action) (string, bool) {
	t.state.Set(action.ChatID, &application.TrackState{Phase: application.TrackPhaseLink})
	return "Отправьте ссылку на отслеживаемый ресурс (GitHub или StackOverflow)", true
}

func (t *Track) HandleContinuation(ctx context.Context, action domain.Action, state *application.TrackState) (string, bool, *application.TrackState) {
	text := strings.TrimSpace(action.Text)
	switch state.Phase {
	case application.TrackPhaseLink:
		if !application.IsValidLink(text) {
			return "Некорректная ссылка. Поддерживаются только GitHub и StackOverflow.", true, nil
		}
		return "Отправьте теги через запятую или /skip", true, &application.TrackState{
			Phase: application.TrackPhaseTags,
			Link:  text,
		}
	case application.TrackPhaseTags:
		var tags []string
		if text != "" && text != "/skip" {
			tags = application.ParseTags(text)
		}
		if err := t.tracker.AddLink(ctx, action.ChatID, state.Link, tags); err != nil {
			if err.Error() == "link already exists" {
				return "Ссылка уже отслеживается", true, nil
			}
			if err.Error() == errChatNotFound {
				return msgUseStart, true, nil
			}
			return "Ошибка добавления ссылки", true, nil
		}
		return "Ссылка добавлена в отслеживание", true, nil
	}
	return "", false, nil
}

package command

import (
	"context"
	"errors"
	"fmt"
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

func (t *Track) Handle(action domain.Action) (string, error) {
	t.state.Set(action.ChatID, &application.TrackState{Phase: application.TrackPhaseLink})
	return "Отправьте ссылку на отслеживаемый ресурс (GitHub или StackOverflow)", nil
}

func (t *Track) HandlePlainMessage(action domain.Action) (string, error) {
	st := t.state.Get(action.ChatID)
	if st == nil {
		return "", nil
	}
	return t.handleInFlow(context.Background(), action, st)
}

func (t *Track) handleInFlow(ctx context.Context, action domain.Action, state *application.TrackState) (string, error) {
	resp, doSend, newSt, err := t.step(ctx, action, state)
	t.state.Set(action.ChatID, newSt)
	if err != nil {
		return "", err
	}
	if !doSend {
		return "", nil
	}
	return resp, nil
}

func (t *Track) step(ctx context.Context, action domain.Action, state *application.TrackState) (string, bool, *application.TrackState, error) {
	text := strings.TrimSpace(action.Text)
	switch state.Phase {
	case application.TrackPhaseLink:
		if !application.IsValidLink(text) {
			return "Некорректная ссылка. Поддерживаются только GitHub и StackOverflow.", true, nil, nil
		}
		return "Отправьте теги через запятую или /skip", true, &application.TrackState{
			Phase: application.TrackPhaseTags,
			Link:  text,
		}, nil
	case application.TrackPhaseTags:
		var tags []string
		if text != "" && !isSkipTagsInput(text) {
			tags = application.ParseTags(text)
		}
		if err := t.tracker.AddLink(ctx, action.ChatID, state.Link, tags); err != nil {
			if errors.Is(err, application.ErrLinkAlreadyExists) {
				return "Ссылка уже отслеживается", true, nil, nil
			}
			if errors.Is(err, application.ErrChatNotFound) {
				return msgUseStart, true, nil, nil
			}
			return "", false, nil, fmt.Errorf("add link: %w", err)
		}
		return "Ссылка добавлена в отслеживание", true, nil, nil
	}
	return "", false, nil, nil
}

func isSkipTagsInput(text string) bool {
	normalized := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(text)), "/")
	return normalized == "skip"
}

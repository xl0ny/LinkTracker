package application

import (
	"context"
	"log/slog"
	"unicode/utf8"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type Summarizer interface {
	Summarize(ctx context.Context, text string) (string, error)
}

const defaultPriority = "HIGH"

type Processor struct {
	filter     *Filter
	summarizer Summarizer
	threshold  int
}

func NewProcessor(filter *Filter, summarizer Summarizer, threshold int) *Processor {
	return &Processor{filter: filter, summarizer: summarizer, threshold: threshold}
}

// фильтрация и суммаризация.
// возвращает (processed, true, nil) если событие готово к публикации,
// (zero, false, nil) — если отфильтровано.
// ошибка возвращается только при фатальном сбое; ошибка суммаризатора деградирует до original-текста.
func (p *Processor) Process(ctx context.Context, raw domain.RawUpdate) (domain.ProcessedUpdate, bool, error) {
	if reason := p.filter.Decide(raw); reason != SkipNone {
		slog.Info("agent: update filtered",
			slog.String("event_id", raw.EventID),
			slog.String("reason", reason.String()),
			slog.String("author", raw.Author),
			slog.String("url", raw.URL),
		)
		return domain.ProcessedUpdate{}, false, nil
	}

	description := raw.Description
	if p.summarizer != nil && p.threshold > 0 && utf8.RuneCountInString(description) > p.threshold {
		summary, err := p.summarizer.Summarize(ctx, description)
		switch {
		case err != nil:
			slog.Warn("agent: summarization failed, using original",
				slog.String("event_id", raw.EventID),
				slog.String("error", err.Error()))
		case summary != "":
			description = summary
		}
	}

	return domain.ProcessedUpdate{
		EventID:     raw.EventID,
		OccurredAt:  raw.OccurredAt,
		URL:         raw.URL,
		Description: description,
		TgChatIDs:   raw.TgChatIDs,
		Priority:    defaultPriority,
	}, true, nil
}

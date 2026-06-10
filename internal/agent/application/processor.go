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

type Processor struct {
	filter      *Filter
	summarizer  Summarizer
	threshold   int
	prioritizer *Prioritizer
}

func NewProcessor(filter *Filter, summarizer Summarizer, threshold int, prioritizer *Prioritizer) *Processor {
	return &Processor{filter: filter, summarizer: summarizer, threshold: threshold, prioritizer: prioritizer}
}

// Process выполняет фильтрацию и суммаризацию.
// Возвращает (processed, true) если событие готово к публикации,
// (zero, false) — если отфильтровано.
// Ошибка суммаризатора деградирует до original-текста.
func (p *Processor) Process(ctx context.Context, raw domain.RawUpdate) (domain.ProcessedUpdate, bool) {
	if reason := p.filter.Decide(raw); reason != SkipNone {
		slog.Info("agent: update filtered",
			slog.String("event_id", raw.EventID),
			slog.String("reason", reason.String()),
			slog.String("author", raw.Author),
			slog.String("url", raw.URL),
		)
		return domain.ProcessedUpdate{}, false
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

	priority := PriorityMedium
	if p.prioritizer != nil {
		priority = p.prioritizer.Prioritize(description)
	}

	return domain.ProcessedUpdate{
		EventID:     raw.EventID,
		OccurredAt:  raw.OccurredAt,
		URL:         raw.URL,
		Description: description,
		TgChatIDs:   raw.TgChatIDs,
		Priority:    priority,
	}, true
}

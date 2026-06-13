package application

import (
	"context"
	"fmt"
	"log/slog"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type RawUpdateProcessor interface {
	Process(ctx context.Context, raw domain.RawUpdate) (domain.ProcessedUpdate, bool)
}

type UpdateHandler struct {
	processor RawUpdateProcessor
	publisher UpdatePublisher
}

func NewUpdateHandler(processor RawUpdateProcessor, publisher UpdatePublisher) *UpdateHandler {
	return &UpdateHandler{processor: processor, publisher: publisher}
}

func (h *UpdateHandler) Handle(ctx context.Context, raw domain.RawUpdate) error {
	processed, ok := h.processor.Process(ctx, raw)
	if !ok {
		return nil
	}
	if err := h.publisher.Publish(ctx, processed); err != nil {
		return fmt.Errorf("publish processed update: %w", err)
	}
	slog.Info("agent: processed update published",
		slog.String("event_id", processed.EventID),
		slog.String("url", processed.URL),
		slog.Int("chats", len(processed.TgChatIDs)),
	)
	return nil
}

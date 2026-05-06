// Package outbox implements the Transactional Outbox pattern for the scrapper service:
// the Notifier writes Avro-encoded events into an outbox table inside the same DB transaction
// as the domain change, while the Publisher delivers them to Kafka asynchronously.
package outbox

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/avro/registry"
	commoncfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	scrapperkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/kafka"
)

type NotifierRepository interface {
	SaveOutbox(ctx context.Context, e domain.OutboxEvent) error
}

type Notifier struct {
	repo        NotifierRepository
	enc         *registry.Encoder
	updateTopic string
	failedTopic string
}

func NewNotifier(ctx context.Context, repo NotifierRepository, k commoncfg.Kafka) (*Notifier, error) {
	if k.SchemaRegistryURL == "" {
		return nil, errors.New("outbox-notifier: schema_registry_url required")
	}
	enc, err := registry.NewEncoder(ctx, k.SchemaRegistryURL, k.UpdateSubject, k.FailedSubject,
		scrapperkafka.UpdateAvroSchemaPath, scrapperkafka.FailedAvroSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("outbox-notifier: schema registry encoder: %w", err)
	}
	return &Notifier{
		repo:        repo,
		enc:         enc,
		updateTopic: k.UpadateLinksTopic,
		failedTopic: k.FailedLinksTopic,
	}, nil
}

func (n *Notifier) Notify(ctx context.Context, chatID int64, link domain.Link, description string) error {
	var descriptionValue any
	if description != "" {
		descriptionValue = map[string]any{"string": description}
	}

	eventID := uuid.NewString()
	native := map[string]any{
		"eventId":     eventID,
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"url":         link.URL,
		"description": descriptionValue,
	}
	payload, err := n.enc.EncodeUpdate(native)
	if err != nil {
		return fmt.Errorf("outbox-notifier: encode update: %w", err)
	}

	if savErr := n.repo.SaveOutbox(ctx, domain.OutboxEvent{
		EventID: eventID,
		Topic:   n.updateTopic,
		Key:     []byte(strconv.FormatInt(chatID, 10)),
		Payload: payload,
	}); savErr != nil {
		return fmt.Errorf("outbox-notifier: save update event: %w", savErr)
	}
	return nil
}

func (n *Notifier) NotifyFailedLinks(ctx context.Context, chatID int64, links []string) error {
	if len(links) == 0 {
		return nil
	}
	description := "Не удалось обработать ссылки:\n" + strings.Join(links, "\n")

	eventID := uuid.NewString()
	native := map[string]any{
		"eventId":     eventID,
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"description": description,
	}
	payload, err := n.enc.EncodeFailed(native)
	if err != nil {
		return fmt.Errorf("outbox-notifier: encode failed: %w", err)
	}

	if savErr := n.repo.SaveOutbox(ctx, domain.OutboxEvent{
		EventID: eventID,
		Topic:   n.failedTopic,
		Key:     []byte(strconv.FormatInt(chatID, 10)),
		Payload: payload,
	}); savErr != nil {
		return fmt.Errorf("outbox-notifier: save failed-links event: %w", savErr)
	}
	return nil
}

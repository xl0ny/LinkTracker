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

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	scrapperkafka "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/kafka"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
	commoncfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
)

type NotifierRepository interface {
	SaveOutbox(ctx context.Context, e domain.OutboxEvent) error
}

type Notifier struct {
	repo        NotifierRepository
	rawEnc      *registry.SingleEncoder
	failedEnc   *registry.SingleEncoder
	rawTopic    string
	failedTopic string
}

func NewNotifier(ctx context.Context, repo NotifierRepository, k commoncfg.Kafka) (*Notifier, error) {
	if k.SchemaRegistryURL == "" {
		return nil, errors.New("outbox-notifier: schema_registry_url required")
	}
	if k.RawUpdateSubject == "" || k.FailedSubject == "" {
		return nil, errors.New("outbox-notifier: raw_update_subject and failed_subject required")
	}
	if k.RawUpdatesTopic == "" || k.FailedLinksTopic == "" {
		return nil, errors.New("outbox-notifier: raw_updates_topic and failed_links_topic required")
	}
	rawEnc, err := registry.NewSingleEncoder(ctx, k.SchemaRegistryURL, k.RawUpdateSubject, scrapperkafka.RawUpdateAvroSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("outbox-notifier: register raw encoder: %w", err)
	}
	failedEnc, err := registry.NewSingleEncoder(ctx, k.SchemaRegistryURL, k.FailedSubject, scrapperkafka.FailedAvroSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("outbox-notifier: register failed encoder: %w", err)
	}
	return &Notifier{
		repo:        repo,
		rawEnc:      rawEnc,
		failedEnc:   failedEnc,
		rawTopic:    k.RawUpdatesTopic,
		failedTopic: k.FailedLinksTopic,
	}, nil
}

func (n *Notifier) Notify(ctx context.Context, chatID int64, link domain.Link, description, author string) error {
	eventID := uuid.NewString()
	native := map[string]any{
		"eventId":     eventID,
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"url":         link.URL,
		"description": description,
		"author":      author,
		"tgChatIds":   []any{chatID},
	}
	payload, err := n.rawEnc.Encode(native)
	if err != nil {
		return fmt.Errorf("outbox-notifier: encode raw update: %w", err)
	}

	if savErr := n.repo.SaveOutbox(ctx, domain.OutboxEvent{
		EventID: eventID,
		Topic:   n.rawTopic,
		Key:     []byte(strconv.FormatInt(chatID, 10)),
		Payload: payload,
	}); savErr != nil {
		return fmt.Errorf("outbox-notifier: save raw event: %w", savErr)
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
	payload, err := n.failedEnc.Encode(native)
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

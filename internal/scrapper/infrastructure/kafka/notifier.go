package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/avro/registry"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

const (
	UpdateAvroSchemaPath = "schemas/avro/link_update_event.avsc"
	FailedAvroSchemaPath = "schemas/avro/failed_links_event.avsc"
)

// Notifier sends link events directly to Kafka (non-outbox producer mode).
type Notifier struct {
	updateWriter *kafkago.Writer
	failedWriter *kafkago.Writer
	enc          *registry.Encoder
}

func NewNotifier(ctx context.Context, kconfig config.Kafka, pconfig config.KafkaProducer) (*Notifier, error) {
	if kconfig.SchemaRegistryURL == "" {
		return nil, errors.New("kafka-notifier: schema_registry_url required")
	}
	if kconfig.UpdateSubject == "" || kconfig.FailedSubject == "" {
		return nil, errors.New("kafka-notifier: update_subject and failed_subject required")
	}
	enc, err := registry.NewEncoder(ctx, kconfig.SchemaRegistryURL, kconfig.UpdateSubject, kconfig.FailedSubject,
		UpdateAvroSchemaPath, FailedAvroSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("kafka-notifier: schema registry encoder: %w", err)
	}

	dialer := &kafkago.Dialer{ClientID: pconfig.ProducerClient}
	base := kafkago.WriterConfig{
		Brokers:      kconfig.Brokers,
		WriteTimeout: pconfig.WriteTimeout,
		RequiredAcks: pconfig.RequiredACK,
		MaxAttempts:  pconfig.MaxAttempts,
		Dialer:       dialer,
	}

	updateCfg, failedCfg := base, base
	updateCfg.Topic, failedCfg.Topic = kconfig.UpadateLinksTopic, kconfig.FailedLinksTopic

	return &Notifier{
		updateWriter: kafkago.NewWriter(updateCfg),
		failedWriter: kafkago.NewWriter(failedCfg),
		enc:          enc,
	}, nil
}

func (p *Notifier) Notify(ctx context.Context, chatID int64, link domain.Link, description string) error {
	var descriptionValue any
	if description != "" {
		descriptionValue = map[string]any{"string": description}
	}

	native := map[string]any{
		"eventId":     uuid.NewString(),
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"url":         link.URL,
		"description": descriptionValue,
	}
	value, err := p.enc.EncodeUpdate(native)
	if err != nil {
		return fmt.Errorf("kafka-notifier: encode update payload: %w", err)
	}

	err = p.updateWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.FormatInt(chatID, 10)),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("kafka-notifier: write update kafka message: %w", err)
	}
	return nil
}

func (p *Notifier) NotifyFailedLinks(ctx context.Context, chatID int64, links []string) error {
	if len(links) == 0 {
		return nil
	}
	description := "Не удалось обработать ссылки:\n" + strings.Join(links, "\n")

	native := map[string]any{
		"eventId":     uuid.NewString(),
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"description": description,
	}
	value, err := p.enc.EncodeFailed(native)
	if err != nil {
		return fmt.Errorf("kafka-notifier: encode failed avro payload: %w", err)
	}

	err = p.failedWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.FormatInt(chatID, 10)),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("kafka-notifier: write failed kafka message: %w", err)
	}
	return nil
}

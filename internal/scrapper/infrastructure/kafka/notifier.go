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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/metrics"
)

const (
	RawUpdateAvroSchemaPath = "schemas/avro/link_raw_update_event.avsc"
	FailedAvroSchemaPath    = "schemas/avro/failed_links_event.avsc"
)

// Notifier sends link events directly to Kafka (non-outbox producer mode).
type Notifier struct {
	rawWriter    *kafkago.Writer
	failedWriter *kafkago.Writer
	rawEnc       *registry.SingleEncoder
	failedEnc    *registry.SingleEncoder
	metrics      *metrics.Scrapper
}

func NewNotifier(ctx context.Context, kconfig config.Kafka, pconfig config.KafkaProducer, m *metrics.Scrapper) (*Notifier, error) {
	if kconfig.SchemaRegistryURL == "" {
		return nil, errors.New("kafka-notifier: schema_registry_url required")
	}
	if kconfig.RawUpdateSubject == "" || kconfig.FailedSubject == "" {
		return nil, errors.New("kafka-notifier: raw_update_subject and failed_subject required")
	}
	if kconfig.RawUpdatesTopic == "" || kconfig.FailedLinksTopic == "" {
		return nil, errors.New("kafka-notifier: raw_updates_topic and failed_links_topic required")
	}

	rawEnc, err := registry.NewSingleEncoder(ctx, kconfig.SchemaRegistryURL, kconfig.RawUpdateSubject, RawUpdateAvroSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("kafka-notifier: register raw encoder: %w", err)
	}
	failedEnc, err := registry.NewSingleEncoder(ctx, kconfig.SchemaRegistryURL, kconfig.FailedSubject, FailedAvroSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("kafka-notifier: register failed encoder: %w", err)
	}

	dialer := &kafkago.Dialer{ClientID: pconfig.ProducerClient}
	base := kafkago.WriterConfig{
		Brokers:      kconfig.Brokers,
		WriteTimeout: pconfig.WriteTimeout,
		RequiredAcks: pconfig.RequiredACK,
		MaxAttempts:  pconfig.MaxAttempts,
		Dialer:       dialer,
	}

	rawCfg, failedCfg := base, base
	rawCfg.Topic, failedCfg.Topic = kconfig.RawUpdatesTopic, kconfig.FailedLinksTopic

	return &Notifier{
		rawWriter:    kafkago.NewWriter(rawCfg),
		failedWriter: kafkago.NewWriter(failedCfg),
		rawEnc:       rawEnc,
		failedEnc:    failedEnc,
		metrics:      m,
	}, nil
}

func (p *Notifier) Notify(ctx context.Context, chatID int64, link domain.Link, description, author string) error {
	native := map[string]any{
		"eventId":     uuid.NewString(),
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"url":         link.URL,
		"description": description,
		"author":      author,
		"tgChatIds":   []any{chatID},
	}
	value, err := p.rawEnc.Encode(native)
	if err != nil {
		return fmt.Errorf("kafka-notifier: encode raw payload: %w", err)
	}

	err = writeMessages(ctx, p.rawWriter, p.metrics, p.rawWriter.Topic, kafkago.Message{
		Key:   []byte(strconv.FormatInt(chatID, 10)),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("kafka-notifier: write raw kafka message: %w", err)
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
	value, err := p.failedEnc.Encode(native)
	if err != nil {
		return fmt.Errorf("kafka-notifier: encode failed avro payload: %w", err)
	}

	err = writeMessages(ctx, p.failedWriter, p.metrics, p.failedWriter.Topic, kafkago.Message{
		Key:   []byte(strconv.FormatInt(chatID, 10)),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("kafka-notifier: write failed kafka message: %w", err)
	}
	return nil
}

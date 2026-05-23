package kafka

import (
	"context"
	"errors"
	"fmt"

	kafkago "github.com/segmentio/kafka-go"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
)

const ProcessedAvroSchemaPath = "schemas/avro/link_processed_update_event.avsc"

type Producer struct {
	writer *kafkago.Writer
	enc    *registry.SingleEncoder
}

func NewProducer(ctx context.Context, kconfig config.Kafka, pconfig config.KafkaProducer) (*Producer, error) {
	if kconfig.SchemaRegistryURL == "" {
		return nil, errors.New("agent-producer: schema_registry_url required")
	}
	if kconfig.ProcessedUpdateSubject == "" || kconfig.ProcessedUpdatesTopic == "" {
		return nil, errors.New("agent-producer: processed_update_subject and processed_updates_topic required")
	}
	enc, err := registry.NewSingleEncoder(ctx, kconfig.SchemaRegistryURL, kconfig.ProcessedUpdateSubject, ProcessedAvroSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("agent-producer: register processed encoder: %w", err)
	}

	dialer := &kafkago.Dialer{ClientID: pconfig.ProducerClient}
	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers:      kconfig.Brokers,
		Topic:        kconfig.ProcessedUpdatesTopic,
		WriteTimeout: pconfig.WriteTimeout,
		RequiredAcks: pconfig.RequiredACK,
		MaxAttempts:  pconfig.MaxAttempts,
		Dialer:       dialer,
	})
	return &Producer{writer: writer, enc: enc}, nil
}

func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("agent-producer: close writer: %w", err)
	}
	return nil
}

func (p *Producer) Publish(ctx context.Context, u domain.ProcessedUpdate) error {
	tgChatIDs := make([]any, 0, len(u.TgChatIDs))
	for _, id := range u.TgChatIDs {
		tgChatIDs = append(tgChatIDs, id)
	}
	native := map[string]any{
		"eventId":     u.EventID,
		"occurredAt":  u.OccurredAt.UTC().UnixMilli(),
		"url":         u.URL,
		"description": u.Description,
		"tgChatIds":   tgChatIDs,
		"priority":    u.Priority,
	}
	value, err := p.enc.Encode(native)
	if err != nil {
		return fmt.Errorf("agent-producer: encode processed payload: %w", err)
	}
	if wErr := p.writer.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(u.EventID),
		Value: value,
	}); wErr != nil {
		return fmt.Errorf("agent-producer: write processed kafka message: %w", wErr)
	}
	return nil
}

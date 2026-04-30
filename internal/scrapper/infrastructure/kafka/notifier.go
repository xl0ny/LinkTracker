package kafka

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linkedin/goavro/v2"
	"github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type notifier struct {
	updateWriter *kafka.Writer
	failedWriter *kafka.Writer
	updateCodec  *goavro.Codec
	failedCodec  *goavro.Codec
}

func NewNotifier(kconfig config.Kafka, pconfig config.KafkaProducer) notifier {
	base := kafka.WriterConfig{
		Brokers:      kconfig.Brokers,
		WriteTimeout: pconfig.WriteTimeout,
		RequiredAcks: pconfig.RequiredACK,
		MaxAttempts:  pconfig.MaxAttempts,
	}

	updateCfg, failedCfg := base, base
	updateCfg.Topic = kconfig.UpadateLinksTopic
	failedCfg.Topic = kconfig.FailedLinksTopic

	return notifier{
		updateWriter: kafka.NewWriter(updateCfg),
		failedWriter: kafka.NewWriter(failedCfg),
		updateCodec:  mustLoadCodecFromFile("schemas/avro/link_update_event.avsc"),
		failedCodec:  mustLoadCodecFromFile("schemas/avro/failed_links_event.avsc"),
	}
}

func mustLoadCodecFromFile(path string) *goavro.Codec {
	schema, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("kafka-notifier: read avro schema %q: %w", path, err))
	}
	codec, err := goavro.NewCodec(string(schema))
	if err != nil {
		panic(fmt.Errorf("kafka-notifier: build avro codec from %q: %w", path, err))
	}
	return codec
}

func (p *notifier) Notify(ctx context.Context, chatID int64, link domain.Link, description string) error {
	var descriptionValue any = nil
	if description != "" {
		descriptionValue = map[string]any{"string": description}
	}

	native := map[string]any{
		"eventId":     uuid.NewString(),
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"url":         link.URL,
		"description": descriptionValue,
	}
	value, err := p.updateCodec.BinaryFromNative(nil, native)
	if err != nil {
		return fmt.Errorf("kafka-notifier: encode update avro payload: %w", err)
	}

	err = p.updateWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(strconv.FormatInt(chatID, 10)),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("kafka-notifier: write update kafka message: %w", err)
	}
	return nil
}

func (p *notifier) NotifyFailedLinks(ctx context.Context, chatID int64, links []string) error {
	if len(links) == 0 {
		return nil
	}
	description := "Не удалось обработать ссылки:\n" + strings.Join(links, "\n")

	native := map[string]any{
		"eventId":     uuid.NewString(),
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"description": description,
	}
	value, err := p.failedCodec.BinaryFromNative(nil, native)
	if err != nil {
		return fmt.Errorf("kafka-notifier: encode failed avro payload: %w", err)
	}

	err = p.failedWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(strconv.FormatInt(chatID, 10)),
		Value: value,
	})
	if err != nil {
		return fmt.Errorf("kafka-notifier: write failed kafka message: %w", err)
	}
	return nil
}

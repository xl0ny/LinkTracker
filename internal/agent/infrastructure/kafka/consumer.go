package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
)

type RawUpdateHandler interface {
	Handle(ctx context.Context, raw domain.RawUpdate) error
}

type Consumer struct {
	reader    *kafkago.Reader
	dlqWriter *kafkago.Writer

	sr *registry.Client

	handler    RawUpdateHandler
	maxRetries int
	retryDelay time.Duration
}

func NewConsumer(kconfig config.Kafka, cconfig config.KafkaConsumer, handler RawUpdateHandler) (*Consumer, error) {
	if kconfig.SchemaRegistryURL == "" {
		return nil, errors.New("agent-consumer: schema_registry_url required")
	}
	if kconfig.RawUpdatesTopic == "" {
		return nil, errors.New("agent-consumer: raw_updates_topic required")
	}
	if handler == nil {
		return nil, errors.New("agent-consumer: handler required")
	}
	startOffset := kafkago.LastOffset
	if cconfig.StartOffset == "earliest" {
		startOffset = kafkago.FirstOffset
	}
	dialer := &kafkago.Dialer{ClientID: cconfig.ConsumerClient}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        kconfig.Brokers,
		Topic:          kconfig.RawUpdatesTopic,
		MaxWait:        cconfig.ReadTimeout,
		GroupID:        cconfig.ConsumerGroup,
		StartOffset:    startOffset,
		CommitInterval: cconfig.CommitInterval,
		Dialer:         dialer,
	})

	var dlqWriter *kafkago.Writer
	if kconfig.DLQTopic != "" {
		dlqWriter = kafkago.NewWriter(kafkago.WriterConfig{
			Brokers:      kconfig.Brokers,
			Topic:        kconfig.DLQTopic,
			RequiredAcks: int(kafkago.RequireAll),
			Dialer:       dialer,
		})
	}

	return &Consumer{
		reader:     reader,
		dlqWriter:  dlqWriter,
		sr:         registry.NewClient(kconfig.SchemaRegistryURL),
		handler:    handler,
		maxRetries: cconfig.ProcessRetries,
		retryDelay: cconfig.RetryDelay,
	}, nil
}

func (c *Consumer) Close() error {
	var errs []error
	if c.reader != nil {
		if err := c.reader.Close(); err != nil {
			errs = append(errs, wrapErr("close reader", err))
		}
	}
	if c.dlqWriter != nil {
		if err := c.dlqWriter.Close(); err != nil {
			errs = append(errs, wrapErr("close dlq writer", err))
		}
	}
	return errors.Join(errs...)
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return wrapErr("fetch message", err)
		}

		processErr := c.handleMessage(ctx, msg)
		if processErr != nil {
			if dlqErr := c.writeToDLQ(ctx, msg, processErr); dlqErr != nil {
				return dlqErr
			}
		}

		if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
			return wrapErr("commit message", commitErr)
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	raw, err := c.decodeRaw(ctx, msg)
	if err != nil {
		return err
	}
	return c.retryBusiness(ctx, func(ctx context.Context) error {
		return wrapErr("handle raw update", c.handler.Handle(ctx, raw))
	})
}

func (c *Consumer) retryBusiness(ctx context.Context, fn func(context.Context) error) error {
	if c.maxRetries < 1 {
		return fn(ctx)
	}
	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		if err := fn(ctx); err != nil {
			lastErr = err
			if attempt == c.maxRetries {
				break
			}
			select {
			case <-ctx.Done():
				return wrapErr("retry cancelled", ctx.Err())
			case <-time.After(c.retryDelay):
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (c *Consumer) decodeRaw(ctx context.Context, msg kafkago.Message) (domain.RawUpdate, error) {
	schemaID, datum, err := registry.DecodeConfluent(msg.Value)
	if err != nil {
		return domain.RawUpdate{}, wrapErr("decode confluent wire", err)
	}
	codec, err := c.sr.CodecForID(ctx, schemaID)
	if err != nil {
		return domain.RawUpdate{}, wrapErr("schema registry codec", err)
	}
	native, _, err := codec.NativeFromBinary(datum)
	if err != nil {
		return domain.RawUpdate{}, wrapErr("decode raw avro", err)
	}
	record, ok := native.(map[string]any)
	if !ok {
		return domain.RawUpdate{}, wrapErr("decode raw avro", fmt.Errorf("unexpected native type %T", native))
	}

	description := extractString(record["description"])
	if description == "" {
		return domain.RawUpdate{}, wrapErr("validate raw payload", errors.New("empty description"))
	}
	url := extractString(record["url"])
	if url == "" {
		return domain.RawUpdate{}, wrapErr("validate raw payload", errors.New("empty url"))
	}

	return domain.RawUpdate{
		EventID:     extractString(record["eventId"]),
		OccurredAt:  extractTime(record["occurredAt"]),
		URL:         url,
		Description: description,
		Author:      extractString(record["author"]),
		TgChatIDs:   extractInt64Slice(record["tgChatIds"]),
	}, nil
}

func (c *Consumer) writeToDLQ(ctx context.Context, msg kafkago.Message, processErr error) error {
	slog.Warn("agent: routing message to DLQ",
		slog.String("topic", msg.Topic),
		slog.Int64("offset", msg.Offset),
		slog.String("error", processErr.Error()),
	)
	if c.dlqWriter == nil {
		return wrapErr("write dlq message", fmt.Errorf("dlq writer is nil, original error: %w", processErr))
	}

	dlqPayload := map[string]any{
		"sourceTopic": msg.Topic,
		"partition":   msg.Partition,
		"offset":      msg.Offset,
		"keyBase64":   base64.StdEncoding.EncodeToString(msg.Key),
		"valueBase64": base64.StdEncoding.EncodeToString(msg.Value),
		"error":       processErr.Error(),
		"createdAt":   time.Now().UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(dlqPayload)
	if err != nil {
		return wrapErr("marshal dlq payload", err)
	}

	if err = c.dlqWriter.WriteMessages(ctx, kafkago.Message{
		Key:   msg.Key,
		Value: payload,
	}); err != nil {
		return wrapErr("write dlq message", err)
	}
	return nil
}

func wrapErr(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("agent-consumer: %s: %w", operation, err)
}

func extractString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func extractTime(v any) time.Time {
	switch x := v.(type) {
	case time.Time:
		return x.UTC()
	case int64:
		return time.UnixMilli(x).UTC()
	}
	return time.Time{}
}

func extractInt64Slice(v any) []int64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]int64, 0, len(arr))
	for _, item := range arr {
		switch x := item.(type) {
		case int64:
			out = append(out, x)
		case int32:
			out = append(out, int64(x))
		case int:
			out = append(out, int64(x))
		case float64:
			out = append(out, int64(x))
		}
	}
	return out
}

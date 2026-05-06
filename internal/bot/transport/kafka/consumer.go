package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/avro/registry"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
)

type MessageSender interface {
	SendMessage(chatID int64, message string) error
}

type IdempotencyStore interface {
	Acquire(ctx context.Context, eventID string) (bool, error)
}

type Consumer struct {
	updateReader *kafkago.Reader
	failedReader *kafkago.Reader
	dlqWriter    *kafkago.Writer

	sr *registry.Client

	sender     MessageSender
	idempotent IdempotencyStore
	maxRetries int
	retryDelay time.Duration
}

const topicReaderGoroutines = 2

func NewConsumer(kconfig config.Kafka, cconfig config.KafkaConsumer, sender MessageSender, idem IdempotencyStore) (*Consumer, error) {
	if kconfig.SchemaRegistryURL == "" {
		return nil, errors.New("kafka-consumer: schema_registry_url required")
	}
	startOffset := kafkago.LastOffset
	if cconfig.StartOffset == "earliest" {
		startOffset = kafkago.FirstOffset
	}

	dialer := &kafkago.Dialer{ClientID: cconfig.ConsumerClient}

	base := kafkago.ReaderConfig{
		Brokers:        kconfig.Brokers,
		MaxWait:        cconfig.ReadTimeout,
		Topic:          kconfig.UpadateLinksTopic,
		GroupID:        cconfig.ConsumerGroup,
		StartOffset:    startOffset,
		CommitInterval: cconfig.CommitInterval,
		Dialer:         dialer,
	}

	updateCfg, failedCfg := base, base
	updateCfg.Topic, failedCfg.Topic = kconfig.UpadateLinksTopic, kconfig.FailedLinksTopic

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
		updateReader: kafkago.NewReader(updateCfg),
		failedReader: kafkago.NewReader(failedCfg),
		dlqWriter:    dlqWriter,
		sr:           registry.NewClient(kconfig.SchemaRegistryURL),
		sender:       sender,
		idempotent:   idem,
		maxRetries:   cconfig.ProcessRetries,
		retryDelay:   cconfig.RetryDelay,
	}, nil
}

func (c *Consumer) Close() error {
	var errs []error
	if c.updateReader != nil {
		if err := c.updateReader.Close(); err != nil {
			errs = append(errs, wrapConsumerError("close update reader", err))
		}
	}
	if c.failedReader != nil {
		if err := c.failedReader.Close(); err != nil {
			errs = append(errs, wrapConsumerError("close failed reader", err))
		}
	}
	if c.dlqWriter != nil {
		if err := c.dlqWriter.Close(); err != nil {
			errs = append(errs, wrapConsumerError("close dlq writer", err))
		}
	}
	return errors.Join(errs...)
}

func (c *Consumer) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, topicReaderGoroutines)

	wg.Add(topicReaderGoroutines)
	go func() {
		defer wg.Done()
		errCh <- c.consumeTopic(ctx, c.updateReader, c.decodeUpdate, c.deliverUpdate)
	}()
	go func() {
		defer wg.Done()
		errCh <- c.consumeTopic(ctx, c.failedReader, c.decodeFailed, c.deliverFailed)
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

type decodeFunc func(ctx context.Context, msg kafkago.Message) (chatID int64, record map[string]any, err error)
type deliverFunc func(ctx context.Context, chatID int64, record map[string]any) error

// consumeTopic: decode/validation errors go to DLQ once (no retries). Business errors (idempotency, SendMessage) are retried.
func (c *Consumer) consumeTopic(ctx context.Context, reader *kafkago.Reader, decode decodeFunc, deliver deliverFunc) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return wrapConsumerError("fetch message", err)
		}

		var processErr error
		chatID, record, decErr := decode(ctx, msg)
		if decErr != nil {
			processErr = decErr
		} else {
			processErr = c.retryBusiness(ctx, func(ctx context.Context) error {
				return deliver(ctx, chatID, record)
			})
		}
		if processErr != nil {
			if dlqErr := c.writeToDLQ(ctx, msg, processErr); dlqErr != nil {
				return dlqErr
			}
		}

		if commitErr := reader.CommitMessages(ctx, msg); commitErr != nil {
			return wrapConsumerError("commit message", commitErr)
		}
	}
}

func (c *Consumer) retryBusiness(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		if err := fn(ctx); err != nil {
			lastErr = err
			if attempt == c.maxRetries {
				break
			}
			select {
			case <-ctx.Done():
				return wrapConsumerError("retry cancelled", ctx.Err())
			case <-time.After(c.retryDelay):
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (c *Consumer) decodeMessageRecord(ctx context.Context, msg kafkago.Message, parseKeyOp, nativeStage string) (int64, map[string]any, error) {
	chatID, err := parseChatID(msg.Key)
	if err != nil {
		return 0, nil, wrapConsumerError(parseKeyOp, err)
	}

	schemaID, datum, err := registry.DecodeConfluent(msg.Value)
	if err != nil {
		return 0, nil, wrapConsumerError("decode confluent wire", err)
	}

	codec, err := c.sr.CodecForID(ctx, schemaID)
	if err != nil {
		return 0, nil, wrapConsumerError("schema registry codec", err)
	}

	native, _, err := codec.NativeFromBinary(datum)
	if err != nil {
		return 0, nil, wrapConsumerError(nativeStage, err)
	}

	record, ok := native.(map[string]any)
	if !ok {
		return 0, nil, wrapConsumerError(nativeStage, fmt.Errorf("unexpected native type %T", native))
	}

	return chatID, record, nil
}

func (c *Consumer) decodeUpdate(ctx context.Context, msg kafkago.Message) (int64, map[string]any, error) {
	chatID, record, err := c.decodeMessageRecord(ctx, msg, "parse update key", "decode update avro")
	if err != nil {
		return 0, nil, err
	}

	description := extractOptionalString(record["description"])
	if description == "" {
		return 0, nil, wrapConsumerError("validate update payload", errors.New("empty description"))
	}

	return chatID, record, nil
}

func (c *Consumer) decodeFailed(ctx context.Context, msg kafkago.Message) (int64, map[string]any, error) {
	chatID, record, err := c.decodeMessageRecord(ctx, msg, "parse failed key", "decode failed avro")
	if err != nil {
		return 0, nil, err
	}

	description := extractString(record["description"])
	if description == "" {
		return 0, nil, wrapConsumerError("validate failed payload", errors.New("empty description"))
	}

	return chatID, record, nil
}

func (c *Consumer) deliverUpdate(ctx context.Context, chatID int64, record map[string]any) error {
	first, err := c.acquireEvent(ctx, record)
	if err != nil {
		return wrapConsumerError("idempotency check", err)
	}
	if !first {
		return nil
	}
	description := extractOptionalString(record["description"])
	if err = c.sender.SendMessage(chatID, description); err != nil {
		return wrapConsumerError("send update message", err)
	}
	return nil
}

func (c *Consumer) deliverFailed(ctx context.Context, chatID int64, record map[string]any) error {
	first, err := c.acquireEvent(ctx, record)
	if err != nil {
		return wrapConsumerError("idempotency check", err)
	}
	if !first {
		return nil
	}
	description := extractString(record["description"])
	if err = c.sender.SendMessage(chatID, description); err != nil {
		return wrapConsumerError("send failed message", err)
	}
	return nil
}

func (c *Consumer) acquireEvent(ctx context.Context, record map[string]any) (bool, error) {
	if c.idempotent == nil {
		return true, nil
	}
	first, err := c.idempotent.Acquire(ctx, extractString(record["eventId"]))
	if err != nil {
		return false, fmt.Errorf("idempotent acquire: %w", err)
	}
	return first, nil
}

func (c *Consumer) writeToDLQ(ctx context.Context, msg kafkago.Message, processErr error) error {
	if c.dlqWriter == nil {
		return wrapConsumerError("write dlq message", fmt.Errorf("dlq writer is nil, original error: %w", processErr))
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
		return wrapConsumerError("marshal dlq payload", err)
	}

	err = c.dlqWriter.WriteMessages(ctx, kafkago.Message{
		Key:   msg.Key,
		Value: payload,
	})
	if err != nil {
		return wrapConsumerError("write dlq message", err)
	}
	return nil
}

func wrapConsumerError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("kafka-consumer: %s: %w", operation, err)
}

func parseChatID(raw []byte) (int64, error) {
	if len(raw) == 0 {
		return 0, errors.New("empty kafka key")
	}
	chatID, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse chat id from key: %w", err)
	}
	return chatID, nil
}

func extractOptionalString(v any) string {
	if v == nil {
		return ""
	}
	union, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	unwrapped, ok := union["string"]
	if !ok {
		return ""
	}
	s, ok := unwrapped.(string)
	if !ok {
		return ""
	}
	return s
}

func extractString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

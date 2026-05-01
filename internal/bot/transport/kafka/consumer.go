package kafka

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/linkedin/goavro/v2"
	kafkago "github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
)

const (
	updateSchemaPath = "schemas/avro/link_update_event.avsc"
	failedSchemaPath = "schemas/avro/failed_links_event.avsc"
)

type MessageSender interface {
	SendMessage(chatID int64, message string) error
}

type Consumer struct {
	updateReader *kafkago.Reader
	failedReader *kafkago.Reader
	dlqWriter    *kafkago.Writer

	updateCodec *goavro.Codec
	failedCodec *goavro.Codec

	sender     MessageSender
	maxRetries int
	retryDelay time.Duration
}

func NewConsumer(kconfig config.Kafka, cconfig config.KafkaConsumer, sender MessageSender) *Consumer {
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
		updateCodec:  mustLoadCodecFromFile(updateSchemaPath),
		failedCodec:  mustLoadCodecFromFile(failedSchemaPath),
		sender:       sender,
		maxRetries:   cconfig.ProcessRetries,
		retryDelay:   cconfig.RetryDelay,
	}
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
	errCh := make(chan error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errCh <- c.consumeLoop(ctx, c.updateReader, c.processUpdate)
	}()
	go func() {
		defer wg.Done()
		errCh <- c.consumeLoop(ctx, c.failedReader, c.processFailed)
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

func (c *Consumer) consumeLoop(
	ctx context.Context,
	reader *kafkago.Reader,
	handler func(context.Context, kafkago.Message) error,
) error {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return wrapConsumerError("fetch message", err)
		}

		processErr := c.processWithRetry(ctx, msg, handler)
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

func (c *Consumer) processWithRetry(
	ctx context.Context,
	msg kafkago.Message,
	handler func(context.Context, kafkago.Message) error,
) error {
	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		if err := handler(ctx, msg); err != nil {
			lastErr = err
			if attempt == c.maxRetries {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryDelay):
			}
			continue
		}
		return nil
	}
	return lastErr
}

func (c *Consumer) processUpdate(ctx context.Context, msg kafkago.Message) error {
	chatID, err := parseChatID(msg.Key)
	if err != nil {
		return wrapConsumerError("parse update key", err)
	}

	native, _, err := c.updateCodec.NativeFromBinary(msg.Value)
	if err != nil {
		return wrapConsumerError("decode update avro", err)
	}

	record, ok := native.(map[string]any)
	if !ok {
		return wrapConsumerError("decode update avro", fmt.Errorf("unexpected native type %T", native))
	}

	description := extractOptionalString(record["description"])
	if description == "" {
		return wrapConsumerError("validate update payload", errors.New("empty description"))
	}

	if err = c.sender.SendMessage(chatID, description); err != nil {
		return wrapConsumerError("send update message", err)
	}

	return nil
}

func (c *Consumer) processFailed(ctx context.Context, msg kafkago.Message) error {
	chatID, err := parseChatID(msg.Key)
	if err != nil {
		return wrapConsumerError("parse failed key", err)
	}

	native, _, err := c.failedCodec.NativeFromBinary(msg.Value)
	if err != nil {
		return wrapConsumerError("decode failed avro", err)
	}

	record, ok := native.(map[string]any)
	if !ok {
		return wrapConsumerError("decode failed avro", fmt.Errorf("unexpected native type %T", native))
	}

	description := extractString(record["description"])
	if description == "" {
		return wrapConsumerError("validate failed payload", errors.New("empty description"))
	}

	if err = c.sender.SendMessage(chatID, description); err != nil {
		return wrapConsumerError("send failed message", err)
	}

	return nil
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

func mustLoadCodecFromFile(path string) *goavro.Codec {
	schema, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("kafka-consumer: read avro schema %q: %w", path, err))
	}
	codec, err := goavro.NewCodec(string(schema))
	if err != nil {
		panic(fmt.Errorf("kafka-consumer: build avro codec from %q: %w", path, err))
	}
	return codec
}

func parseChatID(raw []byte) (int64, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("empty kafka key")
	}
	chatID, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		return 0, err
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

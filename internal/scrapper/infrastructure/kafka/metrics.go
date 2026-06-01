package kafka

import (
	"context"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/metrics"
)

func writeMessages(ctx context.Context, w *kafkago.Writer, m *metrics.Scrapper, topic string, msgs ...kafkago.Message) error {
	start := time.Now()
	err := w.WriteMessages(ctx, msgs...)
	if m != nil {
		m.ObserveKafkaWrite(topic, start)
	}
	if err != nil {
		return fmt.Errorf("kafka write messages: %w", err)
	}
	return nil
}

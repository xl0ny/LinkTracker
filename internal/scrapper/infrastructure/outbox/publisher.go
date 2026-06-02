package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/prometrics"
)

// maxBackoffShift caps exponential backoff exponent to avoid overflowing time.Duration when shifting.
const maxBackoffShift = 30

type PublisherRepository interface {
	FetchAndLockOutbox(ctx context.Context, batchSize int, lockFor time.Duration) ([]domain.OutboxEvent, error)
	MarkOutboxSent(ctx context.Context, id int64) error
	IncrementOutboxAttempts(ctx context.Context, id int64, lastErr string, nextAvailableAt time.Time, maxAttempts int) error
}

type Publisher struct {
	repo   PublisherRepository
	writer *kafkago.Writer
	m      *prometrics.Scrapper

	pollInterval time.Duration
	batchSize    int
	lockFor      time.Duration
	maxAttempts  int
	baseBackoff  time.Duration
	maxBackoff   time.Duration
}

func NewPublisher(repo PublisherRepository, kcfg config.Kafka, pcfg config.KafkaProducer, ocfg config.KafkaOutbox, m *prometrics.Scrapper) *Publisher {
	dialer := &kafkago.Dialer{ClientID: pcfg.ProducerClient}
	writer := kafkago.NewWriter(kafkago.WriterConfig{
		Brokers:      kcfg.Brokers,
		WriteTimeout: pcfg.WriteTimeout,
		RequiredAcks: pcfg.RequiredACK,
		MaxAttempts:  pcfg.MaxAttempts,
		Dialer:       dialer,
	})
	return &Publisher{
		repo:         repo,
		writer:       writer,
		m:            m,
		pollInterval: ocfg.PollInterval,
		batchSize:    ocfg.BatchSize,
		lockFor:      ocfg.LockFor,
		maxAttempts:  ocfg.MaxAttempts,
		baseBackoff:  ocfg.BaseBackoff,
		maxBackoff:   ocfg.MaxBackoff,
	}
}

func (p *Publisher) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("outbox-publisher: close writer: %w", err)
	}
	return nil
}

func (p *Publisher) Run(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		p.processBatch(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *Publisher) processBatch(ctx context.Context) {
	events, err := p.repo.FetchAndLockOutbox(ctx, p.batchSize, p.lockFor)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("outbox-publisher: fetch pending", slog.String("error", err.Error()))
		}
		return
	}
	for _, e := range events {
		p.publishOne(ctx, e)
	}
}

func (p *Publisher) publishOne(ctx context.Context, e domain.OutboxEvent) {
	start := time.Now()
	wErr := p.writer.WriteMessages(ctx, kafkago.Message{
		Topic: e.Topic,
		Key:   e.Key,
		Value: e.Payload,
	})
	if p.m != nil {
		p.m.ObserveKafkaWrite(e.Topic, start)
	}
	if wErr == nil {
		if mErr := p.repo.MarkOutboxSent(ctx, e.ID); mErr != nil {
			slog.Error("outbox-publisher: mark sent",
				slog.Int64("id", e.ID),
				slog.String("error", mErr.Error()),
			)
		}
		return
	}

	if errors.Is(wErr, context.Canceled) {
		return
	}

	nextAttempt := e.Attempts + 1
	nextAvailableAt := time.Now().Add(p.backoff(nextAttempt))
	if iErr := p.repo.IncrementOutboxAttempts(ctx, e.ID, wErr.Error(), nextAvailableAt, p.maxAttempts); iErr != nil {
		slog.Error("outbox-publisher: increment attempts",
			slog.Int64("id", e.ID),
			slog.String("error", iErr.Error()),
		)
		return
	}

	logArgs := []any{
		slog.Int64("id", e.ID),
		slog.String("event_id", e.EventID),
		slog.String("topic", e.Topic),
		slog.Int("attempt", nextAttempt),
		slog.String("error", wErr.Error()),
	}
	if nextAttempt >= p.maxAttempts {
		slog.Error("outbox-publisher: max attempts reached, marked failed", logArgs...)
	} else {
		slog.Warn("outbox-publisher: write failed, will retry", logArgs...)
	}
}

func (p *Publisher) backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := attempt - 1
	if shift > maxBackoffShift {
		shift = maxBackoffShift
	}
	d := p.baseBackoff << shift
	if d <= 0 || d > p.maxBackoff {
		return p.maxBackoff
	}
	return d
}

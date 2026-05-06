//go:build integration

package kafka

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	tcKafka "github.com/testcontainers/testcontainers-go/modules/kafka"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/avro/registry"
	commoncfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/common/config"
)

func TestKafkaIntegration_produceConsume(t *testing.T) {
	ctx := context.Background()

	kafkaC, err := tcKafka.Run(ctx, "confluentinc/confluent-local:7.6.1",
		tcKafka.WithClusterID("linktracker-integration"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tc.TerminateContainer(kafkaC)
	})

	brokers, err := kafkaC.Brokers(ctx)
	require.NoError(t, err)

	_, testFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "..")

	updateSchema, err := os.ReadFile(filepath.Join(repoRoot, "schemas", "avro", "link_update_event.avsc"))
	require.NoError(t, err)
	failedSchema, err := os.ReadFile(filepath.Join(repoRoot, "schemas", "avro", "failed_links_event.avsc"))
	require.NoError(t, err)

	var nextID int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && len(r.URL.Path) > len("/subjects/"):
			nextID++
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%d}`, nextID)))
		case r.Method == http.MethodGet && len(r.URL.Path) > len("/schemas/ids/"):
			id := 0
			if _, err := fmt.Sscanf(r.URL.Path, "/schemas/ids/%d", &id); err != nil || id < 1 || id > 2 {
				http.NotFound(w, r)
				return
			}
			var sch string
			if id == 1 {
				sch = string(updateSchema)
			} else {
				sch = string(failedSchema)
			}
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			fmt.Fprintf(w, `{"schema":%q}`, sch)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	encoder, err := registry.NewEncoder(ctx, srv.URL,
		"link-update-integration", "failed-integration",
		filepath.Join(repoRoot, "schemas", "avro", "link_update_event.avsc"),
		filepath.Join(repoRoot, "schemas", "avro", "failed_links_event.avsc"),
	)
	require.NoError(t, err)

	topicUpdates := fmt.Sprintf("link-updates-it-%d", time.Now().UnixNano())
	topicFailed := fmt.Sprintf("failed-links-it-%d", time.Now().UnixNano())
	topicDLQ := fmt.Sprintf("link-updates-dlq-it-%d", time.Now().UnixNano())

	dialConn, err := kafkago.DialContext(ctx, "tcp", brokers[0])
	require.NoError(t, err)

	err = dialConn.CreateTopics(
		kafkago.TopicConfig{
			Topic:             topicUpdates,
			NumPartitions:     2,
			ReplicationFactor: 1,
		},
		kafkago.TopicConfig{
			Topic:             topicFailed,
			NumPartitions:     2,
			ReplicationFactor: 1,
		},
		kafkago.TopicConfig{
			Topic:             topicDLQ,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	)
	require.NoError(t, dialConn.Close())

	payload, err := encoder.EncodeUpdate(map[string]any{
		"eventId":     "integration-event-id",
		"occurredAt":  time.Now().UnixMilli(),
		"url":         "https://example.com/integration",
		"description": map[string]any{"string": "integration ok"},
	})
	require.NoError(t, err)

	w := &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Topic:                  topicUpdates,
		AllowAutoTopicCreation: false,
		BatchTimeout:           time.Millisecond,
		WriteTimeout:           30 * time.Second,
		ReadTimeout:            30 * time.Second,
	}
	defer func() { _ = w.Close() }()

	err = w.WriteMessages(ctx,
		kafkago.Message{
			Key:   []byte("424242"),
			Value: payload,
		})
	require.NoError(t, err)

	failPayload, err := encoder.EncodeFailed(map[string]any{
		"eventId":     "bootstrap-failed-msg",
		"occurredAt":  int64(1),
		"description": "__bootstrap_failed__",
	})
	require.NoError(t, err)
	wFail := &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Topic:                  topicFailed,
		AllowAutoTopicCreation: false,
		BatchTimeout:           time.Millisecond,
		ReadTimeout:            30 * time.Second,
		WriteTimeout:           30 * time.Second,
	}
	err = wFail.WriteMessages(ctx, kafkago.Message{Key: []byte("1"), Value: failPayload})
	require.NoError(t, err)
	defer func() { _ = wFail.Close() }()

	sent := make(chan string, 16)
	sender := &captureSender{ch: sent}

	ccfg := commoncfg.KafkaConsumer{
		ConsumerGroup:  fmt.Sprintf("bot-it-%d", time.Now().UnixNano()),
		ConsumerClient: "bot-consumer-it",
		ReadTimeout:    3 * time.Second,
		CommitInterval: 0,
		StartOffset:    "earliest",
		ProcessRetries: 5,
		RetryDelay:     100 * time.Millisecond,
	}

	kcfg := commoncfg.Kafka{
		Enabled:           true,
		Brokers:           brokers,
		UpadateLinksTopic: topicUpdates,
		FailedLinksTopic:  topicFailed,
		DLQTopic:          topicDLQ,
		SchemaRegistryURL: srv.URL,
		UpdateSubject:     "u",
		FailedSubject:     "f",
	}

	co, err := NewConsumer(kcfg, ccfg, sender, nil)
	require.NoError(t, err)

	rctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- co.Run(rctx) }()

	found := map[string]struct{}{}
	timeout := time.NewTimer(110 * time.Second)
	defer timeout.Stop()
	for len(found) < 2 {
		select {
		case msg := <-sent:
			found[msg] = struct{}{}
		case err := <-errCh:
			t.Fatalf("consumer stopped early: err=%v found=%v", err, found)
		case <-timeout.C:
			t.Fatalf("timeout; found=%v", found)
		}
	}
	require.Contains(t, found, "integration ok")
	require.Contains(t, found, "__bootstrap_failed__")
	cancel()

	select {
	case err := <-errCh:
		_ = err
	case <-time.After(30 * time.Second):
		t.Fatal("consumer goroutine did not exit")
	}
	require.NoError(t, co.Close())
}

type captureSender struct {
	ch chan string
}

func (c *captureSender) SendMessage(chatID int64, message string) error {
	c.ch <- message
	_ = strconv.FormatInt(chatID, 10)
	return nil
}

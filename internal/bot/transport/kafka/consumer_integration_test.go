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
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	tcKafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	"go.uber.org/mock/gomock"

	kafkamocks "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/transport/kafka/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
	commoncfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
)

func TestKafkaIntegration_produceConsume(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "kafka integration produce consume"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

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
			assert.True(t, ok)
			repoRoot := filepath.Join(filepath.Dir(testFile), "..", "..", "..", "..")

			processedPath := filepath.Join(repoRoot, "schemas", "avro", "link_processed_update_event.avsc")
			failedPath := filepath.Join(repoRoot, "schemas", "avro", "failed_links_event.avsc")
			processedSchema, err := os.ReadFile(processedPath)
			require.NoError(t, err)
			failedSchema, err := os.ReadFile(failedPath)
			require.NoError(t, err)

			var nextID int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && len(r.URL.Path) > len("/subjects/"):
					nextID++
					_, _ = fmt.Fprintf(w, `{"id":%d}`, nextID)
				case r.Method == http.MethodGet && len(r.URL.Path) > len("/schemas/ids/"):
					id := 0
					if _, sErr := fmt.Sscanf(r.URL.Path, "/schemas/ids/%d", &id); sErr != nil || id < 1 || id > 2 {
						http.NotFound(w, r)
						return
					}
					var sch string
					if id == 1 {
						sch = string(processedSchema)
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

			processedEnc, err := registry.NewSingleEncoder(ctx, srv.URL, "processed-integration", processedPath)
			require.NoError(t, err)
			failedEnc, err := registry.NewSingleEncoder(ctx, srv.URL, "failed-integration", failedPath)
			require.NoError(t, err)

			topicProcessed := fmt.Sprintf("link.processed-updates-it-%d", time.Now().UnixNano())
			topicFailed := fmt.Sprintf("failed-links-it-%d", time.Now().UnixNano())
			topicDLQ := fmt.Sprintf("link.processed-updates-dlq-it-%d", time.Now().UnixNano())

			dialConn, err := kafkago.DialContext(ctx, "tcp", brokers[0])
			require.NoError(t, err)

			require.NoError(t, dialConn.CreateTopics(
				kafkago.TopicConfig{
					Topic:             topicProcessed,
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
			))
			require.NoError(t, dialConn.Close())

			payload, err := processedEnc.Encode(map[string]any{
				"eventId":     "integration-event-id",
				"occurredAt":  time.Now().UnixMilli(),
				"url":         "https://example.com/integration",
				"description": "integration ok",
				"tgChatIds":   []any{int64(424242)},
				"priority":    "HIGH",
			})
			require.NoError(t, err)

			w := &kafkago.Writer{
				Addr:                   kafkago.TCP(brokers...),
				Topic:                  topicProcessed,
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

			failPayload, err := failedEnc.Encode(map[string]any{
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
			ctrl := gomock.NewController(t)
			sender := kafkamocks.NewMockMessageSender(ctrl)
			sender.EXPECT().
				SendMessage(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ int64, message string) error {
					sent <- message
					return nil
				}).
				AnyTimes()

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
				Enabled:                true,
				Brokers:                brokers,
				ProcessedUpdatesTopic:  topicProcessed,
				FailedLinksTopic:       topicFailed,
				DLQTopic:               topicDLQ,
				SchemaRegistryURL:      srv.URL,
				ProcessedUpdateSubject: "p",
				FailedSubject:          "f",
			}

			co, err := NewConsumer(kcfg, ccfg, sender, nil, nil)
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
				case rerr := <-errCh:
					t.Fatalf("consumer stopped early: err=%v found=%v", rerr, found)
				case <-timeout.C:
					t.Fatalf("timeout; found=%v", found)
				}
			}
			assert.Contains(t, found, "integration ok")
			assert.Contains(t, found, "__bootstrap_failed__")
			cancel()

			select {
			case <-errCh:
			case <-time.After(30 * time.Second):
				t.Fatal("consumer goroutine did not exit")
			}
			require.NoError(t, co.Close())
		})
	}
}

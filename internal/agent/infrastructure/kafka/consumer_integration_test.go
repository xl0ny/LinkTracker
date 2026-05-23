//go:build integration

package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	tcKafka "github.com/testcontainers/testcontainers-go/modules/kafka"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/application"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/infrastructure/summarizer"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
	commoncfg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/config"
)

func TestAgentIntegration_RawProcessedRoundTrip(t *testing.T) {
	ctx := context.Background()

	kafkaC, err := tcKafka.Run(ctx, "confluentinc/confluent-local:7.6.1",
		tcKafka.WithClusterID("agent-integration"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tc.TerminateContainer(kafkaC)
	})

	brokers, err := kafkaC.Brokers(ctx)
	require.NoError(t, err)

	repoRoot := repoRootDir(t)
	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(repoRoot))
	t.Cleanup(func() { _ = os.Chdir(wd) })

	rawSchemaPath := filepath.Join(repoRoot, "schemas", "avro", "link_raw_update_event.avsc")
	processedSchemaPath := filepath.Join(repoRoot, "schemas", "avro", "link_processed_update_event.avsc")
	rawSchema, err := os.ReadFile(rawSchemaPath)
	require.NoError(t, err)
	processedSchema, err := os.ReadFile(processedSchemaPath)
	require.NoError(t, err)

	srv := newSchemaRegistryStub(map[int]string{1: string(rawSchema), 2: string(processedSchema)})
	defer srv.Close()

	rawTopic := fmt.Sprintf("link.raw-it-%d", time.Now().UnixNano())
	processedTopic := fmt.Sprintf("link.processed-it-%d", time.Now().UnixNano())
	dlqTopic := fmt.Sprintf("link.raw-dlq-it-%d", time.Now().UnixNano())

	createTopics(ctx, t, brokers, rawTopic, processedTopic, dlqTopic)

	rawEnc, err := registry.NewSingleEncoder(ctx, srv.URL, "raw-it", rawSchemaPath)
	require.NoError(t, err)

	rawWriter := &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Topic:                  rawTopic,
		AllowAutoTopicCreation: false,
		BatchTimeout:           time.Millisecond,
		WriteTimeout:           30 * time.Second,
		ReadTimeout:            30 * time.Second,
	}
	defer func() { _ = rawWriter.Close() }()

	validPayload, err := rawEnc.Encode(map[string]any{
		"eventId":     "evt-valid",
		"occurredAt":  time.Now().UTC().UnixMilli(),
		"url":         "https://example.com/integration",
		"description": "this is a valid update with enough length to pass min-length filter",
		"author":      "alice",
		"tgChatIds":   []any{int64(123), int64(456)},
	})
	require.NoError(t, err)

	require.NoError(t, rawWriter.WriteMessages(ctx,
		kafkago.Message{Key: []byte("evt-valid"), Value: validPayload},
		kafkago.Message{Key: []byte("poison"), Value: []byte{0xff, 0xff, 0xff, 0xff}},
	))

	filter := application.NewFilter(application.FilterConfig{MinLength: 10})
	processor := application.NewProcessor(filter, summarizer.NewStub(40), 40)

	kcfg := commoncfg.Kafka{
		Enabled:                true,
		Brokers:                brokers,
		RawUpdatesTopic:        rawTopic,
		ProcessedUpdatesTopic:  processedTopic,
		DLQTopic:               dlqTopic,
		FailedLinksTopic:       "unused",
		SchemaRegistryURL:      srv.URL,
		RawUpdateSubject:       "raw-sub",
		ProcessedUpdateSubject: "processed-sub",
	}
	pcfg := commoncfg.KafkaProducer{
		ProducerClient: "agent-it-producer",
		WriteTimeout:   5 * time.Second,
		RequiredACK:    int(kafkago.RequireOne),
		MaxAttempts:    3,
	}
	ccfg := commoncfg.KafkaConsumer{
		ConsumerGroup:  fmt.Sprintf("agent-it-%d", time.Now().UnixNano()),
		ConsumerClient: "agent-it-consumer",
		ReadTimeout:    3 * time.Second,
		CommitInterval: 0,
		StartOffset:    "earliest",
		ProcessRetries: 1,
		RetryDelay:     100 * time.Millisecond,
	}

	producer, err := NewProducer(ctx, kcfg, pcfg)
	require.NoError(t, err)
	defer func() { _ = producer.Close() }()

	consumer, err := NewConsumer(kcfg, ccfg, processor, producer)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- consumer.Run(rctx) }()

	processedReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		Topic:       processedTopic,
		GroupID:     fmt.Sprintf("processed-it-%d", time.Now().UnixNano()),
		StartOffset: kafkago.FirstOffset,
		MaxWait:     time.Second,
	})
	defer func() { _ = processedReader.Close() }()

	dlqReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		Topic:       dlqTopic,
		GroupID:     fmt.Sprintf("dlq-it-%d", time.Now().UnixNano()),
		StartOffset: kafkago.FirstOffset,
		MaxWait:     time.Second,
	})
	defer func() { _ = dlqReader.Close() }()

	processedMsg, err := readMessage(rctx, processedReader, 60*time.Second)
	require.NoError(t, err)
	processedClient := registry.NewClient(srv.URL)
	processedID, processedDatum, err := registry.DecodeConfluent(processedMsg.Value)
	require.NoError(t, err)
	codec, err := processedClient.CodecForID(rctx, processedID)
	require.NoError(t, err)
	native, _, err := codec.NativeFromBinary(processedDatum)
	require.NoError(t, err)
	rec, ok := native.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "evt-valid", rec["eventId"])
	desc, _ := rec["description"].(string)
	require.Contains(t, desc, "this is a valid update")
	require.Equal(t, "HIGH", rec["priority"])

	dlqMsg, err := readMessage(rctx, dlqReader, 60*time.Second)
	require.NoError(t, err)
	require.Contains(t, string(dlqMsg.Value), "decode confluent wire")

	cancel()
	select {
	case <-errCh:
	case <-time.After(30 * time.Second):
		t.Fatal("consumer did not stop")
	}
}

func readMessage(ctx context.Context, r *kafkago.Reader, timeout time.Duration) (kafkago.Message, error) {
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return r.ReadMessage(rctx)
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
}

func newSchemaRegistryStub(initial map[int]string) *httptest.Server {
	schemas := map[int]string{}
	for k, v := range initial {
		schemas[k] = v
	}
	nextID := 0
	for id := range schemas {
		if id > nextID {
			nextID = id
		}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && len(r.URL.Path) > len("/subjects/"):
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Schema string `json:"schema"`
			}
			_ = json.Unmarshal(body, &req)
			for id, s := range schemas {
				if s == req.Schema {
					fmt.Fprintf(w, `{"id":%d}`, id)
					return
				}
			}
			nextID++
			schemas[nextID] = req.Schema
			fmt.Fprintf(w, `{"id":%d}`, nextID)
		case r.Method == http.MethodGet && len(r.URL.Path) > len("/schemas/ids/"):
			id := 0
			if _, sErr := fmt.Sscanf(r.URL.Path, "/schemas/ids/%d", &id); sErr != nil || schemas[id] == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			fmt.Fprintf(w, `{"schema":%q}`, schemas[id])
		default:
			http.NotFound(w, r)
		}
	}))
}

func createTopics(ctx context.Context, t *testing.T, brokers []string, topics ...string) {
	t.Helper()
	conn, err := kafkago.DialContext(ctx, "tcp", brokers[0])
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	configs := make([]kafkago.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		configs = append(configs, kafkago.TopicConfig{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}
	require.NoError(t, conn.CreateTopics(configs...))
}

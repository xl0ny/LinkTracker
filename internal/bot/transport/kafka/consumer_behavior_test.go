package kafka

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/linkedin/goavro/v2"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	commonreg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
)

func TestRetryBusiness_stopsAfterMaxRetries(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "retry business stops after max retries"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			c := &Consumer{maxRetries: 3, retryDelay: time.Millisecond}
			var calls int
			err := c.retryBusiness(context.Background(), func(context.Context) error {
				calls++
				return errors.New("fail")
			})
			assert.Error(t, err)
			assert.Equal(t, 3, calls)
		})
	}
}

func TestDecodeUpdate_rejectsGarbageWire(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "decode update rejects garbage wire"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			c := &Consumer{sr: commonreg.NewClient("http://unused.example")}
			_, err := c.decodeUpdate(context.Background(), kafka.Message{Key: []byte("1"), Value: []byte{0xff}})
			assert.Error(t, err)
		})
	}
}

func TestDecodeUpdate_validConfluent_payload(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "decode update valid confluent payload"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, thisFile, _, ok := runtime.Caller(0)
			assert.True(t, ok)
			repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
			processedPath := filepath.Join(repoRoot, "schemas", "avro", "link_processed_update_event.avsc")
			processedSchema, err := os.ReadFile(processedPath)
			assert.NoError(t, err)

			srv := httptest.NewServer(schemaRegistryStub(map[int]string{7: string(processedSchema)}))
			defer srv.Close()

			ctx := context.Background()
			c := &Consumer{sr: commonreg.NewClient(srv.URL)}

			raw, err := codecBinaryFromFixture(t, repoRoot)
			assert.NoError(t, err)
			wire := commonreg.EncodeConfluent(7, raw)

			rec, err := c.decodeUpdate(ctx, kafka.Message{Key: []byte("999"), Value: wire})
			assert.NoError(t, err)
			assert.Equal(t, "hello", extractString(rec["description"]))
			assert.Equal(t, []int64{42}, extractInt64Slice(rec["tgChatIds"]))
		})
	}
}

func codecBinaryFromFixture(t *testing.T, repoRoot string) ([]byte, error) {
	t.Helper()
	path := filepath.Join(repoRoot, "schemas", "avro", "link_processed_update_event.avsc")
	schema, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	codec, err := goavro.NewCodec(string(schema))
	if err != nil {
		return nil, err
	}
	return codec.BinaryFromNative(nil, map[string]any{
		"eventId":     "evt-1",
		"occurredAt":  int64(1),
		"url":         "https://example.com",
		"description": "hello",
		"tgChatIds":   []any{int64(42)},
		"priority":    "HIGH",
	})
}

func schemaRegistryStub(schemas map[int]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && len(r.URL.Path) > len("/schemas/ids/") {
			id := 0
			if _, err := fmt.Sscanf(r.URL.Path, "/schemas/ids/%d", &id); err != nil || schemas[id] == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			fmt.Fprintf(w, `{"schema":%q}`, schemas[id])
			return
		}
		http.NotFound(w, r)
	}
}

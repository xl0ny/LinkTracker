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
	"github.com/stretchr/testify/require"
	commonreg "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/pkg/avro/registry"
)

func TestRetryBusiness_stopsAfterMaxRetries(t *testing.T) {
	c := &Consumer{maxRetries: 3, retryDelay: time.Millisecond}
	var calls int
	err := c.retryBusiness(context.Background(), func(context.Context) error {
		calls++
		return errors.New("fail")
	})
	require.Error(t, err)
	require.Equal(t, 3, calls)
}

func TestDecodeUpdate_rejectsGarbageWire(t *testing.T) {
	c := &Consumer{sr: commonreg.NewClient("http://unused.example")}
	_, _, err := c.decodeUpdate(context.Background(), kafka.Message{Key: []byte("1"), Value: []byte{0xff}})
	require.Error(t, err)
}

func TestDecodeUpdate_rejectsBadKeyNoRetrySemantics(t *testing.T) {
	// decode errors are categorized outside retryBusiness — this only checks decode returns quickly.
	c := &Consumer{sr: commonreg.NewClient("http://unused.example")}
	_, _, err := c.decodeUpdate(context.Background(), kafka.Message{Key: []byte("not-int"), Value: commonreg.EncodeConfluent(1, []byte{1, 2})})
	require.Error(t, err)
}

func TestDecodeUpdate_validConfluent_payload(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	updatePath := filepath.Join(repoRoot, "schemas", "avro", "link_update_event.avsc")
	updateSchema, err := os.ReadFile(updatePath)
	require.NoError(t, err)

	srv := httptest.NewServer(schemaRegistryStub(map[int]string{7: string(updateSchema)}))
	defer srv.Close()

	ctx := context.Background()
	c := &Consumer{sr: commonreg.NewClient(srv.URL)}

	raw, err := codecBinaryFromFixture(t, repoRoot)
	require.NoError(t, err)
	wire := commonreg.EncodeConfluent(7, raw)

	chID, rec, err := c.decodeUpdate(ctx, kafka.Message{Key: []byte("999"), Value: wire})
	require.NoError(t, err)
	require.Equal(t, int64(999), chID)
	require.Equal(t, "hello", extractOptionalString(rec["description"]))
}

func codecBinaryFromFixture(t *testing.T, repoRoot string) ([]byte, error) {
	t.Helper()
	path := filepath.Join(repoRoot, "schemas", "avro", "link_update_event.avsc")
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
		"description": map[string]any{"string": "hello"},
	})
}

func schemaRegistryStub(schemas map[int]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && len(r.URL.Path) > len("/schemas/ids/"):
			id := 0
			if _, err := fmt.Sscanf(r.URL.Path, "/schemas/ids/%d", &id); err != nil || schemas[id] == "" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			fmt.Fprintf(w, `{"schema":%q}`, schemas[id])
		default:
			http.NotFound(w, r)
		}
	}
}

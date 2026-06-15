package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_RegisterAndCodecForID(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "client register and codec for id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			schema := `{"type":"record","name":"X","fields":[{"name":"n","type":"int"}]}`

			var registeredID int

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/subjects/topic-value/versions":
					registeredID++
					_ = json.NewEncoder(w).Encode(map[string]int{"id": registeredID})
				case r.Method == http.MethodGet && r.URL.Path == "/schemas/ids/1":
					w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
					_ = json.NewEncoder(w).Encode(map[string]string{schemaRegistryJSONField: schema})
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			ctx := context.Background()
			c := NewClient(srv.URL)
			id, err := c.RegisterSchema(ctx, "topic-value", schema)
			assert.NoError(t, err)
			assert.Equal(t, int32(1), id)

			codec, err := c.CodecForID(ctx, 1)
			assert.NoError(t, err)
			payload, err := codec.BinaryFromNative(nil, map[string]any{"n": int32(7)})
			assert.NoError(t, err)
			native, _, err := codec.NativeFromBinary(payload)
			assert.NoError(t, err)
			rec := native.(map[string]any)
			assert.Equal(t, int32(7), rec["n"])
		})
	}
}

func TestNewSingleEncoder_integrationWithMockSR(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "new single encoder integration with mock sr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			_, thisFile, _, ok := runtime.Caller(0)
			assert.True(t, ok)
			root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
			schemaPath := filepath.Join(root, "schemas", "avro", "link_raw_update_event.avsc")
			_, err := os.Stat(schemaPath)
			assert.NoError(t, err)

			schema, err := os.ReadFile(schemaPath)
			assert.NoError(t, err)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/subjects/raw-value/versions":
					_ = json.NewEncoder(w).Encode(map[string]int{"id": 1})
				case r.Method == http.MethodGet && r.URL.Path == "/schemas/ids/1":
					w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
					_ = json.NewEncoder(w).Encode(map[string]string{schemaRegistryJSONField: string(schema)})
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			ctx := context.Background()
			enc, err := NewSingleEncoder(ctx, srv.URL, "raw-value", schemaPath)
			assert.NoError(t, err)

			wire, err := enc.Encode(map[string]any{
				"eventId":     "e1",
				"occurredAt":  int64(1),
				"url":         "http://x",
				"description": "hi",
				"author":      "",
				"tgChatIds":   []any{},
			})
			assert.NoError(t, err)
			sid, datum, err := DecodeConfluent(wire)
			assert.NoError(t, err)
			assert.Equal(t, int32(1), sid)
			assert.Equal(t, enc.SchemaID(), sid)
			codec, err := NewClient(srv.URL).CodecForID(ctx, 1)
			assert.NoError(t, err)
			native, _, err := codec.NativeFromBinary(datum)
			assert.NoError(t, err)
			rec := native.(map[string]any)
			assert.Equal(t, "e1", rec["eventId"])
		})
	}
}

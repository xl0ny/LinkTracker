package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient_RegisterAndCodecForID(t *testing.T) {
	schema := `{"type":"record","name":"X","fields":[{"name":"n","type":"int"}]}`

	var registeredID int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/subjects/topic-value/versions":
			registeredID++
			_ = json.NewEncoder(w).Encode(map[string]int{"id": registeredID})
		case r.Method == http.MethodGet && r.URL.Path == "/schemas/ids/1":
			w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
			_ = json.NewEncoder(w).Encode(map[string]string{"schema": schema})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	c := NewClient(srv.URL)
	id, err := c.RegisterSchema(ctx, "topic-value", schema)
	require.NoError(t, err)
	require.Equal(t, int32(1), id)

	codec, err := c.CodecForID(ctx, 1)
	require.NoError(t, err)
	payload, err := codec.BinaryFromNative(nil, map[string]any{"n": int32(7)})
	require.NoError(t, err)
	native, _, err := codec.NativeFromBinary(payload)
	require.NoError(t, err)
	rec := native.(map[string]any)
	require.Equal(t, int32(7), rec["n"])
}

func TestNewEncoder_integrationWithMockSR(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	updatePath := filepath.Join(root, "schemas", "avro", "link_update_event.avsc")
	failedPath := filepath.Join(root, "schemas", "avro", "failed_links_event.avsc")
	_, err := os.Stat(updatePath)
	require.NoError(t, err)

	updateSchema, err := os.ReadFile(updatePath)
	require.NoError(t, err)
	failedSchema, err := os.ReadFile(failedPath)
	require.NoError(t, err)

	schemas := map[int]string{1: string(updateSchema), 2: string(failedSchema)}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/subjects/u/versions":
			_ = json.NewEncoder(w).Encode(map[string]int{"id": 1})
		case r.Method == http.MethodPost && r.URL.Path == "/subjects/f/versions":
			_ = json.NewEncoder(w).Encode(map[string]int{"id": 2})
		case r.Method == http.MethodGet:
			var id int
			if _, scanErr := fmt.Sscanf(r.URL.Path, "/schemas/ids/%d", &id); scanErr == nil && schemas[id] != "" {
				w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
				_ = json.NewEncoder(w).Encode(map[string]string{"schema": schemas[id]})
				return
			}
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	enc, err := NewEncoder(ctx, srv.URL, "u", "f", updatePath, failedPath)
	require.NoError(t, err)

	wire, err := enc.EncodeUpdate(map[string]any{
		"eventId":     "e1",
		"occurredAt":  int64(1),
		"url":         "http://x",
		"description": map[string]any{"string": "hi"},
	})
	require.NoError(t, err)
	sid, datum, err := DecodeConfluent(wire)
	require.NoError(t, err)
	require.Equal(t, int32(1), sid)
	codec, err := enc.client.CodecForID(ctx, 1)
	require.NoError(t, err)
	native, _, err := codec.NativeFromBinary(datum)
	require.NoError(t, err)
	rec := native.(map[string]any)
	require.Equal(t, "e1", rec["eventId"])
}

package summarizer

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHuggingFace_Summarize_Success(t *testing.T) {
	type captured struct {
		method string
		auth   string
		req    map[string]any
	}
	captureCh := make(chan captured, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		captureCh <- captured{method: r.Method, auth: r.Header.Get("Authorization"), req: req}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"summary_text":"short version"}]`))
	}))
	defer srv.Close()

	hf, err := NewHuggingFace(HuggingFaceConfig{
		APIURL:    srv.URL,
		Token:     "test-token",
		MaxLength: 50,
		MinLength: 10,
	})
	require.NoError(t, err)

	out, err := hf.Summarize(context.Background(), "this is a long story about something important")
	require.NoError(t, err)
	require.Equal(t, "short version", out)

	got := <-captureCh
	require.Equal(t, http.MethodPost, got.method)
	require.Equal(t, "Bearer test-token", got.auth)
	require.Contains(t, got.req["inputs"], "long story")
}

func TestHuggingFace_Summarize_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"Model is currently loading","estimated_time":20}`))
	}))
	defer srv.Close()

	hf, err := NewHuggingFace(HuggingFaceConfig{APIURL: srv.URL, Token: "t"})
	require.NoError(t, err)

	_, err = hf.Summarize(context.Background(), "anything")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Model is currently loading")
}

func TestHuggingFace_Summarize_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	hf, err := NewHuggingFace(HuggingFaceConfig{APIURL: srv.URL})
	require.NoError(t, err)

	_, err = hf.Summarize(context.Background(), "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty summary")
}

func TestNewHuggingFace_RequiresAPIURL(t *testing.T) {
	_, err := NewHuggingFace(HuggingFaceConfig{})
	require.Error(t, err)
}

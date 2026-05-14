package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func TestCheckLink_RepoNewIssue(t *testing.T) {
	since := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"updated_at": "2025-06-01T00:00:00Z",
			"pushed_at":  "2025-06-01T00:00:00Z",
		})
	})
	mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "2025-06-01T00:00:00Z", r.URL.Query().Get("since"))
		w.Header().Set("Content-Type", "application/json")
		iss := []map[string]any{{
			"html_url":   "https://github.com/o/r/issues/9",
			"title":      "Новая задача",
			"body":       "<p>Описание задачи длинное</p>",
			"user":       map[string]string{"login": "devuser"},
			"created_at": "2026-02-01T12:00:00Z",
			"updated_at": "2026-02-01T12:00:00Z",
		}}
		_ = json.NewEncoder(w).Encode(iss)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
	out, err := c.CheckLink(context.Background(), domain.Link{URL: "https://github.com/o/r", LastUpdated: since})
	require.NoError(t, err)
	require.True(t, out.Changed)
	require.Contains(t, out.Description, "Новая задача")
	require.Contains(t, out.Description, "devuser")
	require.Contains(t, out.Description, "Issue")
	require.Contains(t, out.Description, "Превью:")
}

func TestCheckLink_RepoBaselineNoNotify(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"updated_at": "2026-01-01T00:00:00Z",
			"pushed_at":  "2026-01-01T00:00:00Z",
		})
	})
	mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.Query().Get("since"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]any{})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
	out, err := c.CheckLink(context.Background(), domain.Link{URL: "https://github.com/o/r"})
	require.NoError(t, err)
	require.False(t, out.Changed)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), out.Latest.UTC())
}

func TestCheckLink_APIUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
	_, err := c.CheckLink(context.Background(), domain.Link{URL: "https://github.com/o/r", LastUpdated: time.Unix(1, 0)})
	require.Error(t, err)
}

func TestCheckLink_PreviewTruncationInMessage(t *testing.T) {
	since := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	longBody := strings.Repeat("ж", 500)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/x/y", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"updated_at": "2020-01-02T00:00:00Z",
			"pushed_at":  "2020-01-02T00:00:00Z",
		})
	})
	mux.HandleFunc("/repos/x/y/issues", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "2020-01-01T00:00:00Z", r.URL.Query().Get("since"))
		w.Header().Set("Content-Type", "application/json")
		iss := []map[string]any{{
			"html_url":   "https://github.com/x/y/issues/1",
			"title":      "T",
			"body":       longBody,
			"user":       map[string]string{"login": "u"},
			"created_at": "2021-01-01T00:00:00Z",
			"updated_at": "2021-01-01T00:00:00Z",
		}}
		_ = json.NewEncoder(w).Encode(iss)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
	out, err := c.CheckLink(context.Background(), domain.Link{URL: "https://github.com/x/y", LastUpdated: since})
	require.NoError(t, err)
	require.True(t, out.Changed)
	idx := strings.Index(out.Description, "Превью: ")
	require.GreaterOrEqual(t, idx, 0)
	rest := out.Description[idx+len("Превью: "):]
	previewLine := strings.SplitN(rest, "\n", 2)[0]
	require.True(t, strings.HasSuffix(previewLine, "…"))
	require.LessOrEqual(t, utf8.RuneCountInString(previewLine), 201)
}

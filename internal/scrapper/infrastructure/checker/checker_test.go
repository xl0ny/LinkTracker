package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/github"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/infrastructure/stackoverflow"
)

func TestChecker_Check_RoutesGitHub(t *testing.T) {
	since := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/a/b", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"updated_at": "2020-01-02T00:00:00Z",
			"pushed_at":  "2020-01-02T00:00:00Z",
		})
	})
	mux.HandleFunc("/repos/a/b/issues", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"html_url":   "https://github.com/a/b/issues/1",
			"title":      "Issue from checker",
			"body":       "x",
			"user":       map[string]string{"login": "u"},
			"created_at": "2022-01-01T00:00:00Z",
			"updated_at": "2022-01-01T00:00:00Z",
		}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	gh := github.NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
	so := stackoverflow.NewClient(nil)
	ch := New(gh, so)

	out, err := ch.Check(context.Background(), domain.Link{
		URL:         "https://github.com/a/b",
		LastUpdated: since,
	})
	require.NoError(t, err)
	require.True(t, out.Changed)
	require.Len(t, out.Updates, 1)
	require.Contains(t, out.Updates[0].Description, "Issue from checker")
}

func TestChecker_Check_RoutesStackOverflow(t *testing.T) {
	since := time.Unix(10, 0)
	mux := http.NewServeMux()
	mux.HandleFunc("/questions/7", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"question_id": 7, "title": "Q?", "last_activity_date": 100,
			}},
		})
	})
	mux.HandleFunc("/questions/7/answers", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"answer_id": 1, "body": "A", "owner": map[string]string{"display_name": "p"},
				"creation_date": 50, "link": "https://stackoverflow.com/a/1",
			}},
		})
	})
	mux.HandleFunc("/questions/7/comments", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	})
	mux.HandleFunc("/answers/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	gh := github.NewClient(nil, "")
	so := stackoverflow.NewClientWithAPIBase(http.DefaultClient, srv.URL)
	ch := New(gh, so)

	out, err := ch.Check(context.Background(), domain.Link{
		URL:         "https://stackoverflow.com/questions/7/title",
		LastUpdated: since,
	})
	require.NoError(t, err)
	require.True(t, out.Changed)
	require.Contains(t, out.Description, "Q?")
	require.Contains(t, out.Description, "новый ответ")
}

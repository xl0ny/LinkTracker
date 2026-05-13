package stackoverflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func TestCheckQuestion_NewAnswer(t *testing.T) {
	since := time.Unix(100, 0)
	mux := newSOMux(t, 100, soFixture{
		qTitle: "Как на Go?", qLastAct: 200,
		answers: []map[string]any{{
			"answer_id":     10,
			"body":          "<code>fmt.Println</code>",
			"owner":         map[string]string{"display_name": "gopher"},
			"creation_date": 150,
			"link":          "https://stackoverflow.com/a/10",
		}},
		qComments: nil,
		aComments: map[string][]map[string]any{},
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, srv.URL)
	out, err := c.CheckQuestion(context.Background(), domain.Link{
		URL: "https://stackoverflow.com/questions/42/how", LastUpdated: since,
	})
	require.NoError(t, err)
	require.True(t, out.Changed)
	require.Contains(t, out.Description, "Как на Go?")
	require.Contains(t, out.Description, "gopher")
	require.Contains(t, out.Description, "новый ответ")
	require.Contains(t, out.Description, "Превью:")
}

func TestCheckQuestion_NewQuestionComment(t *testing.T) {
	since := time.Unix(100, 0)
	mux := newSOMux(t, 100, soFixture{
		qTitle: "T", qLastAct: 300,
		answers: nil,
		qComments: []map[string]any{{
			"comment_id":    1,
			"body":          "спасибо",
			"owner":         map[string]string{"display_name": "bob"},
			"creation_date": 200,
			"link":          "https://stackoverflow.com/questions/42/x#comment1",
		}},
		aComments: map[string][]map[string]any{},
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, srv.URL)
	out, err := c.CheckQuestion(context.Background(), domain.Link{
		URL: "https://stackoverflow.com/questions/42/x", LastUpdated: since,
	})
	require.NoError(t, err)
	require.True(t, out.Changed)
	require.Contains(t, out.Description, "новый комментарий к вопросу")
	require.Contains(t, out.Description, "bob")
}

func TestCheckQuestion_NewAnswerComment(t *testing.T) {
	since := time.Unix(100, 0)
	mux := newSOMux(t, 100, soFixture{
		qTitle: "T", qLastAct: 400,
		answers: []map[string]any{{
			"answer_id":     99,
			"body":          "old answer",
			"owner":         map[string]string{"display_name": "a1"},
			"creation_date": 50,
			"link":          "https://stackoverflow.com/a/99",
		}},
		qComments: nil,
		aComments: map[string][]map[string]any{
			"99": {{
				"comment_id":    2,
				"body":          "уточнение к ответу",
				"owner":         map[string]string{"display_name": "critic"},
				"creation_date": 250,
				"link":          "https://stackoverflow.com/questions/42/x#comment2",
			}},
		},
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, srv.URL)
	out, err := c.CheckQuestion(context.Background(), domain.Link{
		URL: "https://stackoverflow.com/questions/42/x", LastUpdated: since,
	})
	require.NoError(t, err)
	require.True(t, out.Changed)
	require.Contains(t, out.Description, "новый комментарий к ответу")
	require.Contains(t, out.Description, "critic")
}

func TestCheckQuestion_APIUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, srv.URL)
	_, err := c.CheckQuestion(context.Background(), domain.Link{URL: "https://stackoverflow.com/questions/1/x"})
	require.Error(t, err)
}

func TestCheckQuestion_NoFromdateWhenSinceZero(t *testing.T) {
	mux := newSOMux(t, 0, soFixture{
		qTitle: "T", qLastAct: 500,
		answers:   nil,
		qComments: nil,
		aComments: map[string][]map[string]any{},
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClientWithAPIBase(http.DefaultClient, srv.URL)
	out, err := c.CheckQuestion(context.Background(), domain.Link{
		URL: "https://stackoverflow.com/questions/42/x",
	})
	require.NoError(t, err)
	require.False(t, out.Changed)
}

type soFixture struct {
	qTitle    string
	qLastAct  int64
	answers   []map[string]any
	qComments []map[string]any
	// ключ — answer_id как строка для пути /answers/99/comments
	aComments map[string][]map[string]any
}

func newSOMux(t *testing.T, wantFromdateUnix int64, f soFixture) http.Handler {
	t.Helper()
	assertFromdate := func(r *http.Request) {
		t.Helper()
		got := r.URL.Query().Get("fromdate")
		if wantFromdateUnix == 0 {
			require.Empty(t, got)
			return
		}
		require.Equal(t, strconv.FormatInt(wantFromdateUnix, 10), got)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/questions/42", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "site=stackoverflow") {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"question_id":        42,
				"title":              f.qTitle,
				"last_activity_date": f.qLastAct,
			}},
		})
	})
	mux.HandleFunc("/questions/42/answers", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "site=stackoverflow") {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		require.Empty(t, r.URL.Query().Get("fromdate"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": f.answers})
	})
	mux.HandleFunc("/questions/42/comments", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "site=stackoverflow") {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		assertFromdate(r)
		w.Header().Set("Content-Type", "application/json")
		items := f.qComments
		if items == nil {
			items = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	mux.HandleFunc("/answers/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/comments") {
			http.NotFound(w, r)
			return
		}
		if !strings.Contains(r.URL.RawQuery, "site=stackoverflow") {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		assertFromdate(r)
		suffix := strings.TrimPrefix(r.URL.Path, "/answers/")
		suffix = strings.TrimSuffix(suffix, "/comments")
		var items []map[string]any
		for _, id := range strings.Split(suffix, ";") {
			if id == "" {
				continue
			}
			if list, ok := f.aComments[id]; ok {
				items = append(items, list...)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
	})
	return mux
}

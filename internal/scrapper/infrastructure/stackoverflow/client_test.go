package stackoverflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckQuestion_NewAnswer(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check question new answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			since := time.Unix(100, 0)
			mux := newSOMux(t, soFixture{
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
			out, err := c.CheckQuestion(context.Background(), "https://stackoverflow.com/questions/42/how", since)
			require.NoError(t, err)
			assert.True(t, out.Changed)
			assert.Contains(t, out.Description, "Как на Go?")
			assert.Contains(t, out.Description, "gopher")
			assert.Contains(t, out.Description, "новый ответ")
			assert.Contains(t, out.Description, "Превью:")
		})
	}
}

func TestCheckQuestion_NewQuestionComment(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check question new question comment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			since := time.Unix(100, 0)
			mux := newSOMux(t, soFixture{
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
			out, err := c.CheckQuestion(context.Background(), "https://stackoverflow.com/questions/42/x", since)
			require.NoError(t, err)
			assert.True(t, out.Changed)
			assert.Contains(t, out.Description, "новый комментарий к вопросу")
			assert.Contains(t, out.Description, "bob")
		})
	}
}

func TestCheckQuestion_NewAnswerComment(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check question new answer comment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			since := time.Unix(100, 0)
			mux := newSOMux(t, soFixture{
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
			out, err := c.CheckQuestion(context.Background(), "https://stackoverflow.com/questions/42/x", since)
			require.NoError(t, err)
			assert.True(t, out.Changed)
			assert.Contains(t, out.Description, "новый комментарий к ответу")
			assert.Contains(t, out.Description, "critic")
		})
	}
}

func TestCheckQuestion_APIUnavailable(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check question apiunavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			c := NewClientWithAPIBase(http.DefaultClient, srv.URL)
			_, err := c.CheckQuestion(context.Background(), "https://stackoverflow.com/questions/1/x", time.Time{})
			require.Error(t, err)
		})
	}
}

type soFixture struct {
	qTitle    string
	qLastAct  int64
	answers   []map[string]any
	qComments []map[string]any
	// ключ — answer_id как строка для пути /answers/99/comments
	aComments map[string][]map[string]any
}

func newSOMux(t *testing.T, f soFixture) http.Handler {
	t.Helper()
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
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": f.answers})
	})
	mux.HandleFunc("/questions/42/comments", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "site=stackoverflow") {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
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

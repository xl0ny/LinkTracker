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
)

func TestCheckLink_RepoNewIssue(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check link repo new issue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			since := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"updated_at": "2025-06-01T00:00:00Z",
					"pushed_at":  "2025-06-01T00:00:00Z",
				})
			})
			mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
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
			out, err := c.CheckLink(context.Background(), "https://github.com/o/r", since)
			assert.NoError(t, err)
			assert.True(t, out.Changed)
			assert.Len(t, out.Updates, 1)
			desc := out.Updates[0].Description
			assert.Contains(t, desc, "Новая задача")
			assert.Contains(t, desc, "devuser")
			assert.Contains(t, desc, "Issue")
			assert.Contains(t, desc, "Превью:")
		})
	}
}

func TestCheckLink_RepoMultipleNewIssues(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check link repo multiple new issues"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			since := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"updated_at": "2025-06-01T00:00:00Z",
					"pushed_at":  "2025-06-01T00:00:00Z",
				})
			})
			mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				iss := []map[string]any{
					{
						"html_url":   "https://github.com/o/r/issues/2",
						"title":      "Second",
						"body":       "b2",
						"user":       map[string]string{"login": "u2"},
						"created_at": "2026-02-03T12:00:00Z",
						"updated_at": "2026-02-03T12:00:00Z",
					},
					{
						"html_url":   "https://github.com/o/r/issues/1",
						"title":      "First",
						"body":       "b1",
						"user":       map[string]string{"login": "u1"},
						"created_at": "2026-02-01T12:00:00Z",
						"updated_at": "2026-02-01T12:00:00Z",
					},
					{
						"html_url":   "https://github.com/o/r/issues/3",
						"title":      "Third",
						"body":       "b3",
						"user":       map[string]string{"login": "u3"},
						"created_at": "2026-02-02T12:00:00Z",
						"updated_at": "2026-02-02T12:00:00Z",
					},
				}
				_ = json.NewEncoder(w).Encode(iss)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
			out, err := c.CheckLink(context.Background(), "https://github.com/o/r", since)
			assert.NoError(t, err)
			assert.True(t, out.Changed)
			assert.Len(t, out.Updates, 3)
			assert.Contains(t, out.Updates[0].Description, "First")
			assert.Contains(t, out.Updates[1].Description, "Third")
			assert.Contains(t, out.Updates[2].Description, "Second")
		})
	}
}

func TestCheckLink_RepoBaselineNoNotify(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check link repo baseline no notify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"updated_at": "2026-01-01T00:00:00Z",
					"pushed_at":  "2026-01-01T00:00:00Z",
				})
			})
			mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]any{})
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
			out, err := c.CheckLink(context.Background(), "https://github.com/o/r", time.Time{})
			assert.NoError(t, err)
			assert.False(t, out.Changed)
			assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), out.Latest.UTC())
		})
	}
}

func TestCheckLink_APIUnavailable(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check link apiunavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
			_, err := c.CheckLink(context.Background(), "https://github.com/o/r", time.Unix(1, 0))
			assert.Error(t, err)
		})
	}
}

func TestCheckLink_PreviewTruncationInMessage(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "check link preview truncation in message"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

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
			mux.HandleFunc("/repos/x/y/issues", func(w http.ResponseWriter, _ *http.Request) {
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
			out, err := c.CheckLink(context.Background(), "https://github.com/x/y", since)
			assert.NoError(t, err)
			assert.True(t, out.Changed)
			assert.Len(t, out.Updates, 1)
			idx := strings.Index(out.Updates[0].Description, "Превью: ")
			assert.GreaterOrEqual(t, idx, 0)
			rest := out.Updates[0].Description[idx+len("Превью: "):]
			previewLine := strings.SplitN(rest, "\n", 2)[0]
			assert.True(t, strings.HasSuffix(previewLine, "…"))
			assert.LessOrEqual(t, utf8.RuneCountInString(previewLine), 201)
		})
	}
}

func TestParseGitHubRef(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		expected    ghRef
		wantErr     bool
		errContains string
	}{
		{
			name: "repo url",
			raw:  "https://github.com/o/r",
			expected: ghRef{
				Owner:  "o",
				Repo:   "r",
				IsRepo: true,
			},
			wantErr: false,
		},
		{
			name: "issue url",
			raw:  "https://github.com/o/r/issues/12",
			expected: ghRef{
				Owner:  "o",
				Repo:   "r",
				IssueN: 12,
			},
			wantErr: false,
		},
		{
			name: "pull url",
			raw:  "https://github.com/o/r/pull/5",
			expected: ghRef{
				Owner:  "o",
				Repo:   "r",
				IssueN: 5,
				IsPull: true,
			},
			wantErr: false,
		},
		{
			name:        "rejects other host",
			raw:         "https://example.com/o/r",
			wantErr:     true,
			errContains: "not a github url",
		},
		{
			name:        "rejects missing repo",
			raw:         "https://github.com/o",
			wantErr:     true,
			errContains: "invalid repo path",
		},
		{
			name:        "rejects invalid issue number",
			raw:         "https://github.com/o/r/issues/nope",
			wantErr:     true,
			errContains: "invalid issue number",
		},
		{
			name:        "rejects invalid pull number",
			raw:         "https://github.com/o/r/pull/nope",
			wantErr:     true,
			errContains: "invalid pull number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGitHubRef(tt.raw)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestCheckLink_IssueOrPullComments(t *testing.T) {
	tests := []struct {
		name             string
		pageURL          string
		since            time.Time
		comments         []map[string]any
		expectedChanged  bool
		expectedLatest   time.Time
		expectedContains []string
	}{
		{
			name:    "issue comment after since changes",
			pageURL: "https://github.com/o/r/issues/7",
			since:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			comments: []map[string]any{
				{
					"html_url":   "https://github.com/o/r/issues/7#issuecomment-1",
					"body":       "<p>новый комментарий</p>",
					"user":       map[string]string{"login": "commenter"},
					"created_at": "2026-01-02T00:00:00Z",
				},
			},
			expectedChanged: true,
			expectedLatest:  time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			expectedContains: []string{
				"комментарий к Issue",
				"commenter",
				"новый комментарий",
			},
		},
		{
			name:    "pull comment uses pr kind",
			pageURL: "https://github.com/o/r/pull/7",
			since:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			comments: []map[string]any{
				{
					"html_url":   "https://github.com/o/r/pull/7#issuecomment-2",
					"body":       "pr comment",
					"user":       map[string]string{"login": "reviewer"},
					"created_at": "2026-01-03T00:00:00Z",
				},
			},
			expectedChanged:  true,
			expectedLatest:   time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
			expectedContains: []string{"комментарий к PR", "reviewer", "pr comment"},
		},
		{
			name:    "baseline returns latest without notification",
			pageURL: "https://github.com/o/r/issues/7",
			since:   time.Time{},
			comments: []map[string]any{
				{
					"html_url":   "https://github.com/o/r/issues/7#issuecomment-3",
					"body":       "baseline comment",
					"user":       map[string]string{"login": "u"},
					"created_at": "2026-01-04T00:00:00Z",
				},
			},
			expectedChanged: false,
			expectedLatest:  time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC),
		},
		{
			name:            "old comments do not notify",
			pageURL:         "https://github.com/o/r/issues/7",
			since:           time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			comments:        []map[string]any{},
			expectedChanged: false,
			expectedLatest:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r/issues/7", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"html_url":   tt.pageURL,
					"title":      "Tracked discussion",
					"body":       "issue body",
					"user":       map[string]string{"login": "author"},
					"created_at": "2026-01-01T00:00:00Z",
					"updated_at": "2026-01-01T00:00:00Z",
				})
			})
			mux.HandleFunc("/repos/o/r/issues/7/comments", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tt.comments)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
			out, err := c.CheckLink(context.Background(), tt.pageURL, tt.since)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedChanged, out.Changed)
			assert.Equal(t, tt.expectedLatest, out.Latest.UTC())
			for _, expected := range tt.expectedContains {
				assert.Contains(t, out.Description, expected)
			}
		})
	}
}

func TestCheckUpdated(t *testing.T) {
	tests := []struct {
		name     string
		pageURL  string
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "returns latest repository watermark",
			pageURL:  "https://github.com/o/r",
			expected: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:    "wraps parse error",
			pageURL: "https://example.com/o/r",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode([]any{})
			})
			mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"updated_at": "2026-01-01T00:00:00Z",
					"pushed_at":  "2026-01-02T00:00:00Z",
				})
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			c := NewClientWithAPIBase(http.DefaultClient, "", srv.URL)
			got, err := c.CheckUpdated(context.Background(), tt.pageURL)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "github CheckUpdated")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got.UTC())
		})
	}
}

func TestGetJSON(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		status         int
		body           string
		expectedAuth   string
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name:         "sets auth header and decodes json",
			token:        "secret",
			status:       http.StatusOK,
			body:         `{"ok":"yes"}`,
			expectedAuth: "Bearer secret",
			wantErr:      false,
		},
		{
			name:           "not found returns not found error",
			status:         http.StatusNotFound,
			body:           `{}`,
			wantErr:        true,
			expectedErrMsg: "not found",
		},
		{
			name:           "bad json returns decode error",
			status:         http.StatusOK,
			body:           `{`,
			wantErr:        true,
			expectedErrMsg: "decode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(srv.Close)

			c := NewClientWithAPIBase(http.DefaultClient, tt.token, srv.URL)
			var got map[string]string
			err := c.getJSON(context.Background(), srv.URL, &got)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedAuth, gotAuth)
			assert.Equal(t, "yes", got["ok"])
		})
	}
}

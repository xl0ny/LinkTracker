package scrapperclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
)

func TestNewLinkTracker_emptyURL_error(t *testing.T) {
	tests := []struct {
		name                string
		baseURL             string
		expectedErrContains string
		wantErr             bool
	}{
		{
			name:                "empty url returns error",
			baseURL:             "",
			expectedErrContains: "required",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLinkTracker(tt.baseURL, nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewLinkTracker_whitespaceURL_error(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{
			name:    "whitespace url returns error",
			baseURL: "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLinkTracker(tt.baseURL, nil)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_RegisterChat_409_returnsNil(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{
			name:    "conflict returns nil",
			status:  http.StatusConflict,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.RegisterChat(context.Background(), 1)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_RegisterChat_200_returnsNil(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{
			name:    "ok returns nil",
			status:  http.StatusOK,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.RegisterChat(context.Background(), 1)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_RegisterChat_400_returnsError(t *testing.T) {
	tests := []struct {
		name                string
		status              int
		expectedErrContains string
		wantErr             bool
	}{
		{
			name:                "bad request returns error",
			status:              http.StatusBadRequest,
			expectedErrContains: "status 400",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.RegisterChat(context.Background(), 1)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_AddLink_409_returnsLinkAlreadyExists(t *testing.T) {
	tests := []struct {
		name                string
		status              int
		expectedErrContains string
		wantErr             bool
	}{
		{
			name:                "conflict returns link already exists",
			status:              http.StatusConflict,
			expectedErrContains: "link already exists",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.AddLink(context.Background(), 1, "https://github.com/a/b", nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_AddLink_404_returnsChatNotFound(t *testing.T) {
	tests := []struct {
		name                string
		status              int
		expectedErrContains string
		wantErr             bool
	}{
		{
			name:                "not found returns chat not found",
			status:              http.StatusNotFound,
			expectedErrContains: "chat not found",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.AddLink(context.Background(), 1, "https://github.com/a/b", nil)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_AddLink_moreStatusesAndRequest(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		link         string
		tags         []string
		wantErr      bool
		expectedErr  error
		errContains  string
		expectedPath string
		expectedChat string
	}{
		{
			name:         "created sends chat header and body",
			status:       http.StatusCreated,
			link:         "https://github.com/a/b",
			tags:         []string{"go", "review"},
			wantErr:      false,
			expectedPath: "/links",
			expectedChat: "42",
		},
		{
			name:        "server error returns status error",
			status:      http.StatusInternalServerError,
			link:        "https://github.com/a/b",
			wantErr:     true,
			errContains: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotChatID string
			var gotBody PostLinksJSONRequestBody
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotChatID = r.Header.Get("Tg-Chat-Id")
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.AddLink(context.Background(), 42, tt.link, tt.tags)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != nil {
					assert.ErrorIs(t, err, tt.expectedErr)
				}
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPath, gotPath)
			assert.Equal(t, tt.expectedChat, gotChatID)
			assert.Equal(t, tt.link, *gotBody.Link)
			assert.Equal(t, tt.tags, *gotBody.Tags)
		})
	}
}

func TestClient_ListLinks_404_returnsError(t *testing.T) {
	tests := []struct {
		name                string
		status              int
		body                string
		expectedErrContains string
		wantErr             bool
	}{
		{
			name:                "not found returns chat not found",
			status:              http.StatusNotFound,
			expectedErrContains: "chat not found",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			links, err := lt.ListLinks(context.Background(), 1, "")

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, links)
				assert.Contains(t, err.Error(), tt.expectedErrContains)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClient_ListLinks_moreResponses(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		tagFilter     string
		expectedLinks []application.LinkInfo
		wantErr       bool
		errContains   string
	}{
		{
			name:      "filters links by tag",
			status:    http.StatusOK,
			body:      `{"links":[{"url":"https://github.com/a/b","tags":["go"]},{"url":"https://github.com/c/d","tags":["java"]}]}`,
			tagFilter: "go",
			expectedLinks: []application.LinkInfo{
				{URL: "https://github.com/a/b", Tags: []string{"go"}},
			},
			wantErr: false,
		},
		{
			name:          "handles nil url and tags",
			status:        http.StatusOK,
			body:          `{"links":[{}]}`,
			expectedLinks: []application.LinkInfo{{URL: "", Tags: []string{}}},
			wantErr:       false,
		},
		{
			name:        "bad json returns decode error",
			status:      http.StatusOK,
			body:        `{`,
			wantErr:     true,
			errContains: "decode response",
		},
		{
			name:        "server error returns status error",
			status:      http.StatusInternalServerError,
			body:        `{}`,
			wantErr:     true,
			errContains: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			links, err := lt.ListLinks(context.Background(), 1, tt.tagFilter)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLinks, links)
		})
	}
}

func TestClient_ListLinks_200_emptyBody_returnsNilSlice(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		expectedLinks []application.LinkInfo
		wantErr       bool
	}{
		{
			name:          "empty body returns nil slice",
			status:        http.StatusOK,
			body:          `{}`,
			expectedLinks: nil,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			links, err := lt.ListLinks(context.Background(), 1, "")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLinks, links)
		})
	}
}

func TestClient_ListLinks_200_withLinks_returnsParsed(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		expectedLinks []application.LinkInfo
		wantErr       bool
	}{
		{
			name:   "with links returns parsed response",
			status: http.StatusOK,
			body:   `{"links":[{"url":"https://github.com/x/y","tags":["work"]}]}`,
			expectedLinks: []application.LinkInfo{
				{URL: "https://github.com/x/y", Tags: []string{"work"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			links, err := lt.ListLinks(context.Background(), 1, "")

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedLinks, links)
		})
	}
}

func TestClient_RemoveLink_404_returnsLinkNotFound(t *testing.T) {
	tests := []struct {
		name                string
		status              int
		expectedErrContains string
		wantErr             bool
	}{
		{
			name:                "not found returns link not found",
			status:              http.StatusNotFound,
			expectedErrContains: "link not found",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.RemoveLink(context.Background(), 1, "https://github.com/a/b")

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_RemoveLink_moreStatusesAndRequest(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		link         string
		wantErr      bool
		errContains  string
		expectedPath string
		expectedChat string
	}{
		{
			name:         "ok sends chat header and body",
			status:       http.StatusOK,
			link:         "https://github.com/a/b",
			wantErr:      false,
			expectedPath: "/links",
			expectedChat: "42",
		},
		{
			name:        "server error returns status error",
			status:      http.StatusInternalServerError,
			link:        "https://github.com/a/b",
			wantErr:     true,
			errContains: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotChatID string
			var gotBody RemoveLinkRequest
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotChatID = r.Header.Get("Tg-Chat-Id")
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			lt, err := NewLinkTracker(srv.URL, nil)
			if !assert.NoError(t, err) {
				return
			}
			err = lt.RemoveLink(context.Background(), 42, tt.link)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPath, gotPath)
			assert.Equal(t, tt.expectedChat, gotChatID)
			assert.Equal(t, tt.link, *gotBody.Link)
		})
	}
}

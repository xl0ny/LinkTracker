package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/transport/http/api"
)

type fakeUseCase struct {
	registerErr error
	deleteErr   error
	addErr      error
	getLinks    []domain.Link
	getErr      error
	removeLink  domain.Link
	removeErr   error

	addChatID int64
	addLink   string
	addTags   *[]string
	addFilter *[]string
}

func (f *fakeUseCase) ChatRegistration(_ context.Context, _ int64) error {
	return f.registerErr
}

func (f *fakeUseCase) ChatDelition(_ context.Context, _ int64) error {
	return f.deleteErr
}

func (f *fakeUseCase) LinkAddment(_ context.Context, chatID int64, link string, tags, filters *[]string) error {
	f.addChatID = chatID
	f.addLink = link
	f.addTags = tags
	f.addFilter = filters
	return f.addErr
}

func (f *fakeUseCase) GetLinks(_ context.Context, _ int64) ([]domain.Link, error) {
	return f.getLinks, f.getErr
}

func (f *fakeUseCase) DeleteLink(_ context.Context, _ int64, _ string) (domain.Link, error) {
	return f.removeLink, f.removeErr
}

func TestHandler_PostTgChatId(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{name: "registers chat", expectedStatus: http.StatusOK},
		{name: "chat already exists", err: domain.ErrChatAlreadyExists, expectedStatus: http.StatusConflict},
		{name: "internal error", err: errors.New("db down"), expectedStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h := NewHandler(&fakeUseCase{registerErr: tt.err})

			h.PostTgChatId(rec, httptest.NewRequest(http.MethodPost, "/tg-chat/1", nil), 1)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestHandler_DeleteTgChatId(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
	}{
		{name: "deletes chat", expectedStatus: http.StatusOK},
		{name: "chat not found", err: domain.ErrChatNotFound, expectedStatus: http.StatusNotFound},
		{name: "internal error", err: errors.New("db down"), expectedStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h := NewHandler(&fakeUseCase{deleteErr: tt.err})

			h.DeleteTgChatId(rec, httptest.NewRequest(http.MethodDelete, "/tg-chat/1", nil), 1)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestHandler_PostLinks(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		err            error
		expectedStatus int
		expectedLink   string
		expectedTags   []string
	}{
		{
			name:           "adds link",
			body:           `{"link":"https://github.com/a/b","tags":["go"],"filters":["priority=high"]}`,
			expectedStatus: http.StatusOK,
			expectedLink:   "https://github.com/a/b",
			expectedTags:   []string{"go"},
		},
		{name: "invalid json", body: `{`, expectedStatus: http.StatusBadRequest},
		{name: "empty link", body: `{}`, expectedStatus: http.StatusBadRequest},
		{name: "link already exists", body: `{"link":"https://github.com/a/b"}`, err: domain.ErrLinkAlreadyExists, expectedStatus: http.StatusConflict},
		{name: "chat not found", body: `{"link":"https://github.com/a/b"}`, err: domain.ErrChatNotFound, expectedStatus: http.StatusNotFound},
		{name: "internal error", body: `{"link":"https://github.com/a/b"}`, err: errors.New("db down"), expectedStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			uc := &fakeUseCase{addErr: tt.err}
			h := NewHandler(uc)

			h.PostLinks(rec, httptest.NewRequest(http.MethodPost, "/links", bytes.NewBufferString(tt.body)), api.PostLinksParams{TgChatId: 7})

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedStatus == http.StatusOK {
				var got api.LinkResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.NotNil(t, got.Url)
				assert.Equal(t, tt.expectedLink, *got.Url)
				require.NotNil(t, got.Tags)
				assert.Equal(t, tt.expectedTags, *got.Tags)
				assert.Equal(t, int64(7), uc.addChatID)
				assert.Equal(t, tt.expectedLink, uc.addLink)
			}
		})
	}
}

func TestHandler_GetLinks(t *testing.T) {
	tests := []struct {
		name           string
		links          []domain.Link
		err            error
		expectedStatus int
		expectedSize   int32
	}{
		{
			name: "returns links",
			links: []domain.Link{
				{URL: "https://github.com/a/b", Tags: []string{"go"}, Filters: []string{"x=y"}},
			},
			expectedStatus: http.StatusOK,
			expectedSize:   1,
		},
		{name: "chat not found", err: domain.ErrChatNotFound, expectedStatus: http.StatusNotFound},
		{name: "unexpected error still returns empty list", err: errors.New("db down"), expectedStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h := NewHandler(&fakeUseCase{getLinks: tt.links, getErr: tt.err})

			h.GetLinks(rec, httptest.NewRequest(http.MethodGet, "/links", nil), api.GetLinksParams{TgChatId: 7})

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedStatus == http.StatusOK {
				var got api.ListLinksResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.NotNil(t, got.Size)
				assert.Equal(t, tt.expectedSize, *got.Size)
			}
		})
	}
}

func TestHandler_DeleteLinks(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		removed        domain.Link
		err            error
		expectedStatus int
		expectedURL    string
	}{
		{
			name:           "removes link",
			body:           `{"link":"https://github.com/a/b"}`,
			removed:        domain.Link{URL: "https://github.com/a/b", Tags: []string{"go"}, Filters: []string{"x=y"}},
			expectedStatus: http.StatusOK,
			expectedURL:    "https://github.com/a/b",
		},
		{name: "invalid json", body: `{`, expectedStatus: http.StatusBadRequest},
		{name: "empty link", body: `{}`, expectedStatus: http.StatusBadRequest},
		{name: "chat not found", body: `{"link":"https://github.com/a/b"}`, err: domain.ErrChatNotFound, expectedStatus: http.StatusNotFound},
		{name: "link not found", body: `{"link":"https://github.com/a/b"}`, err: domain.ErrLinkNotFound, expectedStatus: http.StatusNotFound},
		{name: "internal error", body: `{"link":"https://github.com/a/b"}`, err: errors.New("db down"), expectedStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h := NewHandler(&fakeUseCase{removeLink: tt.removed, removeErr: tt.err})

			h.DeleteLinks(rec, httptest.NewRequest(http.MethodDelete, "/links", bytes.NewBufferString(tt.body)), api.DeleteLinksParams{TgChatId: 7})

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if tt.expectedStatus == http.StatusOK {
				var got api.LinkResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
				require.NotNil(t, got.Url)
				assert.Equal(t, tt.expectedURL, *got.Url)
			}
		})
	}
}

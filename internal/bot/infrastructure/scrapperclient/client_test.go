package scrapperclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLinkTracker_emptyURL_error(t *testing.T) {
	_, err := NewLinkTracker("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required")
}

func TestNewLinkTracker_whitespaceURL_error(t *testing.T) {
	_, err := NewLinkTracker("   ")
	require.Error(t, err)
}

func TestClient_RegisterChat_409_returnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	err = lt.RegisterChat(context.Background(), 1)
	assert.NoError(t, err)
}

func TestClient_RegisterChat_200_returnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	err = lt.RegisterChat(context.Background(), 1)
	assert.NoError(t, err)
}

func TestClient_RegisterChat_400_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	err = lt.RegisterChat(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
}

func TestClient_AddLink_409_returnsLinkAlreadyExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	err = lt.AddLink(context.Background(), 1, "https://github.com/a/b", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link already exists")
}

func TestClient_AddLink_404_returnsChatNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	err = lt.AddLink(context.Background(), 1, "https://github.com/a/b", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chat not found")
}

func TestClient_ListLinks_404_returnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	links, err := lt.ListLinks(context.Background(), 1, "")
	require.Error(t, err)
	assert.Nil(t, links)
	assert.Contains(t, err.Error(), "chat not found")
}

func TestClient_ListLinks_200_emptyBody_returnsNilSlice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	links, err := lt.ListLinks(context.Background(), 1, "")
	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestClient_ListLinks_200_withLinks_returnsParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"links":[{"url":"https://github.com/x/y","tags":["work"]}]}`))
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	links, err := lt.ListLinks(context.Background(), 1, "")
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "https://github.com/x/y", links[0].URL)
	assert.Equal(t, []string{"work"}, links[0].Tags)
}

func TestClient_RemoveLink_404_returnsLinkNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	lt, err := NewLinkTracker(srv.URL)
	require.NoError(t, err)
	err = lt.RemoveLink(context.Background(), 1, "https://github.com/a/b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link not found")
}

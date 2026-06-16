package helper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		body           map[string]string
		expectedStatus int
		expectedBody   map[string]string
	}{
		{
			name:           "writes json body and status",
			status:         http.StatusCreated,
			body:           map[string]string{"ok": "true"},
			expectedStatus: http.StatusCreated,
			expectedBody:   map[string]string{"ok": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			WriteJSON(rec, tt.status, tt.body)

			var got map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.Equal(t, tt.expectedBody, got)
		})
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		code           string
		description    string
		exceptionName  string
		message        string
		expectedStatus int
	}{
		{
			name:           "writes api error response",
			status:         http.StatusBadRequest,
			code:           "bad_request",
			description:    "bad input",
			exceptionName:  "ValidationError",
			message:        "field is required",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			WriteError(rec, tt.status, "test error", tt.code, tt.description, tt.exceptionName, tt.message)

			var got APIErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.code, *got.Code)
			assert.Equal(t, tt.description, *got.Description)
			assert.Equal(t, tt.exceptionName, *got.ExceptionName)
			assert.Equal(t, tt.message, *got.ExceptionMessage)
		})
	}
}

func TestPtr(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "returns pointer to value",
			input: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Ptr(tt.input)

			assert.NotNil(t, got)
			assert.Equal(t, tt.input, *got)
		})
	}
}

func TestListenAddress(t *testing.T) {
	tests := []struct {
		name     string
		port     string
		expected string
	}{
		{
			name:     "joins empty host with port",
			port:     "8080",
			expected: ":8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ListenAddress(tt.port))
		})
	}
}

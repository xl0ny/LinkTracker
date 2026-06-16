package transporthttp

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeSender struct {
	err      error
	messages map[int64]string
}

func (s *fakeSender) SendMessage(chatID int64, message string) error {
	if s.messages == nil {
		s.messages = make(map[int64]string)
	}
	s.messages[chatID] = message
	return s.err
}

func TestHandler_PostUpdates(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		sendErr          error
		expectedStatus   int
		expectedMessages map[int64]string
	}{
		{
			name:           "invalid json",
			body:           `{`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing chat ids",
			body:           `{"url":"https://github.com/a/b"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "uses default message",
			body:           `{"tgChatIds":[1]}`,
			expectedStatus: http.StatusOK,
			expectedMessages: map[int64]string{
				1: "Обновление обнаружено",
			},
		},
		{
			name:           "uses url message",
			body:           `{"tgChatIds":[1],"url":"https://github.com/a/b"}`,
			expectedStatus: http.StatusOK,
			expectedMessages: map[int64]string{
				1: "Обновление: https://github.com/a/b",
			},
		},
		{
			name:           "description overrides url",
			body:           `{"tgChatIds":[1,2],"url":"https://github.com/a/b","description":"custom update"}`,
			expectedStatus: http.StatusOK,
			expectedMessages: map[int64]string{
				1: "custom update",
				2: "custom update",
			},
		},
		{
			name:           "send error returns internal error",
			body:           `{"tgChatIds":[1],"description":"custom update"}`,
			sendErr:        errors.New("telegram down"),
			expectedStatus: http.StatusInternalServerError,
			expectedMessages: map[int64]string{
				1: "custom update",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &fakeSender{err: tt.sendErr}
			handler := NewHandler(sender)
			rec := httptest.NewRecorder()

			handler.PostUpdates(rec, httptest.NewRequest(http.MethodPost, "/updates", bytes.NewBufferString(tt.body)))

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.expectedMessages, sender.messages)
		})
	}
}

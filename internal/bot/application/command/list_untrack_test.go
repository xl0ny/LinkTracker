package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application"
	appmocks "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
)

func TestList_Name(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "returns command name",
			expected: "list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewList(nil)

			assert.Equal(t, tt.expected, cmd.Name())
		})
	}
}

func TestList_Description(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "returns non-empty description"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewList(nil)

			assert.NotEmpty(t, cmd.Description())
		})
	}
}

func TestList_Handle(t *testing.T) {
	tests := []struct {
		name             string
		action           domain.Action
		links            []application.LinkInfo
		listErr          error
		expectedText     string
		expectedContains []string
		expectedTag      string
		wantErr          bool
	}{
		{
			name:         "empty list returns empty message",
			action:       domain.Action{ChatID: 7},
			expectedText: "Нет отслеживаемых ссылок",
			wantErr:      false,
		},
		{
			name:   "formats links with tags",
			action: domain.Action{ChatID: 7, Args: " go "},
			links: []application.LinkInfo{
				{URL: "https://github.com/a/b", Tags: []string{"go", "review"}},
				{URL: "https://stackoverflow.com/questions/1/x"},
			},
			expectedContains: []string{
				"• https://github.com/a/b [go, review]",
				"• https://stackoverflow.com/questions/1/x",
			},
			expectedTag: "go",
			wantErr:     false,
		},
		{
			name:         "chat not found asks to start",
			action:       domain.Action{ChatID: 9},
			listErr:      application.ErrChatNotFound,
			expectedText: msgUseStart,
			wantErr:      false,
		},
		{
			name:    "unexpected error is returned",
			action:  domain.Action{ChatID: 10},
			listErr: errors.New("transport down"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tracker := appmocks.NewMockLinkTracker(ctrl)
			tracker.EXPECT().
				ListLinks(gomock.Any(), tt.action.ChatID, tt.expectedTag).
				Return(tt.links, tt.listErr)
			cmd := NewList(tracker)

			text, err := cmd.Handle(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.expectedText != "" {
				assert.Equal(t, tt.expectedText, text)
			}
			for _, expected := range tt.expectedContains {
				assert.Contains(t, text, expected)
			}
		})
	}
}

func TestUntrack_Name(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "returns command name",
			expected: "untrack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewUntrack(nil)

			assert.Equal(t, tt.expected, cmd.Name())
		})
	}
}

func TestUntrack_Description(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "returns non-empty description"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewUntrack(nil)

			assert.NotEmpty(t, cmd.Description())
		})
	}
}

func TestUntrack_Handle(t *testing.T) {
	tests := []struct {
		name         string
		action       domain.Action
		removeErr    error
		expectedText string
		expectedLink string
		wantErr      bool
	}{
		{
			name:         "empty args asks for link",
			action:       domain.Action{ChatID: 1, Args: " "},
			expectedText: "Укажите ссылку: /untrack <ссылка>",
			wantErr:      false,
		},
		{
			name:         "removes link",
			action:       domain.Action{ChatID: 1, Args: " https://github.com/a/b "},
			expectedText: "Ссылка удалена из отслеживания",
			expectedLink: "https://github.com/a/b",
			wantErr:      false,
		},
		{
			name:         "link not found returns message",
			action:       domain.Action{ChatID: 1, Args: "https://github.com/a/b"},
			removeErr:    application.ErrLinkNotFound,
			expectedText: "Ссылка не найдена в отслеживаемых",
			expectedLink: "https://github.com/a/b",
			wantErr:      false,
		},
		{
			name:         "chat not found asks to start",
			action:       domain.Action{ChatID: 1, Args: "https://github.com/a/b"},
			removeErr:    application.ErrChatNotFound,
			expectedText: msgUseStart,
			expectedLink: "https://github.com/a/b",
			wantErr:      false,
		},
		{
			name:         "unexpected error is returned",
			action:       domain.Action{ChatID: 1, Args: "https://github.com/a/b"},
			removeErr:    errors.New("transport down"),
			expectedLink: "https://github.com/a/b",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			tracker := appmocks.NewMockLinkTracker(ctrl)
			if tt.expectedLink != "" {
				tracker.EXPECT().
					RemoveLink(gomock.Any(), tt.action.ChatID, tt.expectedLink).
					Return(tt.removeErr)
			}
			cmd := NewUntrack(tracker)

			text, err := cmd.Handle(tt.action)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedText, text)
		})
	}
}

func TestCreateLinksString(t *testing.T) {
	tests := []struct {
		name     string
		links    []application.LinkInfo
		expected string
	}{
		{
			name: "formats multiple links",
			links: []application.LinkInfo{
				{URL: "https://github.com/a/b", Tags: []string{"go"}},
				{URL: "https://stackoverflow.com/questions/1/x"},
			},
			expected: "• https://github.com/a/b [go]\n• https://stackoverflow.com/questions/1/x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, createLinksString(tt.links))
		})
	}
}

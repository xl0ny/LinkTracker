package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrackStateStore(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "stores gets clears and deletes nil state"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewTrackStateStore()
			state := &TrackState{Phase: TrackPhaseLink, Link: "https://github.com/a/b"}

			assert.Nil(t, store.Get(1))

			store.Set(1, state)
			assert.Equal(t, state, store.Get(1))

			store.Clear(1)
			assert.Nil(t, store.Get(1))

			store.Set(1, state)
			store.Set(1, nil)
			assert.Nil(t, store.Get(1))
		})
	}
}

func TestIsValidLink(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "accepts github http link",
			input:    "http://github.com/a/b",
			expected: true,
		},
		{
			name:     "accepts github https link",
			input:    "https://github.com/a/b",
			expected: true,
		},
		{
			name:     "accepts stackoverflow link",
			input:    "https://stackoverflow.com/questions/1/x",
			expected: true,
		},
		{
			name:     "accepts localized stackoverflow link",
			input:    "https://ru.stackoverflow.com/questions/1/x",
			expected: true,
		},
		{
			name:     "rejects unsupported scheme",
			input:    "ftp://github.com/a/b",
			expected: false,
		},
		{
			name:     "rejects unsupported host",
			input:    "https://example.com/a/b",
			expected: false,
		},
		{
			name:     "rejects invalid url",
			input:    "http://%",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidLink(tt.input))
		})
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty input returns nil",
			input:    " ",
			expected: nil,
		},
		{
			name:     "splits and trims comma separated tags",
			input:    " go, review , , backend ",
			expected: []string{"go", "review", "backend"},
		},
		{
			name:     "keeps single tag",
			input:    "go",
			expected: []string{"go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ParseTags(tt.input))
		})
	}
}

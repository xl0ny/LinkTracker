package textutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPreview_TruncatesRunes(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		limit          int
		expectedRunes  int
		expectedSuffix string
	}{
		{
			name:           "truncates by runes",
			input:          strings.Repeat("аб", 150),
			limit:          200,
			expectedRunes:  201,
			expectedSuffix: "…",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Preview(tt.input, tt.limit)

			assert.Len(t, []rune(got), tt.expectedRunes)
			assert.True(t, strings.HasSuffix(got, tt.expectedSuffix))
		})
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "removes html tags",
			input:    "<p>hello</p> <b>world</b>",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StripHTML(tt.input))
		})
	}
}

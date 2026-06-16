package summarizer

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStub_TruncatesLongText(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "truncates long text",
			limit:    10,
			input:    "abcdefghijKLMNOP",
			expected: "abcdefghij...",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStub(tt.limit)

			out, err := s.Summarize(context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestStub_KeepsShortText(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "keeps short text",
			limit:    50,
			input:    "short text",
			expected: "short text",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStub(tt.limit)

			out, err := s.Summarize(context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

func TestStub_RuneSafeCut(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "truncates by runes",
			limit:    3,
			input:    strings.Repeat("ё", 10),
			expected: "ёёё...",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStub(tt.limit)

			out, err := s.Summarize(context.Background(), tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, out)
		})
	}
}

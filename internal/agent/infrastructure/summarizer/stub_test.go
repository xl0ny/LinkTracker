package summarizer

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStub_TruncatesLongText(t *testing.T) {
	s := NewStub(10)
	out, err := s.Summarize(context.Background(), "abcdefghijKLMNOP")
	require.NoError(t, err)
	require.Equal(t, "abcdefghij...", out)
}

func TestStub_KeepsShortText(t *testing.T) {
	s := NewStub(50)
	in := "short text"
	out, err := s.Summarize(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestStub_RuneSafeCut(t *testing.T) {
	s := NewStub(3)
	out, err := s.Summarize(context.Background(), strings.Repeat("ё", 10))
	require.NoError(t, err)
	require.Equal(t, "ёёё...", out)
}

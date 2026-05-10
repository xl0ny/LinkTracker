package textutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreview_TruncatesRunes(t *testing.T) {
	s := strings.Repeat("аб", 150) // 300 runes
	got := Preview(s, 200)
	require.Len(t, []rune(got), 201) // 200 + ellipsis
	require.True(t, strings.HasSuffix(got, "…"))
}

func TestStripHTML(t *testing.T) {
	require.Equal(t, "hello world", StripHTML("<p>hello</p> <b>world</b>"))
}

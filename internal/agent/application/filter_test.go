package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

func TestFilter_StopWord(t *testing.T) {
	f := NewFilter(FilterConfig{
		StopWords: []string{"spam", "ads"},
		MinLength: 10,
	})
	u := domain.RawUpdate{
		Description: "this update is spam, please ignore",
		Author:      "alice",
	}
	require.False(t, f.Allows(u))
	require.Equal(t, SkipStopWord, f.Decide(u))
}

func TestFilter_StopWordIsCaseInsensitive(t *testing.T) {
	f := NewFilter(FilterConfig{StopWords: []string{"PROMO"}, MinLength: 5})
	u := domain.RawUpdate{Description: "Limited Time PrOmO inside the box", Author: "alice"}
	require.False(t, f.Allows(u))
}

func TestFilter_ExcludedAuthor(t *testing.T) {
	f := NewFilter(FilterConfig{
		ExcludedAuthors: []string{"bot-user", "automation"},
		MinLength:       10,
	})
	u := domain.RawUpdate{
		Description: "real human update with enough length",
		Author:      "Bot-User",
	}
	require.False(t, f.Allows(u))
	require.Equal(t, SkipExcludedAuthor, f.Decide(u))
}

func TestFilter_MinLength(t *testing.T) {
	f := NewFilter(FilterConfig{MinLength: 20})
	u := domain.RawUpdate{
		Description: "too short",
		Author:      "alice",
	}
	require.False(t, f.Allows(u))
	require.Equal(t, SkipMinLength, f.Decide(u))
}

func TestFilter_MinLengthCountsRunes(t *testing.T) {
	f := NewFilter(FilterConfig{MinLength: 10})
	u := domain.RawUpdate{
		Description: strings.Repeat("ж", 9),
		Author:      "alice",
	}
	require.False(t, f.Allows(u))
}

func TestFilter_AllowsValidUpdate(t *testing.T) {
	f := NewFilter(FilterConfig{
		StopWords:       []string{"spam"},
		ExcludedAuthors: []string{"bot"},
		MinLength:       10,
	})
	u := domain.RawUpdate{
		Description: "everything looks fine here",
		Author:      "alice",
	}
	require.True(t, f.Allows(u))
	require.Equal(t, SkipNone, f.Decide(u))
}

func TestFilter_EmptyConfigAllows(t *testing.T) {
	f := NewFilter(FilterConfig{})
	require.True(t, f.Allows(domain.RawUpdate{Description: "x"}))
}

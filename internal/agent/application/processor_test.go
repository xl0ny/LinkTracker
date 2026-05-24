package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type fakeSummarizer struct {
	called bool
	out    string
	err    error
}

func (f *fakeSummarizer) Summarize(_ context.Context, _ string) (string, error) {
	f.called = true
	return f.out, f.err
}

func newTestProcessor(filter *Filter, sum *fakeSummarizer, threshold int) *Processor {
	prioritizer := NewPrioritizer(PrioritizerConfig{})
	return NewProcessor(filter, sum, threshold, prioritizer)
}

func TestProcessor_FiltersBlockedUpdates(t *testing.T) {
	filter := NewFilter(FilterConfig{StopWords: []string{"spam"}})
	sum := &fakeSummarizer{out: "summary"}
	p := newTestProcessor(filter, sum, 10)

	_, ok, err := p.Process(context.Background(), domain.RawUpdate{
		Description: "buy now spam offer",
		Author:      "alice",
	})
	require.NoError(t, err)
	require.False(t, ok)
	require.False(t, sum.called)
}

func TestProcessor_LongTextSummarized(t *testing.T) {
	filter := NewFilter(FilterConfig{})
	long := strings.Repeat("a", 600)
	sum := &fakeSummarizer{out: "short summary"}
	p := newTestProcessor(filter, sum, 500)

	out, ok, err := p.Process(context.Background(), domain.RawUpdate{
		EventID:     "e1",
		URL:         "https://example.com",
		Description: long,
		Author:      "alice",
		TgChatIDs:   []int64{42},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, sum.called)
	require.Equal(t, "short summary", out.Description)
	require.NotEqual(t, long, out.Description)
	require.Equal(t, []int64{42}, out.TgChatIDs)
	require.Equal(t, PriorityMedium, out.Priority)
}

func TestProcessor_ShortTextNotSummarized(t *testing.T) {
	filter := NewFilter(FilterConfig{})
	short := "compact update"
	sum := &fakeSummarizer{out: "should not be used"}
	p := newTestProcessor(filter, sum, 500)

	out, ok, err := p.Process(context.Background(), domain.RawUpdate{
		EventID:     "e2",
		URL:         "https://example.com",
		Description: short,
		Author:      "alice",
		TgChatIDs:   []int64{1, 2},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, sum.called)
	require.Equal(t, short, out.Description)
}

func TestProcessor_SummarizerErrorFallsBackToOriginal(t *testing.T) {
	filter := NewFilter(FilterConfig{})
	long := strings.Repeat("b", 600)
	sum := &fakeSummarizer{err: errors.New("boom")}
	p := newTestProcessor(filter, sum, 500)

	out, ok, err := p.Process(context.Background(), domain.RawUpdate{
		EventID:     "e3",
		URL:         "https://example.com",
		Description: long,
		TgChatIDs:   []int64{99},
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, sum.called)
	require.Equal(t, long, out.Description)
}

func TestProcessor_PrioritizesUpdate(t *testing.T) {
	filter := NewFilter(FilterConfig{})
	prioritizer := NewPrioritizer(PrioritizerConfig{
		HighKeywords: []string{"critical"},
		LowKeywords:  []string{"typo"},
	})
	p := NewProcessor(filter, nil, 0, prioritizer)

	out, ok, err := p.Process(context.Background(), domain.RawUpdate{
		Description: "critical security patch",
		Author:      "alice",
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, PriorityHigh, out.Priority)
}

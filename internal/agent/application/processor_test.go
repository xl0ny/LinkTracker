package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

func newTestProcessor(filter *Filter, sum Summarizer, threshold int) *Processor {
	prioritizer := NewPrioritizer(PrioritizerConfig{})
	return NewProcessor(filter, sum, threshold, prioritizer)
}

func TestProcessor_FiltersBlockedUpdates(t *testing.T) {
	ctrl := gomock.NewController(t)
	filter := NewFilter(FilterConfig{StopWords: []string{"spam"}})
	sum := mocks.NewMockSummarizer(ctrl)
	p := newTestProcessor(filter, sum, 10)

	_, ok := p.Process(context.Background(), domain.RawUpdate{
		Description: "buy now spam offer",
		Author:      "alice",
	})
	require.False(t, ok)
}

func TestProcessor_LongTextSummarized(t *testing.T) {
	ctrl := gomock.NewController(t)
	filter := NewFilter(FilterConfig{})
	long := strings.Repeat("a", 600)
	sum := mocks.NewMockSummarizer(ctrl)
	sum.EXPECT().
		Summarize(gomock.Any(), long).
		Return("short summary", nil)
	p := newTestProcessor(filter, sum, 500)

	out, ok := p.Process(context.Background(), domain.RawUpdate{
		EventID:     "e1",
		URL:         "https://example.com",
		Description: long,
		Author:      "alice",
		TgChatIDs:   []int64{42},
	})
	require.True(t, ok)
	require.Equal(t, "short summary", out.Description)
	require.NotEqual(t, long, out.Description)
	require.Equal(t, []int64{42}, out.TgChatIDs)
	require.Equal(t, PriorityMedium, out.Priority)
}

func TestProcessor_ShortTextNotSummarized(t *testing.T) {
	ctrl := gomock.NewController(t)
	filter := NewFilter(FilterConfig{})
	short := "compact update"
	sum := mocks.NewMockSummarizer(ctrl)
	p := newTestProcessor(filter, sum, 500)

	out, ok := p.Process(context.Background(), domain.RawUpdate{
		EventID:     "e2",
		URL:         "https://example.com",
		Description: short,
		Author:      "alice",
		TgChatIDs:   []int64{1, 2},
	})
	require.True(t, ok)
	require.Equal(t, short, out.Description)
}

func TestProcessor_SummarizerErrorFallsBackToOriginal(t *testing.T) {
	ctrl := gomock.NewController(t)
	filter := NewFilter(FilterConfig{})
	long := strings.Repeat("b", 600)
	sum := mocks.NewMockSummarizer(ctrl)
	sum.EXPECT().
		Summarize(gomock.Any(), long).
		Return("", errors.New("boom"))
	p := newTestProcessor(filter, sum, 500)

	out, ok := p.Process(context.Background(), domain.RawUpdate{
		EventID:     "e3",
		URL:         "https://example.com",
		Description: long,
		TgChatIDs:   []int64{99},
	})
	require.True(t, ok)
	require.Equal(t, long, out.Description)
}

func TestProcessor_PrioritizesUpdate(t *testing.T) {
	filter := NewFilter(FilterConfig{})
	prioritizer := NewPrioritizer(PrioritizerConfig{
		HighKeywords: []string{"critical"},
		LowKeywords:  []string{"typo"},
	})
	p := NewProcessor(filter, nil, 0, prioritizer)

	out, ok := p.Process(context.Background(), domain.RawUpdate{
		Description: "critical security patch",
		Author:      "alice",
	})
	require.True(t, ok)
	require.Equal(t, PriorityHigh, out.Priority)
}

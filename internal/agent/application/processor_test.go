package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/application/mocks"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

func newTestProcessor(filter *Filter, sum Summarizer, threshold int) *Processor {
	prioritizer := NewPrioritizer(PrioritizerConfig{})
	return NewProcessor(filter, sum, threshold, prioritizer)
}

func TestProcessor_FiltersBlockedUpdates(t *testing.T) {
	tests := []struct {
		name     string
		config   FilterConfig
		update   domain.RawUpdate
		expected bool
	}{
		{
			name:   "filters blocked update",
			config: FilterConfig{StopWords: []string{"spam"}},
			update: domain.RawUpdate{
				Description: "buy now spam offer",
				Author:      "alice",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			filter := NewFilter(tt.config)
			sum := mocks.NewMockSummarizer(ctrl)
			p := newTestProcessor(filter, sum, 10)

			_, ok := p.Process(context.Background(), tt.update)

			assert.Equal(t, tt.expected, ok)
		})
	}
}

func TestProcessor_LongTextSummarized(t *testing.T) {
	long := strings.Repeat("a", 600)
	tests := []struct {
		name                string
		update              domain.RawUpdate
		threshold           int
		summary             string
		expectedDescription string
		expectedChatIDs     []int64
		expectedPriority    string
		expectedOK          bool
	}{
		{
			name: "summarizes long text",
			update: domain.RawUpdate{
				EventID:     "e1",
				URL:         "https://example.com",
				Description: long,
				Author:      "alice",
				TgChatIDs:   []int64{42},
			},
			threshold:           500,
			summary:             "short summary",
			expectedDescription: "short summary",
			expectedChatIDs:     []int64{42},
			expectedPriority:    PriorityMedium,
			expectedOK:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			filter := NewFilter(FilterConfig{})
			sum := mocks.NewMockSummarizer(ctrl)
			sum.EXPECT().
				Summarize(gomock.Any(), tt.update.Description).
				Return(tt.summary, nil)
			p := newTestProcessor(filter, sum, tt.threshold)

			out, ok := p.Process(context.Background(), tt.update)

			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedDescription, out.Description)
			assert.NotEqual(t, tt.update.Description, out.Description)
			assert.Equal(t, tt.expectedChatIDs, out.TgChatIDs)
			assert.Equal(t, tt.expectedPriority, out.Priority)
		})
	}
}

func TestProcessor_ShortTextNotSummarized(t *testing.T) {
	tests := []struct {
		name                string
		update              domain.RawUpdate
		threshold           int
		expectedDescription string
		expectedOK          bool
	}{
		{
			name: "does not summarize short text",
			update: domain.RawUpdate{
				EventID:     "e2",
				URL:         "https://example.com",
				Description: "compact update",
				Author:      "alice",
				TgChatIDs:   []int64{1, 2},
			},
			threshold:           500,
			expectedDescription: "compact update",
			expectedOK:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			filter := NewFilter(FilterConfig{})
			sum := mocks.NewMockSummarizer(ctrl)
			p := newTestProcessor(filter, sum, tt.threshold)

			out, ok := p.Process(context.Background(), tt.update)

			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedDescription, out.Description)
		})
	}
}

func TestProcessor_SummarizerErrorFallsBackToOriginal(t *testing.T) {
	long := strings.Repeat("b", 600)
	tests := []struct {
		name                string
		update              domain.RawUpdate
		threshold           int
		summaryErr          error
		expectedDescription string
		expectedOK          bool
	}{
		{
			name: "falls back to original on summarizer error",
			update: domain.RawUpdate{
				EventID:     "e3",
				URL:         "https://example.com",
				Description: long,
				TgChatIDs:   []int64{99},
			},
			threshold:           500,
			summaryErr:          errors.New("boom"),
			expectedDescription: long,
			expectedOK:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			filter := NewFilter(FilterConfig{})
			sum := mocks.NewMockSummarizer(ctrl)
			sum.EXPECT().
				Summarize(gomock.Any(), tt.update.Description).
				Return("", tt.summaryErr)
			p := newTestProcessor(filter, sum, tt.threshold)

			out, ok := p.Process(context.Background(), tt.update)

			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedDescription, out.Description)
		})
	}
}

func TestProcessor_PrioritizesUpdate(t *testing.T) {
	tests := []struct {
		name             string
		prioritizer      PrioritizerConfig
		update           domain.RawUpdate
		expectedPriority string
		expectedOK       bool
	}{
		{
			name: "prioritizes high update",
			prioritizer: PrioritizerConfig{
				HighKeywords: []string{"critical"},
				LowKeywords:  []string{"typo"},
			},
			update: domain.RawUpdate{
				Description: "critical security patch",
				Author:      "alice",
			},
			expectedPriority: PriorityHigh,
			expectedOK:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewFilter(FilterConfig{})
			prioritizer := NewPrioritizer(tt.prioritizer)
			p := NewProcessor(filter, nil, 0, prioritizer)

			out, ok := p.Process(context.Background(), tt.update)

			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedPriority, out.Priority)
		})
	}
}

package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrioritizer_Prioritize(t *testing.T) {
	tests := []struct {
		name     string
		config   PrioritizerConfig
		text     string
		expected string
	}{
		{
			name: "high priority keyword",
			config: PrioritizerConfig{
				HighKeywords: []string{"critical", "urgent"},
				LowKeywords:  []string{"minor", "typo"},
			},
			text:     "critical bug fix in production",
			expected: PriorityHigh,
		},
		{
			name: "medium priority without keywords",
			config: PrioritizerConfig{
				HighKeywords: []string{"critical", "urgent"},
				LowKeywords:  []string{"minor", "typo"},
			},
			text:     "regular update about new feature",
			expected: PriorityMedium,
		},
		{
			name: "low priority keyword",
			config: PrioritizerConfig{
				HighKeywords: []string{"critical", "urgent"},
				LowKeywords:  []string{"minor", "typo"},
			},
			text:     "fix typo in readme",
			expected: PriorityLow,
		},
		{
			name: "high priority takes precedence over low",
			config: PrioritizerConfig{
				HighKeywords: []string{"critical"},
				LowKeywords:  []string{"typo"},
			},
			text:     "critical typo fix",
			expected: PriorityHigh,
		},
		{
			name: "high priority keyword is case insensitive",
			config: PrioritizerConfig{
				HighKeywords: []string{"URGENT"},
				LowKeywords:  []string{"Chore"},
			},
			text:     "this is UrGeNt news",
			expected: PriorityHigh,
		},
		{
			name: "low priority keyword is case insensitive",
			config: PrioritizerConfig{
				HighKeywords: []string{"URGENT"},
				LowKeywords:  []string{"Chore"},
			},
			text:     "small Chore task",
			expected: PriorityLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPrioritizer(tt.config)

			actual := p.Prioritize(tt.text)

			assert.Equal(t, tt.expected, actual)
		})
	}
}

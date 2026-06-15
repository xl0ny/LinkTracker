package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

func TestFilter_AllowsAndDecide(t *testing.T) {
	tests := []struct {
		name         string
		config       FilterConfig
		update       domain.RawUpdate
		expected     bool
		expectedSkip SkipReason
		checkDecide  bool
	}{
		{
			name:   "blocks stop word",
			config: FilterConfig{StopWords: []string{"spam", "ads"}, MinLength: 10},
			update: domain.RawUpdate{
				Description: "this update is spam, please ignore",
				Author:      "alice",
			},
			expected:     false,
			expectedSkip: SkipStopWord,
			checkDecide:  true,
		},
		{
			name:   "stop word is case insensitive",
			config: FilterConfig{StopWords: []string{"PROMO"}, MinLength: 5},
			update: domain.RawUpdate{
				Description: "Limited Time PrOmO inside the box",
				Author:      "alice",
			},
			expected: false,
		},
		{
			name: "blocks excluded author",
			config: FilterConfig{
				ExcludedAuthors: []string{"bot-user", "automation"},
				MinLength:       10,
			},
			update: domain.RawUpdate{
				Description: "real human update with enough length",
				Author:      "Bot-User",
			},
			expected:     false,
			expectedSkip: SkipExcludedAuthor,
			checkDecide:  true,
		},
		{
			name:   "blocks text shorter than minimum",
			config: FilterConfig{MinLength: 20},
			update: domain.RawUpdate{
				Description: "too short",
				Author:      "alice",
			},
			expected:     false,
			expectedSkip: SkipMinLength,
			checkDecide:  true,
		},
		{
			name:   "minimum length counts runes",
			config: FilterConfig{MinLength: 10},
			update: domain.RawUpdate{
				Description: strings.Repeat("ж", 9),
				Author:      "alice",
			},
			expected: false,
		},
		{
			name: "allows valid update",
			config: FilterConfig{
				StopWords:       []string{"spam"},
				ExcludedAuthors: []string{"bot"},
				MinLength:       10,
			},
			update: domain.RawUpdate{
				Description: "everything looks fine here",
				Author:      "alice",
			},
			expected:     true,
			expectedSkip: SkipNone,
			checkDecide:  true,
		},
		{
			name:     "empty config allows update",
			config:   FilterConfig{},
			update:   domain.RawUpdate{Description: "x"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFilter(tt.config)

			assert.Equal(t, tt.expected, f.Allows(tt.update))
			if tt.checkDecide {
				assert.Equal(t, tt.expectedSkip, f.Decide(tt.update))
			}
		})
	}
}

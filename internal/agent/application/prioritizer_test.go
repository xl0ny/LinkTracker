package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrioritizer_HighKeyword(t *testing.T) {
	p := NewPrioritizer(PrioritizerConfig{
		HighKeywords: []string{"critical", "urgent"},
		LowKeywords:  []string{"minor", "typo"},
	})
	require.Equal(t, PriorityHigh, p.Prioritize("critical bug fix in production"))
}

func TestPrioritizer_MediumNoKeywords(t *testing.T) {
	p := NewPrioritizer(PrioritizerConfig{
		HighKeywords: []string{"critical", "urgent"},
		LowKeywords:  []string{"minor", "typo"},
	})
	require.Equal(t, PriorityMedium, p.Prioritize("regular update about new feature"))
}

func TestPrioritizer_LowKeyword(t *testing.T) {
	p := NewPrioritizer(PrioritizerConfig{
		HighKeywords: []string{"critical", "urgent"},
		LowKeywords:  []string{"minor", "typo"},
	})
	require.Equal(t, PriorityLow, p.Prioritize("fix typo in readme"))
}

func TestPrioritizer_HighTakesPrecedenceOverLow(t *testing.T) {
	p := NewPrioritizer(PrioritizerConfig{
		HighKeywords: []string{"critical"},
		LowKeywords:  []string{"typo"},
	})
	require.Equal(t, PriorityHigh, p.Prioritize("critical typo fix"))
}

func TestPrioritizer_CaseInsensitive(t *testing.T) {
	p := NewPrioritizer(PrioritizerConfig{
		HighKeywords: []string{"URGENT"},
		LowKeywords:  []string{"Chore"},
	})
	require.Equal(t, PriorityHigh, p.Prioritize("this is UrGeNt news"))
	require.Equal(t, PriorityLow, p.Prioritize("small Chore task"))
}

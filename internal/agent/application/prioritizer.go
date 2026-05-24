package application

import "strings"

const (
	PriorityHigh   = "HIGH"
	PriorityMedium = "MEDIUM"
	PriorityLow    = "LOW"
)

type PrioritizerConfig struct {
	HighKeywords []string
	LowKeywords  []string
}

type Prioritizer struct {
	highKeywords []string
	lowKeywords  []string
}

func NewPrioritizer(cfg PrioritizerConfig) *Prioritizer {
	return &Prioritizer{
		highKeywords: normalizeKeywords(cfg.HighKeywords),
		lowKeywords:  normalizeKeywords(cfg.LowKeywords),
	}
}

func normalizeKeywords(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.TrimSpace(strings.ToLower(w))
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func (p *Prioritizer) Prioritize(text string) string {
	lower := strings.ToLower(text)
	for _, w := range p.highKeywords {
		if strings.Contains(lower, w) {
			return PriorityHigh
		}
	}
	for _, w := range p.lowKeywords {
		if strings.Contains(lower, w) {
			return PriorityLow
		}
	}
	return PriorityMedium
}

const (
	priorityRankHigh   = 3
	priorityRankMedium = 2
	priorityRankLow    = 1
)

func priorityRank(priority string) int {
	switch priority {
	case PriorityHigh:
		return priorityRankHigh
	case PriorityMedium:
		return priorityRankMedium
	case PriorityLow:
		return priorityRankLow
	default:
		return 0
	}
}

func maxPriority(a, b string) string {
	if priorityRank(a) >= priorityRank(b) {
		return a
	}
	return b
}

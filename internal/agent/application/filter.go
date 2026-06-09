package application

import (
	"strings"
	"unicode/utf8"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type SkipReason int

const (
	SkipNone SkipReason = iota
	SkipStopWord
	SkipExcludedAuthor
	SkipMinLength
)

func (s SkipReason) String() string {
	switch s {
	case SkipNone:
		return "none"
	case SkipStopWord:
		return "stop_word"
	case SkipExcludedAuthor:
		return "excluded_author"
	case SkipMinLength:
		return "min_length"
	default:
		return "unknown"
	}
}

type FilterConfig struct {
	StopWords       []string
	ExcludedAuthors []string
	MinLength       int
}

type Filter struct {
	stopWords       []string
	excludedAuthors map[string]struct{}
	minLength       int
}

func NewFilter(cfg FilterConfig) *Filter {
	stop := make([]string, 0, len(cfg.StopWords))
	for _, w := range cfg.StopWords {
		w = strings.TrimSpace(strings.ToLower(w))
		if w != "" {
			stop = append(stop, w)
		}
	}
	excl := make(map[string]struct{}, len(cfg.ExcludedAuthors))
	for _, a := range cfg.ExcludedAuthors {
		a = strings.TrimSpace(strings.ToLower(a))
		if a != "" {
			excl[a] = struct{}{}
		}
	}
	return &Filter{stopWords: stop, excludedAuthors: excl, minLength: cfg.MinLength}
}

func (f *Filter) Decide(u domain.RawUpdate) SkipReason {
	if f.minLength > 0 && utf8.RuneCountInString(u.Description) < f.minLength {
		return SkipMinLength
	}
	if u.Author != "" {
		if _, ok := f.excludedAuthors[strings.ToLower(u.Author)]; ok {
			return SkipExcludedAuthor
		}
	}
	if len(f.stopWords) > 0 {
		lower := strings.ToLower(u.Description)
		for _, w := range f.stopWords {
			if strings.Contains(lower, w) {
				return SkipStopWord
			}
		}
	}
	return SkipNone
}

func (f *Filter) Allows(u domain.RawUpdate) bool {
	return f.Decide(u) == SkipNone
}

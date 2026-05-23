package summarizer

import (
	"context"
	"errors"
)

type Stub struct {
	threshold int
}

func NewStub(threshold int) *Stub {
	return &Stub{threshold: threshold}
}

func (s *Stub) Summarize(_ context.Context, text string) (string, error) {
	if s.threshold <= 0 {
		return "", errors.New("stub-summarizer: non-positive threshold")
	}
	runes := []rune(text)
	if len(runes) <= s.threshold {
		return text, nil
	}
	return string(runes[:s.threshold]) + "...", nil
}

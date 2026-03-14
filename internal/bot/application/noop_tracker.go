package application

import (
	"context"
	"errors"
)

var errScrapperNotConfigured = errors.New("scrapper not configured")

type noopTracker struct{}

func (noopTracker) RegisterChat(_ context.Context, _ int64) error {
	return nil
}

func (noopTracker) AddLink(_ context.Context, _ int64, _ string, _ []string) error {
	return errScrapperNotConfigured
}

func (noopTracker) RemoveLink(_ context.Context, _ int64, _ string) error {
	return errScrapperNotConfigured
}

func (noopTracker) ListLinks(_ context.Context, _ int64, _ string) ([]LinkInfo, error) {
	return nil, errScrapperNotConfigured
}

func NewNoopTracker() LinkTracker {
	return noopTracker{}
}

package application

import "context"

type noopTracker struct{}

func NewNoopTracker() Tracker {
	return &noopTracker{}
}

func (*noopTracker) RegisterChat(_ context.Context, _ int64) error {
	return nil
}

func (*noopTracker) AddLink(_ context.Context, _ int64, _ string, _ []string) error {
	return nil
}

func (*noopTracker) RemoveLink(_ context.Context, _ int64, _ string) error {
	return nil
}

func (*noopTracker) ListLinks(_ context.Context, _ int64, _ string) ([]LinkInfo, error) {
	return []LinkInfo{}, nil
}

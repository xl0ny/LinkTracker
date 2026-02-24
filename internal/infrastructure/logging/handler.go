package logging

import (
	"context"
	"log/slog"
	"os"
)

// Handler направляет Info/Debug в stdout, Warn/Error — в stderr.
type Handler struct {
	info slog.Handler
	err  slog.Handler
}

// NewHandler создаёт handler с разделением по потокам.
func NewHandler(level slog.Level) *Handler {
	opts := &slog.HandlerOptions{Level: level}
	return &Handler{
		info: slog.NewTextHandler(os.Stdout, opts),
		err:  slog.NewTextHandler(os.Stderr, opts),
	}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.info.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level <= slog.LevelInfo {
		return h.info.Handle(ctx, r)
	}
	return h.err.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		info: h.info.WithAttrs(attrs),
		err:  h.err.WithAttrs(attrs),
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		info: h.info.WithGroup(name),
		err:  h.err.WithGroup(name),
	}
}

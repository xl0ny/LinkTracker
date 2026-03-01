package logging

import (
	"context"
	"log/slog"
	"os"
)

type Logger struct {
	info slog.Handler
	err  slog.Handler
}

func NewLogger(level slog.Level) *Logger {
	opts := &slog.HandlerOptions{Level: level}
	return &Logger{
		info: slog.NewTextHandler(os.Stdout, opts),
		err:  slog.NewTextHandler(os.Stderr, opts),
	}
}

func (h *Logger) Enabled(ctx context.Context, level slog.Level) bool {
	return h.info.Enabled(ctx, level)
}

func (h *Logger) Handle(ctx context.Context, r slog.Record) error {
	if r.Level <= slog.LevelInfo {
		return h.info.Handle(ctx, r)
	}
	return h.err.Handle(ctx, r)
}

func (h *Logger) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Logger{
		info: h.info.WithAttrs(attrs),
		err:  h.err.WithAttrs(attrs),
	}
}

func (h *Logger) WithGroup(name string) slog.Handler {
	return &Logger{
		info: h.info.WithGroup(name),
		err:  h.err.WithGroup(name),
	}
}

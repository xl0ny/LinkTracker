package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Handler struct {
	info slog.Handler
	err  slog.Handler
}

func NewHandler(level slog.Level) *Handler {
	opts := &slog.HandlerOptions{Level: level}
	return &Handler{
		info: slog.NewTextHandler(os.Stdout, opts),
		err:  slog.NewTextHandler(os.Stderr, opts),
	}
}

func LevelFromMode(mode string) slog.Level {
	if strings.EqualFold(mode, "DEBUG") {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.info.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level <= slog.LevelInfo {
		if err := h.info.Handle(ctx, r); err != nil {
			return fmt.Errorf("info handle: %w", err)
		}
		return nil
	}
	if err := h.err.Handle(ctx, r); err != nil {
		return fmt.Errorf("err handle: %w", err)
	}
	return nil
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

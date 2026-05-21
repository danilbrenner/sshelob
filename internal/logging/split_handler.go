package logging

import (
	"context"
	"log/slog"
)

// SplitHandler routes low-severity logs to stdout and errors to stderr.
type SplitHandler struct {
	stdout slog.Handler
	stderr slog.Handler
}

func NewSplitHandler(stdout slog.Handler, stderr slog.Handler) *SplitHandler {
	return &SplitHandler{stdout: stdout, stderr: stderr}
}

func (h *SplitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level >= slog.LevelError {
		return h.stderr.Enabled(ctx, level)
	}
	return h.stdout.Enabled(ctx, level)
}

func (h *SplitHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= slog.LevelError {
		return h.stderr.Handle(ctx, record)
	}
	return h.stdout.Handle(ctx, record)
}

func (h *SplitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SplitHandler{
		stdout: h.stdout.WithAttrs(attrs),
		stderr: h.stderr.WithAttrs(attrs),
	}
}

func (h *SplitHandler) WithGroup(name string) slog.Handler {
	return &SplitHandler{
		stdout: h.stdout.WithGroup(name),
		stderr: h.stderr.WithGroup(name),
	}
}

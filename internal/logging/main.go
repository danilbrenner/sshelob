package logging

import (
	"log/slog"
	"os"
)

func Config() (slog.Handler, slog.Handler) {
	isDev := os.Getenv("ENVIRONMENT") == "dev"

	var stdoutHandler, stderrHandler slog.Handler
	switch {
	case isDev:
		// Human-readable format for development
		stdoutHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     slog.LevelInfo,
			AddSource: true,
		})
		stderrHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level:     slog.LevelError,
			AddSource: true,
		})
	default:
		// Structured JSON format for production
		stdoutHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		stderrHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelError,
		})
	}
	return stdoutHandler, stderrHandler
}

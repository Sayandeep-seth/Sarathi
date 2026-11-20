// Package logger builds the structured logger used across the backend.
package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON logger in production and a text logger elsewhere.
func New(env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	if env == "production" {
		opts.Level = slog.LevelInfo
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

package sprig

import (
	"log/slog"
	"os"
)

// NewLogger creates a structured JSON logger using slog.
func NewLogger(level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler)
}

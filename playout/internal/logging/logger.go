// Package logging provides a thin wrapper around log/slog for the Engine.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New creates a structured logger from the given level and format strings.
//
//   - level:  "debug" | "info" | "warn" | "error"
//   - format: "json" | "text"
//   - w:      destination writer; nil defaults to os.Stderr
func New(level, format string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}

	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(handler)
}

// With returns a child logger with "component" set to the given name.
// Every record emitted by the returned logger will carry the component field,
// matching the log schema defined in the observability spec.
func With(logger *slog.Logger, component string) *slog.Logger {
	return logger.With("component", component)
}

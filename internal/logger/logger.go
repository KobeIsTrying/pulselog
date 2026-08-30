package logger

import (
	"log/slog"
	"os"
	"strings"
)

func New(service, env, level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	h := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(h).With(
		"service", service,
		"env", env,
	)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

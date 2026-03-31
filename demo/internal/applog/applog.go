// Package applog configures structured JSON logging (part 1 — log → stdout for later shipping to Elasticsearch).
package applog

import (
	"log/slog"
	"os"
	"strings"
)

const envLogLevel = "LOG_LEVEL"

// Init sets the default slog logger to JSON on stdout with service.name on every line.
// LOG_LEVEL: debug, info (default), warn, error.
func Init(serviceName string) {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "unknown-service"
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv(envLogLevel)),
	})
	lg := slog.New(h).With(slog.String("service.name", serviceName))
	slog.SetDefault(lg)
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
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

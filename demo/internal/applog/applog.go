// Package applog configures structured JSON logging (part 1 — log → stdout for later shipping to Elasticsearch).
package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

const envLogLevel = "LOG_LEVEL"

// envLogFile duplicates JSON logs to a file (same lines as stdout) so Filebeat can tail
// host paths when running with `go run` (see demo/ops/filebeat filestream input).
const envLogFile = "LOG_FILE"

// Init sets the default slog logger to JSON on stdout with service.name on every line.
// LOG_LEVEL: debug, info (default), warn, error.
// Optional LOG_FILE: append the same JSON lines to that path (e.g. for local dev + Filebeat).
func Init(serviceName string) {
	if strings.TrimSpace(serviceName) == "" {
		serviceName = "unknown-service"
	}
	opts := &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv(envLogLevel)),
	}
	out := io.Writer(os.Stdout)
	if p := strings.TrimSpace(os.Getenv(envLogFile)); p != "" {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "applog: open %s %q: %v\n", envLogFile, p, err)
		} else {
			out = io.MultiWriter(os.Stdout, f)
		}
	}
	h := slog.NewJSONHandler(out, opts)
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

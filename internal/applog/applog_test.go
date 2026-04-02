package applog

import (
	"log/slog"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"DEBUG", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"bogus", slog.LevelInfo},
	}
	for _, tc := range tests {
		if got := parseLogLevel(tc.in); got != tc.want {
			t.Fatalf("parseLogLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

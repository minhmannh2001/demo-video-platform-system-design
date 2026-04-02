package tracing

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseTraceSampleRatio(t *testing.T) {
	tests := []struct {
		raw  string
		def  float64
		want float64
	}{
		{"", 0.1, 0.1},
		{"  ", 0.2, 0.2},
		{"0.25", 0.1, 0.25},
		{"1", 0.1, 1},
		{"0", 0.1, 0},
		{"not-a-float", 0.33, 0.33},
		{"2", 0.5, 0.5},
		{"-0.1", 0.5, 0.5},
	}
	for _, tc := range tests {
		name := tc.raw
		if name == "" {
			name = "(empty)"
		}
		t.Run(fmt.Sprintf("%s_def%v", name, tc.def), func(t *testing.T) {
			got := parseTraceSampleRatio(tc.raw, tc.def)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSamplingSummary(t *testing.T) {
	t.Setenv(envTraceSamplingEnabled, "")
	t.Setenv(envTraceSampleRatio, "")
	if s := SamplingSummary(); !strings.Contains(s, "always_on") {
		t.Fatalf("want always_on, got %q", s)
	}
	t.Setenv(envTraceSamplingEnabled, "true")
	t.Setenv(envTraceSampleRatio, "0.25")
	if s := SamplingSummary(); !strings.Contains(s, "0.25") || !strings.Contains(s, "parent_based") {
		t.Fatalf("want ratio summary, got %q", s)
	}
}

package tracing

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	// envTraceSamplingEnabled: when "true", root spans use trace-ID ratio sampling;
	// children follow the parent (W3C sampled flag). When unset/false, every trace is kept.
	envTraceSamplingEnabled = "OTEL_TRACE_SAMPLING_ENABLED"
	// envTraceSampleRatio: fraction in [0,1] for root spans when sampling is enabled (default 0.1).
	envTraceSampleRatio = "OTEL_TRACE_SAMPLE_RATIO"
)

const defaultSampleRatio = 0.1

// parseTraceSampleRatio parses OTEL_TRACE_SAMPLE_RATIO; invalid or empty returns defaultFrac.
func parseTraceSampleRatio(raw string, defaultFrac float64) float64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return defaultFrac
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 || v > 1 {
		return defaultFrac
	}
	return v
}

func tracerSampler() sdktrace.Sampler {
	if strings.TrimSpace(os.Getenv(envTraceSamplingEnabled)) != "true" {
		return sdktrace.AlwaysSample()
	}
	ratio := parseTraceSampleRatio(os.Getenv(envTraceSampleRatio), defaultSampleRatio)
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
}

// SamplingSummary describes the effective sampling mode (for startup logs).
// Ratio sampling is only active when OTEL_TRACE_SAMPLING_ENABLED=true.
func SamplingSummary() string {
	if strings.TrimSpace(os.Getenv(envTraceSamplingEnabled)) != "true" {
		return fmt.Sprintf("always_on (export all traces; set %s=true for ratio sampling)", envTraceSamplingEnabled)
	}
	r := parseTraceSampleRatio(os.Getenv(envTraceSampleRatio), defaultSampleRatio)
	return fmt.Sprintf("parent_based trace_id_ratio=%g (%s)", r, envTraceSampleRatio)
}

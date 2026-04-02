package tracing

// InitConfig configures tracing bootstrap for API or worker binaries.
type InitConfig struct {
	// ServiceName is the OpenTelemetry service.name (e.g. video-api, video-worker).
	// If empty, OTEL_SERVICE_NAME is used, then "video-app".
	ServiceName string
	// EnableHTTPInstrumentation registers otelhttp when true (HTTP servers only).
	EnableHTTPInstrumentation bool
}

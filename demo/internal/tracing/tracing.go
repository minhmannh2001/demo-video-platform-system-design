// Package tracing configures OpenTelemetry for the API (OTLP → Jaeger in local dev).
package tracing

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	envEnabled  = "OTEL_TRACING_ENABLED"
	envEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"
)

var httpInstrumented bool

// Enabled reports whether OTLP export and HTTP instrumentation are active.
func Enabled() bool { return httpInstrumented }

// Init sets up the global TracerProvider and OTLP HTTP exporter when
// OTEL_TRACING_ENABLED=true. Returns a shutdown function (safe to call when init skipped).
func Init(ctx context.Context, cfg InitConfig) (shutdown func(context.Context) error, err error) {
	if strings.TrimSpace(os.Getenv(envEnabled)) != "true" {
		return func(context.Context) error { return nil }, nil
	}

	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	}
	if serviceName == "" {
		serviceName = "video-app"
	}

	endpoint := strings.TrimSpace(os.Getenv(envEndpoint))
	if endpoint == "" {
		endpoint = "http://localhost:4318"
	}

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if cfg.EnableHTTPInstrumentation {
		httpInstrumented = true
	}
	return tp.Shutdown, nil
}

// httpSpanName returns a root span name Jaeger can show per request (method + path).
// Chi has not matched routes yet when otelhttp runs, so we use the URL path, not a route template.
func httpSpanName(_ string, r *http.Request) string {
	if r == nil {
		return "HTTP"
	}
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	return r.Method + " " + path
}

// WrapHandler adds an HTTP server span for every request (step 3: full API).
// OPTIONS is skipped to avoid one trace per CORS preflight from the browser.
// When tracing is disabled, returns h unchanged.
func WrapHandler(h http.Handler) http.Handler {
	if !httpInstrumented {
		return h
	}
	return otelhttp.NewHandler(h, "http.server",
		otelhttp.WithSpanNameFormatter(httpSpanName),
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.Method != http.MethodOptions
		}),
	)
}

// ShutdownTimeout is the default grace period for exporter flush on process exit.
const ShutdownTimeout = 5 * time.Second

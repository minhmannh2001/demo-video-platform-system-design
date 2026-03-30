package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "video-api"

// Tracer returns the application tracer (same logical scope as HTTP spans).
func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// Start opens a child span under ctx (e.g. the HTTP span from otelhttp).
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// Finish ends the span; if err is non-nil, records it and sets span status to Error.
func Finish(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

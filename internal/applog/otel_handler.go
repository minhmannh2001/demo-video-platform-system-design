package applog

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type oTelContextHandler struct {
	next slog.Handler
}

func newOTelContextHandler(next slog.Handler) slog.Handler {
	return &oTelContextHandler{next: next}
}

// WrapHandlerWithOTelTrace wraps h so each record includes trace_id/span_id when ctx carries a valid OTel span.
// Exposed for tests and custom slog setups; Init uses this internally.
func WrapHandlerWithOTelTrace(h slog.Handler) slog.Handler {
	return newOTelContextHandler(h)
}

func (h *oTelContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *oTelContextHandler) Handle(ctx context.Context, r slog.Record) error {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.next.Handle(ctx, r)
}

func (h *oTelContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &oTelContextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *oTelContextHandler) WithGroup(name string) slog.Handler {
	return &oTelContextHandler{next: h.next.WithGroup(name)}
}

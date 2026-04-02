package applog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestOTelContextHandler_addsTraceFieldsWhenSpanValid(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	var buf bytes.Buffer
	h := WrapHandlerWithOTelTrace(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h)

	lg.InfoContext(ctx, "hello", "k", "v")

	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got := m["msg"]; got != "hello" {
		t.Fatalf("msg = %v", got)
	}
	if got := m["trace_id"]; got != traceID.String() {
		t.Fatalf("trace_id = %v, want %s", got, traceID.String())
	}
	if got := m["span_id"]; got != spanID.String() {
		t.Fatalf("span_id = %v, want %s", got, spanID.String())
	}
}

func TestOTelContextHandler_noTraceFieldsWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	h := WrapHandlerWithOTelTrace(slog.NewJSONHandler(&buf, nil))
	lg := slog.New(h)

	lg.InfoContext(context.Background(), "hello", "k", "v")

	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("json: %v", err)
	}
	if _, ok := m["trace_id"]; ok {
		t.Fatalf("unexpected trace_id: %v", m["trace_id"])
	}
	if _, ok := m["span_id"]; ok {
		t.Fatalf("unexpected span_id: %v", m["span_id"])
	}
}

func TestOTelContextHandler_WithAttrsPreservesWrapper(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("fedcba9876543210fedcba9876543210")
	spanID, _ := trace.SpanIDFromHex("aabbccddeeff0011")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	h := WrapHandlerWithOTelTrace(base).WithAttrs([]slog.Attr{slog.String("service.name", "test")})
	lg := slog.New(h)

	lg.InfoContext(ctx, "ping")

	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, `"service.name":"test"`) {
		t.Fatalf("missing service.name: %s", line)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatal(err)
	}
	if m["trace_id"] != traceID.String() || m["span_id"] != spanID.String() {
		t.Fatalf("trace fields: %+v", m)
	}
}

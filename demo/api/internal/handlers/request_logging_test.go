package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"video-platform/demo/internal/applog"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

func TestRequestLogMiddleware(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := chi.NewRouter()
	r.Use(RequestLogMiddleware())
	r.Get("/videos/{id}", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	})

	req := httptest.NewRequest(http.MethodGet, "/videos/abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	reqID := rec.Header().Get(requestIDHeader)
	if _, err := uuid.Parse(reqID); err != nil {
		t.Fatalf("invalid request id %q: %v", reqID, err)
	}

	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &event); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}
	if got := event["msg"]; got != "request completed" {
		t.Fatalf("msg = %v", got)
	}
	if got := event["level"]; got != "ERROR" {
		t.Fatalf("level = %v, want ERROR for 5xx", got)
	}
	if got := event["method"]; got != http.MethodGet {
		t.Fatalf("method = %v", got)
	}
	if got := event["path"]; got != "/videos/{id}" {
		t.Fatalf("path = %v", got)
	}
	if got := event["status"]; got != float64(http.StatusInternalServerError) {
		t.Fatalf("status = %v", got)
	}
	if got := event["request_id"]; got != reqID {
		t.Fatalf("request_id = %v, want %v", got, reqID)
	}
	if _, ok := event["duration_ms"]; !ok {
		t.Fatal("missing duration_ms")
	}
}

func TestRequestLogMiddleware_includesTraceWhenContextHasSpan(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	spanID, _ := trace.SpanIDFromHex("bbbbbbbbbbbbbbbb")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(applog.WrapHandlerWithOTelTrace(slog.NewJSONHandler(&logs, nil))))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := chi.NewRouter()
	r.Use(RequestLogMiddleware())
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &event); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}
	if event["trace_id"] != traceID.String() {
		t.Fatalf("trace_id = %v, want %s", event["trace_id"], traceID.String())
	}
	if event["span_id"] != spanID.String() {
		t.Fatalf("span_id = %v, want %s", event["span_id"], spanID.String())
	}
}

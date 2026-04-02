package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"video-platform/internal/applog"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

type fakeHijackResponse struct {
	*httptest.ResponseRecorder
	hijackCalled bool
}

func (f *fakeHijackResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	f.hijackCalled = true
	return nil, nil, errors.New("stub hijack")
}

func TestStatusRecorder_delegatesHijackToUnderlying(t *testing.T) {
	under := &fakeHijackResponse{ResponseRecorder: httptest.NewRecorder()}
	sr := &statusRecorder{ResponseWriter: under, status: http.StatusOK}
	_, _, err := sr.Hijack()
	if err == nil || err.Error() != "stub hijack" {
		t.Fatalf("Hijack: %v", err)
	}
	if !under.hijackCalled {
		t.Fatal("expected delegated Hijack on underlying writer")
	}
}

func TestRequestLogMiddleware_preservesHijackerForInnerHandler(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	r := chi.NewRouter()
	r.Use(RequestLogMiddleware())
	r.Get("/ws", func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("inner handler must see http.Hijacker (e.g. for WebSocket upgrade)")
			return
		}
		_, _, _ = hj.Hijack()
	})

	under := &fakeHijackResponse{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.ServeHTTP(under, req)

	if !under.hijackCalled {
		t.Fatal("Hijack was not delegated through statusRecorder to the real connection writer")
	}
}

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

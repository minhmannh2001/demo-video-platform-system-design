package handlers

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(p)
}

// Hijack delegates to the underlying ResponseWriter so WebSocket upgrades work
// (gorilla/websocket requires http.Hijacker on the writer passed to Upgrade).
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("http.Hijacker not supported by wrapped ResponseWriter")
	}
	return hj.Hijack()
}

// Flush delegates to the underlying Flusher when present (optional for streaming).
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// RequestLogMiddleware logs one line per HTTP request with request_id.
func RequestLogMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := uuid.NewString()
			w.Header().Set(requestIDHeader, requestID)

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			pattern := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				pattern = rc.RoutePattern()
			}
			if pattern == "" {
				pattern = r.URL.Path
			}

			attrs := []any{
				"method", r.Method,
				"path", pattern,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", requestID,
			}
			switch {
			case rec.status >= 500:
				slog.ErrorContext(r.Context(), "request completed", attrs...)
			case rec.status >= 400:
				slog.WarnContext(r.Context(), "request completed", attrs...)
			default:
				slog.InfoContext(r.Context(), "request completed", attrs...)
			}
		})
	}
}

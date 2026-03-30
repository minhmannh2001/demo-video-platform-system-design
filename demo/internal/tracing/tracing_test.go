package tracing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrapHandler_withoutInit_passesThrough(t *testing.T) {
	httpInstrumented = false
	t.Cleanup(func() { httpInstrumented = false })

	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	wrapped := WrapHandler(h)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler not invoked")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

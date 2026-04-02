package tracing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsWebSocketUpgradeRequest(t *testing.T) {
	t.Parallel()
	req := func(h http.Header) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/ws", nil)
		for k, vv := range h {
			for _, v := range vv {
				r.Header.Add(k, v)
			}
		}
		return r
	}
	if !isWebSocketUpgradeRequest(req(http.Header{
		"Upgrade":               {"websocket"},
		"Connection":            {"Upgrade"},
		"Sec-WebSocket-Version": {"13"},
	})) {
		t.Fatal("expected true for typical browser handshake")
	}
	if isWebSocketUpgradeRequest(req(http.Header{
		"Upgrade": {"websocket"},
	})) {
		t.Fatal("expected false without Connection: upgrade")
	}
	if isWebSocketUpgradeRequest(req(http.Header{
		"Connection": {"Upgrade"},
	})) {
		t.Fatal("expected false without Upgrade: websocket")
	}
	if !isWebSocketUpgradeRequest(req(http.Header{
		"Upgrade":    {"Websocket"},
		"Connection": {"keep-alive, Upgrade"},
	})) {
		t.Fatal("expected true when Upgrade token appears in Connection list")
	}
}

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

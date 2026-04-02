package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestServer_postMethod_notAllowed(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestServer_plainGET_returnsUpgradeRequired(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{"http://localhost:5173"}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d Upgrade Required", resp.StatusCode, http.StatusUpgradeRequired)
	}
}

func TestServer_handshake_sendsHello(t *testing.T) {
	// Empty AllowedOrigins: CheckOrigin allows any (dev-friendly; production sets explicit origins).
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)

	wsURL := wsURLFromHTTP(srv.URL, "/")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v (resp=%+v)", err, resp)
	}
	t.Cleanup(func() { _ = conn.Close() })

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json: %v body=%s", err, data)
	}
	if msg["type"] != "hello" {
		t.Fatalf("type = %v, want hello", msg["type"])
	}
	if msg["v"] != float64(1) {
		t.Fatalf("v = %v, want 1", msg["v"])
	}
}

func TestServer_tokenMismatch_beforeUpgrade(t *testing.T) {
	s := New(Config{
		AllowedOrigins: []string{},
		Token:          "secret-token",
	})
	ts := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(ts.Close)

	wsURL := wsURLFromHTTP(ts.URL, "/")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got resp=%v err=%v", resp, err)
	}
}

func TestServer_tokenAccepted(t *testing.T) {
	s := New(Config{
		AllowedOrigins: []string{},
		Token:          "ok",
	})
	ts := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(ts.Close)

	u := wsURLFromHTTP(ts.URL, "/") + "?token=ok"
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestServer_originNotAllowed(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{"http://allowed.example"}})
	ts := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(ts.Close)

	wsURL := wsURLFromHTTP(ts.URL, "/")
	hdr := http.Header{"Origin": {"http://evil.example"}}
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err == nil {
		t.Fatal("expected dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got resp=%v err=%v", resp, err)
	}
}

func wsURLFromHTTP(httpURL, path string) string {
	u := strings.TrimPrefix(httpURL, "http:")
	return "ws:" + u + path
}

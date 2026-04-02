package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func readJSONMap(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json: %v body=%s", err, data)
	}
	return msg
}

func TestServer_subscribe_video_and_catalog(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)

	wsURL := wsURLFromHTTP(srv.URL, "/")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	hello := readJSONMap(t, conn)
	if hello["type"] != "hello" {
		t.Fatalf("hello = %v", hello)
	}

	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"vid1"}`)
	sub := readJSONMap(t, conn)
	if sub["type"] != "subscribed" || sub["topic"] != "video:vid1" {
		t.Fatalf("sub = %v", sub)
	}
	if s.Hub().SubscriberCount("video:vid1") != 1 {
		t.Fatalf("hub count = %d", s.Hub().SubscriberCount("video:vid1"))
	}

	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"channel":"uploads"}`)
	sub2 := readJSONMap(t, conn)
	if sub2["type"] != "subscribed" || sub2["topic"] != TopicCatalog {
		t.Fatalf("sub2 = %v", sub2)
	}
	if s.Hub().SubscriberCount(TopicCatalog) != 1 {
		t.Fatalf("catalog subs = %d", s.Hub().SubscriberCount(TopicCatalog))
	}
}

func TestServer_subscribe_idempotent_sameTopic(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial(wsURLFromHTTP(srv.URL, "/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = readJSONMap(t, conn)
	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"x"}`)
	_ = readJSONMap(t, conn)
	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"x"}`)
	again := readJSONMap(t, conn)
	if again["type"] != "subscribed" {
		t.Fatalf("%v", again)
	}
	if s.Hub().SubscriberCount("video:x") != 1 {
		t.Fatalf("hub = %d", s.Hub().SubscriberCount("video:x"))
	}
}

func TestServer_subscribe_subscriptionLimit(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial(wsURLFromHTTP(srv.URL, "/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = readJSONMap(t, conn)
	for i := range MaxSubscriptionsPerConn {
		mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"`+strconv.Itoa(i)+`"}`)
		msg := readJSONMap(t, conn)
		if msg["type"] != "subscribed" {
			t.Fatalf("i=%d msg=%v", i, msg)
		}
	}
	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"overflow"}`)
	errMsg := readJSONMap(t, conn)
	if errMsg["type"] != "error" || errMsg["code"] != ErrSubscriptionLimit {
		t.Fatalf("%v", errMsg)
	}
}

func TestServer_subscribe_rateLimited(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial(wsURLFromHTTP(srv.URL, "/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = readJSONMap(t, conn)
	n := SubscribeRateMaxSubscribe
	for i := range n {
		id := "r" + strconv.Itoa(i)
		mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"`+id+`"}`)
		msg := readJSONMap(t, conn)
		if msg["type"] != "subscribed" {
			t.Fatalf("i=%d %v", i, msg)
		}
		mustWriteJSON(t, conn, `{"type":"unsubscribe","v":1,"video_id":"`+id+`"}`)
		un := readJSONMap(t, conn)
		if un["type"] != "unsubscribed" {
			t.Fatalf("i=%d %v", i, un)
		}
	}
	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"one-too-many"}`)
	errMsg := readJSONMap(t, conn)
	if errMsg["type"] != "error" || errMsg["code"] != ErrRateLimited {
		t.Fatalf("%v", errMsg)
	}
}

func TestServer_unsubscribe(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial(wsURLFromHTTP(srv.URL, "/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = readJSONMap(t, conn)
	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"u1"}`)
	_ = readJSONMap(t, conn)
	mustWriteJSON(t, conn, `{"type":"unsubscribe","v":1,"video_id":"u1"}`)
	un := readJSONMap(t, conn)
	if un["type"] != "unsubscribed" || un["topic"] != "video:u1" {
		t.Fatalf("%v", un)
	}
	if s.Hub().SubscriberCount("video:u1") != 0 {
		t.Fatalf("hub = %d", s.Hub().SubscriberCount("video:u1"))
	}
}

func TestServer_hubPublish_deliversToSubscriber(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial(wsURLFromHTTP(srv.URL, "/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = readJSONMap(t, conn)
	mustWriteJSON(t, conn, `{"type":"subscribe","v":1,"video_id":"pub1"}`)
	_ = readJSONMap(t, conn)

	body := []byte(`{"type":"video.updated","v":1,"payload":{"video_id":"pub1","status":"ready"}}`)
	if n := s.Hub().Publish("video:pub1", body); n != 1 {
		t.Fatalf("Publish n=%d want 1", n)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(body) {
		t.Fatalf("got %s want %s", data, body)
	}
}

func TestServer_ping_pong(t *testing.T) {
	s := New(Config{AllowedOrigins: []string{}})
	srv := httptest.NewServer(http.HandlerFunc(s.ServeHTTP))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial(wsURLFromHTTP(srv.URL, "/"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = readJSONMap(t, conn)
	mustWriteJSON(t, conn, `{"type":"ping","v":1}`)
	pong := readJSONMap(t, conn)
	if pong["type"] != "pong" {
		t.Fatalf("%v", pong)
	}
}

func mustWriteJSON(t *testing.T, conn *websocket.Conn, s string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(s)); err != nil {
		t.Fatal(err)
	}
}

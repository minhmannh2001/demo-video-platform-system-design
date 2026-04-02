package wsevents

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"video-platform/demo/internal/ws"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

func wsURLFromHTTP(httpURL, path string) string {
	u := strings.TrimPrefix(httpURL, "http:")
	return "ws:" + u + path
}

func TestBridge_PublishRedis_thenSubscriberHub(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	wsSrv := ws.New(ws.Config{AllowedOrigins: []string{}})
	ts := httptest.NewServer(http.HandlerFunc(wsSrv.ServeHTTP))
	t.Cleanup(ts.Close)

	wsURL := wsURLFromHTTP(ts.URL, "/")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, _ = conn.ReadMessage() // hello
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"subscribe","v":1,"channel":"uploads"}`))
	_, subData, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(subData), "subscribed") {
		t.Fatalf("subscribe ack: %s", subData)
	}

	apiBridge := NewBridge(wsSrv.Hub(), rdb, "ws:ev")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go apiBridge.RunSubscriber(ctx)
	// Allow Redis subscription to register before publish.
	time.Sleep(200 * time.Millisecond)

	workerBridge := NewBridge(nil, rdb, "ws:ev")
	payload := []byte(`{"type":"catalog.invalidate","v":1}`)
	if err := workerBridge.Publish(context.Background(), ws.TopicCatalog, payload); err != nil {
		t.Fatal(err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "catalog.invalidate") {
		t.Fatalf("want catalog.invalidate, got %s", data)
	}
	cancel()
}

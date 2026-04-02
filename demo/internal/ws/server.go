package ws

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	writeWait  = 10 * time.Second
	maxMsgSize = 512 << 10
)

// Config controls the baseline WebSocket endpoint (Phần 1 — nền tảng).
// AllowedOrigins: empty slice allows any Origin (use only in dev/tests).
// Token: non-empty requires query parameter ?token=<exact> before upgrade (constant-time compare).
type Config struct {
	AllowedOrigins []string
	Token          string
}

// Server upgrades HTTP to WebSocket, optional token + Origin checks, hello message, ping keepalive,
// and Phần 2 protocol (subscribe / unsubscribe with limits).
type Server struct {
	cfg      Config
	hub      *Hub
	upgrader websocket.Upgrader
}

// New builds a Server. CheckOrigin uses AllowedOrigins; empty list allows all origins.
func New(cfg Config) *Server {
	allowed := make(map[string]bool)
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}
	s := &Server{cfg: cfg, hub: NewHub()}
	s.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if len(allowed) == 0 {
				return true
			}
			origin := r.Header.Get("Origin")
			return origin != "" && allowed[origin]
		},
	}
	return s
}

// Hub exposes subscription registry for tests and future publishers (Phần 3).
func (s *Server) Hub() *Hub { return s.hub }

// ServeHTTP handles GET /ws (mount path chosen by caller).
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !websocket.IsWebSocketUpgrade(r) {
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("Upgrade", "websocket")
		http.Error(w, "expected WebSocket upgrade request", http.StatusUpgradeRequired)
		return
	}
	if s.cfg.Token != "" {
		q := r.URL.Query().Get("token")
		if subtle.ConstantTimeCompare([]byte(q), []byte(s.cfg.Token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Debug("websocket upgrade failed", "error", err)
		return
	}
	s.serveConn(conn)
}

type connState struct {
	conn     *websocket.Conn
	hub      *Hub
	writeMu  sync.Mutex
	topics   map[string]struct{}
	subTimes []time.Time
	mu       sync.Mutex
}

func (c *connState) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.writeText(b)
}

func (c *connState) writeText(p []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, p)
}

func (c *connState) ping(deadline time.Time) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return c.conn.WriteControl(websocket.PingMessage, nil, deadline)
}

// trySubscribe registers a new distinct topic on this connection (rate + cap apply). Idempotent for same topic.
func (c *connState) trySubscribe(topic string) (ok bool, already bool, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.topics[topic]; exists {
		return true, true, ""
	}
	if len(c.topics) >= MaxSubscriptionsPerConn {
		return false, false, ErrSubscriptionLimit
	}
	now := time.Now()
	cutoff := now.Add(-SubscribeRateWindow)
	i := 0
	for i < len(c.subTimes) && c.subTimes[i].Before(cutoff) {
		i++
	}
	c.subTimes = c.subTimes[i:]
	if len(c.subTimes) >= SubscribeRateMaxSubscribe {
		return false, false, ErrRateLimited
	}
	c.subTimes = append(c.subTimes, now)
	c.topics[topic] = struct{}{}
	return true, false, ""
}

func (c *connState) removeTopic(topic string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.topics[topic]; !ok {
		return false
	}
	delete(c.topics, topic)
	return true
}

func (s *Server) serveConn(conn *websocket.Conn) {
	defer func() { _ = conn.Close() }()
	defer s.hub.UnsubscribeAll(conn)

	st := &connState{
		conn:   conn,
		hub:    s.hub,
		topics: make(map[string]struct{}),
	}

	conn.SetReadLimit(maxMsgSize)
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	if err := st.writeJSON(map[string]any{"type": TypeHello, "v": ProtocolV}); err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pingLoop(ctx, st)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return
		}
		if err := s.handleClientText(st, data); err != nil {
			return
		}
	}
}

func (s *Server) handleClientText(st *connState, data []byte) error {
	m, err := ParseClientText(data)
	if err != nil {
		if IsUnsupportedVersion(err) {
			_ = st.writeJSON(map[string]any{
				"type": TypeError, "v": ProtocolV,
				"code": ErrUnsupportedVersion, "message": "unsupported protocol version",
			})
			return err
		}
		_ = st.writeJSON(map[string]any{
			"type": TypeError, "v": ProtocolV,
			"code": ErrInvalidMessage, "message": "invalid JSON",
		})
		return nil
	}

	switch m.Type {
	case TypePing:
		return st.writeJSON(map[string]any{"type": TypePong, "v": ProtocolV})
	case TypeSubscribe:
		return s.handleSubscribe(st, m)
	case TypeUnsubscribe:
		return s.handleUnsubscribe(st, m)
	default:
		_ = st.writeJSON(map[string]any{
			"type": TypeError, "v": ProtocolV,
			"code": ErrUnknownType, "message": "unknown message type",
		})
		return nil
	}
}

func (s *Server) handleSubscribe(st *connState, m ClientMessage) error {
	topic, err := TopicFromSubscribe(m.VideoID, m.Channel)
	if err != nil {
		code := ErrInvalidSubscribe
		if errors.Is(err, errUnknownChannel) {
			code = ErrUnknownChannel
		}
		slog.Warn("ws_subscribe_invalid", "code", code, "video_id", m.VideoID, "channel", m.Channel, "error", err)
		_ = st.writeJSON(map[string]any{
			"type": TypeError, "v": ProtocolV,
			"code": code, "message": err.Error(),
		})
		return nil
	}
	ok, already, reason := st.trySubscribe(topic)
	if !ok {
		slog.Warn("ws_subscribe_rejected", "topic", topic, "reason", reason)
		_ = st.writeJSON(map[string]any{
			"type": TypeError, "v": ProtocolV,
			"code": reason, "message": "subscribe rejected",
		})
		return nil
	}
	if !already {
		s.hub.Subscribe(st.conn, topic, st.writeText)
	}
	slog.Debug("ws_subscribed", "topic", topic, "already", already)
	return st.writeJSON(map[string]any{
		"type": TypeSubscribed, "v": ProtocolV, "topic": topic,
	})
}

func (s *Server) handleUnsubscribe(st *connState, m ClientMessage) error {
	topic, err := TopicFromSubscribe(m.VideoID, m.Channel)
	if err != nil {
		code := ErrInvalidSubscribe
		if errors.Is(err, errUnknownChannel) {
			code = ErrUnknownChannel
		}
		slog.Warn("ws_unsubscribe_invalid", "code", code, "video_id", m.VideoID, "channel", m.Channel, "error", err)
		_ = st.writeJSON(map[string]any{
			"type": TypeError, "v": ProtocolV,
			"code": code, "message": err.Error(),
		})
		return nil
	}
	if !st.removeTopic(topic) {
		slog.Warn("ws_unsubscribe_rejected", "topic", topic, "reason", "not_subscribed")
		_ = st.writeJSON(map[string]any{
			"type": TypeError, "v": ProtocolV,
			"code": ErrInvalidSubscribe, "message": "not subscribed to topic",
		})
		return nil
	}
	s.hub.Unsubscribe(st.conn, topic)
	slog.Debug("ws_unsubscribed", "topic", topic)
	return st.writeJSON(map[string]any{
		"type": TypeUnsubscribed, "v": ProtocolV, "topic": topic,
	})
}

func pingLoop(ctx context.Context, st *connState) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deadline := time.Now().Add(writeWait)
			if err := st.ping(deadline); err != nil {
				return
			}
		}
	}
}

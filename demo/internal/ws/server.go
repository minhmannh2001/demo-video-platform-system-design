package ws

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
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

// Server upgrades HTTP to WebSocket, optional token + Origin checks, hello message, ping keepalive.
type Server struct {
	cfg      Config
	upgrader websocket.Upgrader
}

// New builds a Server. CheckOrigin uses AllowedOrigins; empty list allows all origins.
func New(cfg Config) *Server {
	allowed := make(map[string]bool)
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}
	s := &Server{cfg: cfg}
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

func (s *Server) serveConn(conn *websocket.Conn) {
	defer func() { _ = conn.Close() }()

	conn.SetReadLimit(maxMsgSize)
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	hello, err := json.Marshal(map[string]any{"type": "hello", "v": 1})
	if err != nil {
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pingLoop(ctx, conn)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

func pingLoop(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Hub tracks which connections are subscribed to which topic and fans out push messages (Phần 3).
type Hub struct {
	mu      sync.RWMutex
	topics  map[string]map[*websocket.Conn]struct{}
	senders map[*websocket.Conn]func([]byte) error
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{
		topics:  make(map[string]map[*websocket.Conn]struct{}),
		senders: make(map[*websocket.Conn]func([]byte) error),
	}
}

// Subscribe adds conn to topic. send must serialize writes for this conn (e.g. connState.writeText). Idempotent per topic.
func (h *Hub) Subscribe(conn *websocket.Conn, topic string, send func([]byte) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if send != nil {
		h.senders[conn] = send
	}
	m, ok := h.topics[topic]
	if !ok {
		m = make(map[*websocket.Conn]struct{})
		h.topics[topic] = m
	}
	m[conn] = struct{}{}
}

// Unsubscribe removes conn from topic.
func (h *Hub) Unsubscribe(conn *websocket.Conn, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	m, ok := h.topics[topic]
	if !ok {
		return
	}
	delete(m, conn)
	if len(m) == 0 {
		delete(h.topics, topic)
	}
}

// UnsubscribeAll removes conn from every topic and drops its sender.
func (h *Hub) UnsubscribeAll(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.senders, conn)
	for topic, m := range h.topics {
		delete(m, conn)
		if len(m) == 0 {
			delete(h.topics, topic)
		}
	}
}

// SubscriberCount returns how many connections are on topic (for tests / metrics).
func (h *Hub) SubscriberCount(topic string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics[topic])
}

// Publish delivers payload to every subscriber of topic (best-effort). Returns number of successful sends.
func (h *Hub) Publish(topic string, payload []byte) int {
	h.mu.RLock()
	m := h.topics[topic]
	conns := make([]*websocket.Conn, 0, len(m))
	for c := range m {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	n := 0
	for _, c := range conns {
		h.mu.RLock()
		send := h.senders[c]
		h.mu.RUnlock()
		if send == nil {
			continue
		}
		if err := send(payload); err != nil {
			continue
		}
		n++
	}
	return n
}

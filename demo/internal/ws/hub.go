package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Hub tracks which connections are subscribed to which topic (for future broadcast in Phần 3).
type Hub struct {
	mu     sync.RWMutex
	topics map[string]map[*websocket.Conn]struct{}
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{topics: make(map[string]map[*websocket.Conn]struct{})}
}

// Subscribe adds conn to topic. Idempotent.
func (h *Hub) Subscribe(conn *websocket.Conn, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
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

// UnsubscribeAll removes conn from every topic.
func (h *Hub) UnsubscribeAll(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
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

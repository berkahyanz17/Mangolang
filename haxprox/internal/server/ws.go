package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"haxprox/internal/store"
)

// Hub tracks connected WebSocket clients and broadcasts new entries to
// all of them. Implements proxy.Broadcaster (Broadcast(*store.Entry)).
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) Broadcast(e *store.Entry) {
	msg, err := json.Marshal(e)
	if err != nil {
		log.Printf("hub: failed to marshal entry: %v", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

// upgrader.CheckOrigin always allows - this is a local dev tool bound to
// 127.0.0.1, not a public multi-tenant service, so origin checking isn't
// meaningful here.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) registerWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}

	s.hub.mu.Lock()
	s.hub.clients[conn] = true
	s.hub.mu.Unlock()

	// Read loop purely to detect disconnects - the client never sends us
	// anything meaningful on this socket, it's push-only.
	defer func() {
		s.hub.mu.Lock()
		delete(s.hub.clients, conn)
		s.hub.mu.Unlock()
		conn.Close()
	}()
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
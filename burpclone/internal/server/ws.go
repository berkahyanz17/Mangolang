package server

import "net/http"

// registerWS handles the WebSocket endpoint the frontend connects to for
// a live feed of new traffic - every time proxy/listener.go or
// proxy/tls.go logs a new Entry to the store, it should also be pushed
// out to all connected WebSocket clients.
//
// TODO(phase 6):
//  1. Upgrade the HTTP connection using your chosen WS library
//     (gorilla/websocket: websocket.Upgrader{}.Upgrade(w, r, nil)).
//  2. Register the new connection in a small broadcast hub (a
//     map[*websocket.Conn]bool + mutex, or a fan-out channel) so the
//     proxy package can push new entries to all connected clients.
//  3. The cleanest wiring: give the Proxy a `chan *store.Entry` in its
//     Options; every time it logs an Entry (phase 3), it also sends it to
//     this channel. A goroutine in this package reads from that channel
//     and broadcasts the JSON-encoded Entry to all connected WS clients.
//  4. Handle client disconnects (read loop erroring out) by removing them
//     from the hub.
func (s *Server) registerWS(w http.ResponseWriter, r *http.Request) {
	panic("TODO: implement registerWS (phase 6)")
}

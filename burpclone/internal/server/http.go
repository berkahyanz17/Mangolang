// Package server implements the web UI's backend: a REST API over the
// history store + interceptor, plus a WebSocket endpoint for live traffic
// (see ws.go). The frontend itself lives in /web (static HTML/JS/CSS).
package server

import (
	"net/http"

	"burpclone/internal/intercept"
	"burpclone/internal/store"
)

type Options struct {
	Store       *store.DB
	Interceptor *intercept.Queue
}

type Server struct {
	opts Options
	mux  *http.ServeMux
}

func New(opts Options) *Server {
	s := &Server{opts: opts, mux: http.NewServeMux()}
	s.routes()
	return s
}

// routes registers all REST endpoints + static file serving + the
// WebSocket handler (registerWS, in ws.go).
//
// Phase 1 status: only static file serving actually works right now (so
// you can open http://.../  and see the placeholder page while the proxy
// itself is being tested). Every /api/* endpoint below is stubbed to
// return 501 until its corresponding phase is implemented - replace each
// stub as you build phase 3-6.
//
// TODO(phase 6): suggested endpoint list -
//
//	GET  /api/history?limit=&offset=      -> s.opts.Store.List(...)
//	GET  /api/history/{id}                -> s.opts.Store.Get(id)
//	GET  /api/intercept                   -> s.opts.Interceptor.List()
//	POST /api/intercept/{id}/forward      -> s.opts.Interceptor.Resolve(id, {Action: Forward, ...edited fields})
//	POST /api/intercept/{id}/drop         -> s.opts.Interceptor.Resolve(id, {Action: Drop})
//	POST /api/intercept/toggle            -> s.opts.Interceptor.SetOn(!current)
//	POST /api/repeater/send               -> repeater.Send(parsed body) -> JSON response
//	GET  /ws                              -> registerWS (live traffic feed)
func (s *Server) routes() {
	notYet := func(phase string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not implemented yet ("+phase+")", http.StatusNotImplemented)
		}
	}

	s.mux.HandleFunc("/api/history", notYet("phase 3"))
	s.mux.HandleFunc("/api/intercept", notYet("phase 4"))
	s.mux.HandleFunc("/api/intercept/toggle", notYet("phase 4"))
	s.mux.HandleFunc("/api/repeater/send", notYet("phase 5"))
	s.mux.HandleFunc("/ws", notYet("phase 6"))

	// Fallback: serve the static frontend from ./web.
	s.mux.Handle("/", http.FileServer(http.Dir("./web")))
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

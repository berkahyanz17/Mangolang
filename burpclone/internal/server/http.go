package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"burpclone/internal/intercept"
	"burpclone/internal/repeater"
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

func (s *Server) handleRepeaterSend(w http.ResponseWriter, r *http.Request) {
	var reqIn struct {
		Method  string `json:"method"`
		URL     string `json:"url"`
		Headers string `json:"headers"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqIn); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if reqIn.Method == "" {
		reqIn.Method = http.MethodGet
	}
	if reqIn.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	resp := repeater.Send(repeater.Request{
		Method:  reqIn.Method,
		URL:     reqIn.URL,
		Headers: reqIn.Headers,
		Body:    []byte(reqIn.Body),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status_code": resp.StatusCode,
		"headers":     resp.Headers,
		"body":        string(resp.Body),
		"duration_ms": resp.Duration.Milliseconds(),
		"error":       resp.Err,
	})
}

func (s *Server) routes() {
	notYet := func(phase string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not implemented yet ("+phase+")", http.StatusNotImplemented)
		}
	}

	s.mux.HandleFunc("GET /api/history", notYet("phase 3 UI wiring"))
	s.mux.HandleFunc("GET /api/intercept", s.handleListIntercept)
	s.mux.HandleFunc("POST /api/intercept/toggle", s.handleToggleIntercept)
	s.mux.HandleFunc("POST /api/intercept/{id}/forward", s.handleForward)
	s.mux.HandleFunc("POST /api/intercept/{id}/drop", s.handleDrop)
	s.mux.HandleFunc("POST /api/repeater/send", s.handleRepeaterSend)
	s.mux.HandleFunc("GET /ws", notYet("phase 6"))

	s.mux.Handle("/", http.FileServer(http.Dir("./web")))
}

func (s *Server) handleListIntercept(w http.ResponseWriter, r *http.Request) {
	list := s.opts.Interceptor.List()
	type item struct {
		ID      int64  `json:"id"`
		Method  string `json:"method"`
		URL     string `json:"url"`
		Headers string `json:"headers"`
		Body    string `json:"body"`
	}
	out := make([]item, 0, len(list))
	for _, req := range list {
		out = append(out, item{ID: req.ID, Method: req.Method, URL: req.URL, Headers: req.Headers, Body: string(req.Body)})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *Server) handleToggleIntercept(w http.ResponseWriter, r *http.Request) {
	on := !s.opts.Interceptor.IsOn()
	s.opts.Interceptor.SetOn(on)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"on": on})
}

func (s *Server) handleForward(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Optional JSON body to edit the request before forwarding:
	// {"method": "...", "url": "...", "headers": "...", "body": "..."}
	// Empty/missing body just forwards the request unchanged.
	var edits struct {
		Method  string `json:"method"`
		URL     string `json:"url"`
		Headers string `json:"headers"`
		Body    string `json:"body"`
	}
	_ = json.NewDecoder(r.Body).Decode(&edits)

	d := intercept.Decision{Action: intercept.Forward, Method: edits.Method, URL: edits.URL, Headers: edits.Headers}
	if edits.Body != "" {
		d.Body = []byte(edits.Body)
	}

	if err := s.opts.Interceptor.Resolve(id, d); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDrop(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.opts.Interceptor.Resolve(id, intercept.Decision{Action: intercept.Drop}); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
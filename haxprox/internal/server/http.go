package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"haxprox/internal/proxy"
	"haxprox/internal/intercept"
	"haxprox/internal/repeater"
	"haxprox/internal/reqedit"
	"haxprox/internal/store"
	"haxprox/internal/intruder"
)

type Options struct {
	Proxy       *proxy.Proxy
	Store       *store.DB
	Interceptor *intercept.Queue
	Hub         *Hub
	Rules       *reqedit.RuleStore
	Intruder    *intruder.Registry
}

type Server struct {
	opts Options
	mux  *http.ServeMux
	hub  *Hub
}

func New(opts Options) *Server {
	s := &Server{opts: opts, mux: http.NewServeMux(), hub: opts.Hub}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/browse", s.handleBrowse)
	s.mux.HandleFunc("GET /api/history", s.handleHistory)
	s.mux.HandleFunc("GET /api/history/{id}", s.handleHistoryDetail)
	s.mux.HandleFunc("GET /api/history/export", s.handleHistoryExport)
	s.mux.HandleFunc("DELETE /api/history", s.handleHistoryClear)
	s.mux.HandleFunc("GET /api/intercept", s.handleListIntercept)
	s.mux.HandleFunc("POST /api/intercept/toggle", s.handleToggleIntercept)
	s.mux.HandleFunc("POST /api/intercept/{id}/forward", s.handleForward)
	s.mux.HandleFunc("POST /api/intercept/{id}/drop", s.handleDrop)
	s.mux.HandleFunc("POST /api/repeater/send", s.handleRepeaterSend)
	s.mux.HandleFunc("GET /api/rules", s.handleListRules)
	s.mux.HandleFunc("POST /api/rules", s.handleAddRule)
	s.mux.HandleFunc("PATCH /api/rules/{id}", s.handleUpdateRule)
	s.mux.HandleFunc("DELETE /api/rules/{id}", s.handleDeleteRule)
	s.mux.HandleFunc("POST /api/intruder/start", s.handleIntruderStart)
	s.mux.HandleFunc("GET /api/intruder/{id}/results", s.handleIntruderResults)
	s.mux.HandleFunc("POST /api/intruder/{id}/stop", s.handleIntruderStop)
	s.mux.HandleFunc("GET /ws", s.registerWS)

	s.mux.Handle("/", http.FileServer(http.Dir("./web")))
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Method  string `json:"method"`
		URL     string `json:"url"`
		Headers string `json:"headers"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if in.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	result := s.opts.Proxy.Browse(proxy.BrowseRequest{
		Method:  in.Method,
		URL:     in.URL,
		Headers: in.Headers,
		Body:    in.Body,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limit, offset := 50, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	entries, err := s.opts.Store.List(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	entry, err := s.opts.Store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// handleHistoryExport dumps the full history as JSON (default) or CSV,
// triggered as a file download from the UI's Export button.
func (s *Server) handleHistoryExport(w http.ResponseWriter, r *http.Request) {
	entries, err := s.opts.Store.All()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "csv" {
		s.writeHistoryCSV(w, entries)
		return
	}
	s.writeHistoryJSON(w, entries)
}

func (s *Server) handleHistoryClear(w http.ResponseWriter, r *http.Request) {
	if err := s.opts.Store.Clear(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) writeHistoryJSON(w http.ResponseWriter, entries []*store.Entry) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="haxprox-history.json"`)
	json.NewEncoder(w).Encode(entries)
}

func (s *Server) writeHistoryCSV(w http.ResponseWriter, entries []*store.Entry) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="haxprox-history.csv"`)

	cw := csv.NewWriter(w)
	defer cw.Flush()

	cw.Write([]string{"id", "timestamp", "method", "url", "host", "status_code", "req_headers", "resp_headers"})
	for _, e := range entries {
		cw.Write([]string{
			fmt.Sprintf("%d", e.ID),
			e.Timestamp.Format("2006-01-02 15:04:05"),
			e.Method,
			e.URL,
			e.Host,
			fmt.Sprintf("%d", e.StatusCode),
			e.ReqHeaders,
			e.RespHeaders,
		})
	}
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

	// Log to History too - same store/broadcast path as normal proxied
	// traffic and Browse, but deliberately still skips the Interceptor
	// (Repeater must never block waiting for Forward/Drop, that defeats
	// its purpose as a fast iterate-and-resend tool).
	status := resp.StatusCode
	entry := &store.Entry{
		Timestamp:   time.Now(),
		Method:      reqIn.Method,
		URL:         reqIn.URL,
		Host:        hostFromURL(reqIn.URL),
		StatusCode:  status,
		ReqHeaders:  reqIn.Headers,
		ReqBody:     []byte(reqIn.Body),
		RespHeaders: resp.Headers,
		RespBody:    resp.Body,
		Notes:       "sent via Repeater",
	}
	if _, err := s.opts.Store.Insert(entry); err != nil {
		log.Printf("store: failed to log repeater entry: %v", err)
	} else if s.hub != nil {
		s.hub.Broadcast(entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status_code": resp.StatusCode,
		"headers":     resp.Headers,
		"body":        string(resp.Body),
		"duration_ms": resp.Duration.Milliseconds(),
		"error":       resp.Err,
	})
}

// hostFromURL extracts just the host portion for the Entry.Host column,
// matching what the proxy pipeline stores for normal traffic.
func hostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.opts.Rules.List())
}

func (s *Server) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Target  string `json:"target"`
		Match   string `json:"match"`
		Replace string `json:"replace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	target := reqedit.Target(in.Target)
	if target != reqedit.TargetHeader && target != reqedit.TargetBody && target != reqedit.TargetURL {
		http.Error(w, "target must be one of: header, body, url", http.StatusBadRequest)
		return
	}

	rule, err := s.opts.Rules.Add(&reqedit.Rule{Enabled: true, Target: target, Match: in.Match, Replace: in.Replace})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rule)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var in struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.opts.Rules.SetEnabled(id, in.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.opts.Rules.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleIntruderStart(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Method      string   `json:"method"`
		URL         string   `json:"url"`
		Headers     string   `json:"headers"`
		Body        string   `json:"body"`
		Payloads    []string `json:"payloads"`
		Concurrency int      `json:"concurrency"`
		DelayMs     int      `json:"delay_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if in.URL == "" || len(in.Payloads) == 0 {
		http.Error(w, "url and payloads are required", http.StatusBadRequest)
		return
	}

	run := s.opts.Intruder.Start(intruder.Attack{
		Method:      in.Method,
		URL:         in.URL,
		Headers:     in.Headers,
		Body:        in.Body,
		Payloads:    in.Payloads,
		Concurrency: in.Concurrency,
		DelayMs:     in.DelayMs,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": run.ID})
}

func (s *Server) handleIntruderResults(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	run, err := s.opts.Intruder.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	results, done := run.Results()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"results": results, "done": done})
}

func (s *Server) handleIntruderStop(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.opts.Intruder.Stop(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}
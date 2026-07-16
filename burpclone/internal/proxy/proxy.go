// Package proxy implements the MITM proxy engine: a plain HTTP forward
// proxy first (phase 1), extended to handle HTTPS via CONNECT + on-the-fly
// TLS termination (phase 2).
package proxy

import (
	"log"
	"net"
	"sync"

	"burpclone/internal/ca"
	"burpclone/internal/intercept"
	"burpclone/internal/reqedit"
	"burpclone/internal/store"
)

// Broadcaster receives a copy of every logged entry, used to push live
// updates to WebSocket clients connected to the UI (see
// internal/server/ws.go).
type Broadcaster interface {
	Broadcast(e *store.Entry)
}

type Options struct {
	RootCA      *ca.Authority
	Store       *store.DB
	Interceptor *intercept.Queue
	Broadcaster Broadcaster
	ExcludeHosts []string
	MatchReplace *reqedit.RuleStore
}

// Proxy is the top-level proxy server.
type Proxy struct {
	opts Options

	failedMu     sync.Mutex
	failedHosts  map[string]bool // hosts where MITM handshake failed before - auto-passthrough from now on
}

func New(opts Options) *Proxy {
	return &Proxy{opts: opts, failedHosts: make(map[string]bool)}
}

// markHostFailed records that MITM handshake failed for host, so future
// CONNECT requests to it skip straight to passthrough instead of trying
// (and failing) the handshake again every time.
func (p *Proxy) markHostFailed(host string) {
	p.failedMu.Lock()
	defer p.failedMu.Unlock()
	if !p.failedHosts[host] {
		p.failedHosts[host] = true
		log.Printf("proxy: %s failed MITM handshake once - auto-passthrough for future requests (or add it to -exclude to skip the first failed attempt too)", host)
	}
}

func (p *Proxy) hasFailedBefore(host string) bool {
	p.failedMu.Lock()
	defer p.failedMu.Unlock()
	return p.failedHosts[host]
}

// ListenAndServe starts the TCP listener and accept loop.
//
// One goroutine per accepted connection - same shape as the worker-per-job
// pattern in portscanner/webcrawler, except here the "job" is a live
// connection rather than a fetch/probe.
func (p *Proxy) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// A transient accept error shouldn't kill the whole proxy -
			// log it and keep serving other connections.
			log.Printf("accept error: %v", err)
			continue
		}
		go p.handleConn(conn)
	}
}

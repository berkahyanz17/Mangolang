// Package proxy implements the MITM proxy engine: a plain HTTP forward
// proxy first (phase 1), extended to handle HTTPS via CONNECT + on-the-fly
// TLS termination (phase 2).
package proxy

import (
	"log"
	"net"

	"burpclone/internal/ca"
	"burpclone/internal/intercept"
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
}

// Proxy is the top-level proxy server.
type Proxy struct {
	opts Options
}

func New(opts Options) *Proxy {
	return &Proxy{opts: opts}
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

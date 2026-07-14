package proxy

import (
	"log"
	"net"
)

// handleConnect handles a "CONNECT host:port HTTP/1.1" request - this is
// what browsers send when they want to tunnel HTTPS through the proxy.
//
// Phase 1 status: NOT implemented yet. We reply with a clean error instead
// of panicking, so a single HTTPS request from the browser doesn't crash
// the whole proxy process (an unrecovered panic in any goroutine takes
// down the entire program in Go, not just that goroutine).
//
// TODO(phase 2): replace the body below with real MITM handling:
//  1. Write "HTTP/1.1 200 Connection Established\r\n\r\n" to conn first -
//     the browser expects this before it starts its own TLS handshake.
//  2. Call into tls.go's mitmTLS(conn, host) to terminate + re-encrypt.
//  3. (optional) add a passthrough fallback: net.Dial the real host and
//     io.Copy both directions, for hosts you choose not to inspect.
func (p *Proxy) handleConnect(conn net.Conn, targetHostPort string) {
	log.Printf("CONNECT %s - HTTPS MITM not implemented yet (phase 2), rejecting", targetHostPort)
	conn.Write([]byte("HTTP/1.1 501 Not Implemented\r\n\r\nHTTPS not supported yet (phase 2 pending)\n"))
}

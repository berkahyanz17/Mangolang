package proxy

import (
	"log"
	"net"
)

// handleConnect handles "CONNECT host:port HTTP/1.1" - reply 200 first
// (browser expects this before it starts its own TLS handshake), then
// hand off to mitmTLS to actually terminate + re-encrypt.
func (p *Proxy) handleConnect(conn net.Conn, targetHostPort string) {
	host, _, err := net.SplitHostPort(targetHostPort)
	if err != nil {
		host = targetHostPort
	}

	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		log.Printf("CONNECT %s: failed to reply 200: %v", targetHostPort, err)
		return
	}

	p.mitmTLS(conn, host, targetHostPort)
}
package proxy

import (
	"io"
	"log"
	"net"
	"path"
	"time"
)

// handleConnect handles "CONNECT host:port HTTP/1.1". If the host matches
// an exclude pattern, it does a raw passthrough tunnel (no inspection) -
// necessary for apps that use certificate pinning and would otherwise
// break or refuse to connect through our MITM cert. Otherwise it hands
// off to mitmTLS for full inspection.
func (p *Proxy) handleConnect(conn net.Conn, targetHostPort string) {
	host, _, err := net.SplitHostPort(targetHostPort)
	if err != nil {
		host = targetHostPort
	}

	if _, err := conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		log.Printf("CONNECT %s: failed to reply 200: %v", targetHostPort, err)
		return
	}

	if matchesExclude(host, p.opts.ExcludeHosts) {
		log.Printf("CONNECT %s - host is excluded, passing through without MITM", targetHostPort)
		passthrough(conn, targetHostPort)
		return
	}

	if p.hasFailedBefore(host) {
		log.Printf("CONNECT %s - host failed MITM handshake before, passing through", targetHostPort)
		passthrough(conn, targetHostPort)
		return
	}

	p.mitmTLS(conn, host, targetHostPort)
}

// matchesExclude reports whether host matches any of the given wildcard
// patterns. Patterns use path.Match syntax, e.g. "*.bank.co.id" matches
// "www.bank.co.id" and "secure.bank.co.id" but NOT "bank.co.id" itself -
// add both patterns explicitly if the bare domain should also be excluded.
func matchesExclude(host string, patterns []string) bool {
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if p == host {
			return true
		}
		if ok, _ := path.Match(p, host); ok {
			return true
		}
	}
	return false
}

// passthrough relays raw bytes both directions between the client and the
// real target, without any TLS termination or inspection - used for
// excluded hosts (see matchesExclude) where MITM would break the
// connection (cert pinning) or isn't wanted.
func passthrough(clientConn net.Conn, targetHostPort string) {
	upstream, err := net.DialTimeout("tcp", targetHostPort, 10*time.Second)
	if err != nil {
		log.Printf("passthrough: failed to dial %s: %v", targetHostPort, err)
		return
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(upstream, clientConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, upstream)
		done <- struct{}{}
	}()
	<-done // return once either direction closes - the other will error out and exit too
}
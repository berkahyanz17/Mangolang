package proxy

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"burpclone/internal/store"
)

// transport is reused across every forwarded request, same reasoning as
// httpClient being package-level in webcrawler/fetch.go: http.Transport
// pools and reuses TCP connections internally, so creating a new one per
// request would throw that away and be slower.
//
// We use Transport.RoundTrip directly (not http.Client) because a proxy
// should forward the request exactly once and hand the response straight
// back - http.Client would transparently follow redirects, which is NOT
// what a proxy wants (the browser needs to see the 3xx itself).
var transport = &http.Transport{
	Proxy:               nil, // we ARE the proxy, don't recurse into another one
	MaxIdleConns:        100,
	IdleConnTimeout:     90 * time.Second,
	TLSHandshakeTimeout: 10 * time.Second,
}

// hopByHopHeaders are connection-specific headers that must NOT be blindly
// forwarded to the upstream server (RFC 7230 Section 6.1) - they describe
// the client<->proxy hop, not the proxy<->server hop.
var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// handleConn is called once per accepted TCP connection from the client
// (browser/curl -x). It reads one request, decides plain-HTTP vs CONNECT,
// and dispatches accordingly.
func (p *Proxy) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		if err != io.EOF {
			log.Printf("failed to read request: %v", err)
		}
		return
	}

	if req.Method == http.MethodConnect {
		// Phase 2 territory (HTTPS MITM). Not implemented yet - fail
		// clean instead of leaving the browser hanging.
		p.handleConnect(conn, req.Host)
		return
	}

	p.handlePlainHTTP(conn, req)
}

// handlePlainHTTP forwards a single non-HTTPS proxied request to its real
// destination and writes the response straight back to the client.
func (p *Proxy) handlePlainHTTP(conn net.Conn, req *http.Request) {
	// For a forward proxy, req.URL is the full absolute URL
	// (e.g. "http://example.com/path") because the client sent it as
	// "GET http://example.com/path HTTP/1.1", not "GET /path HTTP/1.1".
	if !req.URL.IsAbs() {
		http.Error(newConnWriter(conn), "proxy: request URI must be absolute", http.StatusBadRequest)
		return
	}

	// Strip hop-by-hop headers before forwarding upstream.
	for _, h := range hopByHopHeaders {
		req.Header.Del(h)
	}

	// RequestURI must be empty on the client side of a Transport.RoundTrip -
	// it's only valid on requests read by a server (which is what we just did).
	req.RequestURI = ""

	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Printf("upstream request failed for %s: %v", req.URL, err)
		http.Error(newConnWriter(conn), "proxy: upstream request failed: "+err.Error(), http.StatusBadGateway)
		logEntry(p.opts.Store, req, nil)
		return
	}
	defer resp.Body.Close()

	// Response.Write serializes a client-side *http.Response back into
	// valid HTTP/1.x wire format - exactly what we need to relay it
	// untouched to the browser.
	if err := resp.Write(conn); err != nil {
		log.Printf("failed to write response back to client: %v", err)
	}

	logEntry(p.opts.Store, req, resp)
}

// logEntry records the request/response pair to the datastore.
//
// TODO(phase 3): once store.DB has a real Insert, this should build a
// store.Entry from req/resp (method, URL, status code, headers, and - if
// you want full bodies logged - read+restore req.Body/resp.Body via
// io.TeeReader before they're consumed above, since both are single-read
// streams).
func logEntry(db *store.DB, req *http.Request, resp *http.Response) {
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	log.Printf("%s %s -> %d", req.Method, req.URL, status)
	// db.Insert(&store.Entry{...}) once phase 3 lands.
	_ = db
}

// newConnWriter adapts a raw net.Conn so we can use http.Error (which
// wants an http.ResponseWriter) to send a quick error response directly
// over the connection during phase 1, before any Transport response
// exists to write back.
func newConnWriter(conn net.Conn) http.ResponseWriter {
	return &connWriter{conn: conn, header: http.Header{}}
}

// connWriter is a minimal http.ResponseWriter backed directly by a
// net.Conn - just enough to satisfy http.Error() for the handful of
// "can't even reach the upstream" error cases in this file. Not a general
// purpose ResponseWriter.
type connWriter struct {
	conn        net.Conn
	header      http.Header
	wroteHeader bool
}

func (w *connWriter) Header() http.Header { return w.header }

func (w *connWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.conn.Write(b)
}

func (w *connWriter) WriteHeader(statusCode int) {
	w.wroteHeader = true
	var sb strings.Builder
	sb.WriteString("HTTP/1.1 ")
	sb.WriteString(strconv.Itoa(statusCode))
	sb.WriteString(" ")
	sb.WriteString(http.StatusText(statusCode))
	sb.WriteString("\r\n")
	for k, vv := range w.header {
		for _, v := range vv {
			sb.WriteString(k + ": " + v + "\r\n")
		}
	}
	sb.WriteString("\r\n")
	w.conn.Write([]byte(sb.String()))
}

package proxy

import (
	"bufio"
	"bytes"
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

func (p *Proxy) handlePlainHTTP(conn net.Conn, req *http.Request) {
	if !req.URL.IsAbs() {
		http.Error(newConnWriter(conn), "proxy: request URI must be absolute", http.StatusBadRequest)
		return
	}

	for _, h := range hopByHopHeaders {
		req.Header.Del(h)
	}

	// Body is a single-read stream - buffer it so we can both forward it
	// upstream AND log it afterward.
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
	}
	var reqHeaderBuf bytes.Buffer
	req.Header.Write(&reqHeaderBuf)

	req.RequestURI = ""

	resp, err := transport.RoundTrip(req)
	if err != nil {
		log.Printf("upstream request failed for %s: %v", req.URL, err)
		http.Error(newConnWriter(conn), "proxy: upstream request failed: "+err.Error(), http.StatusBadGateway)
		logEntry(p.opts.Store, req, nil, reqBody, nil, reqHeaderBuf.String(), "")
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	var respHeaderBuf bytes.Buffer
	resp.Header.Write(&respHeaderBuf)

	if err := resp.Write(conn); err != nil {
		log.Printf("failed to write response back to client: %v", err)
	}

	logEntry(p.opts.Store, req, resp, reqBody, respBody, reqHeaderBuf.String(), respHeaderBuf.String())
}

// logEntry records the request/response pair to the datastore.
func logEntry(db *store.DB, req *http.Request, resp *http.Response, reqBody, respBody []byte, reqHeaders, respHeaders string) {
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	log.Printf("%s %s -> %d", req.Method, req.URL, status)

	entry := &store.Entry{
		Timestamp:   time.Now(),
		Method:      req.Method,
		URL:         req.URL.String(),
		Host:        req.URL.Host,
		StatusCode:  status,
		ReqHeaders:  reqHeaders,
		ReqBody:     reqBody,
		RespHeaders: respHeaders,
		RespBody:    respBody,
	}
	if _, err := db.Insert(entry); err != nil {
		log.Printf("store: failed to log entry: %v", err)
	}
}

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

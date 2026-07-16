package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
)

func (p *Proxy) mitmTLS(conn net.Conn, host, hostPort string) {
	tlsConn := tls.Server(conn, &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := hello.ServerName
			if name == "" {
				name = host
			}
			return p.opts.RootCA.GetOrCreateLeaf(name)
		},
	})
	defer tlsConn.Close()

	if err := tlsConn.Handshake(); err != nil {
		log.Printf("mitm: handshake failed for %s: %v", host, err)
		return
	}

	reader := bufio.NewReader(tlsConn)
	for {
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err != io.EOF {
				log.Printf("mitm: failed to read request from %s: %v", host, err)
			}
			return
		}

		req.URL.Scheme = "https"
		if req.URL.Host == "" {
			req.URL.Host = req.Host
		}
		if req.URL.Host == "" {
			req.URL.Host = hostPort
		}
		req.RequestURI = ""

		for _, h := range hopByHopHeaders {
			req.Header.Del(h)
		}

		var reqBody []byte
		if req.Body != nil {
			reqBody, _ = io.ReadAll(req.Body)
			req.Body.Close()
			req.Body = io.NopCloser(bytes.NewReader(reqBody))
		}
		var reqHeaderBuf bytes.Buffer
		req.Header.Write(&reqHeaderBuf)

		proceed, editedBody := interceptAndApply(p.opts.Interceptor, p.opts.MatchReplace, req, reqHeaderBuf.String(), reqBody)
		if !proceed {
			log.Printf("DROPPED %s %s", req.Method, req.URL)
			writeDropped(tlsConn)
			p.logEntry(req, nil, reqBody, nil, reqHeaderBuf.String(), "")
			continue // keep serving next request on this same TLS connection
		}
		reqBody = editedBody

		resp, err := transport.RoundTrip(req)
		if err != nil {
			log.Printf("mitm: upstream request failed for %s: %v", req.URL, err)
			http.Error(newConnWriter(tlsConn), "proxy: upstream request failed: "+err.Error(), http.StatusBadGateway)
			p.logEntry(req, nil, reqBody, nil, reqHeaderBuf.String(), "")
			return
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		var respHeaderBuf bytes.Buffer
		resp.Header.Write(&respHeaderBuf)

		if err := resp.Write(tlsConn); err != nil {
			log.Printf("mitm: failed to write response back to client: %v", err)
			return
		}
		p.logEntry(req, resp, reqBody, respBody, reqHeaderBuf.String(), respHeaderBuf.String())
	}
}
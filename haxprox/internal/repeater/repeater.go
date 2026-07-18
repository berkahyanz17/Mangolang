// Package repeater implements Burp's "Repeater" workflow: take a request
// (loaded from history or typed by hand), let the user edit it, and fire
// it directly at the target - independent of the live proxy traffic.
package repeater

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

// Request is what the caller sends when they hit "Send" in Repeater.
type Request struct {
	Method  string
	URL     string
	Headers string // raw "Key: value\r\nKey2: value2" text, same shape as store.Entry.ReqHeaders
	Body    []byte
}

// Response is what comes back, for the UI to render.
type Response struct {
	StatusCode int
	Headers    string
	Body       []byte
	Duration   time.Duration
	Err        string // string, not error, so it round-trips cleanly through JSON
}

// client is intentionally separate from the proxy package's transport:
// Repeater talks directly to arbitrary targets (including ones with
// self-signed/expired certs, since testing weird TLS configs is a
// legitimate pentest use case), so it skips certificate verification.
var client = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
	// Repeater should show the raw response as-is - if the server sends
	// a redirect, that IS the answer, don't silently follow it.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Send fires a single request directly at the target and returns the
// raw response (or an error string) for display.
func Send(req Request) Response {
	start := time.Now()

	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, body)
	if err != nil {
		return Response{Err: fmt.Sprintf("invalid request: %v", err), Duration: time.Since(start)}
	}

	if req.Headers != "" {
		hdr, err := parseHeaderText(req.Headers)
		if err != nil {
			return Response{Err: fmt.Sprintf("invalid headers: %v", err), Duration: time.Since(start)}
		}
		httpReq.Header = hdr
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{Err: fmt.Sprintf("request failed: %v", err), Duration: time.Since(start)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{Err: fmt.Sprintf("failed to read response body: %v", err), Duration: time.Since(start)}
	}

	var headerBuf bytes.Buffer
	resp.Header.Write(&headerBuf)

	return Response{
		StatusCode: resp.StatusCode,
		Headers:    headerBuf.String(),
		Body:       respBody,
		Duration:   time.Since(start),
	}
}

// parseHeaderText turns raw "Key: value\r\n..." text into an http.Header.
func parseHeaderText(raw string) (http.Header, error) {
	if strings.TrimSpace(raw) == "" {
		return http.Header{}, nil
	}
	tp := textproto.NewReader(bufio.NewReader(strings.NewReader(raw + "\r\n")))
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	return http.Header(mimeHeader), nil
}
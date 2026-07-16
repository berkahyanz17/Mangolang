package proxy

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"burpclone/internal/intercept"
)

// interceptAndApply routes the request through the interceptor queue. If
// interception is off, Hold returns immediately with Forward (no pause).
// If it's on, this blocks until someone calls Resolve via the REST API
// (see internal/server/http.go).
//
// Returns proceed=false if the request was dropped - caller should send a
// "dropped" response and stop instead of forwarding upstream.
func interceptAndApply(q *intercept.Queue, req *http.Request, headerText string, body []byte) (proceed bool, newBody []byte) {
	decision := q.Hold(req.Method, req.URL.String(), headerText, body)
	if decision.Action == intercept.Drop {
		return false, nil
	}

	if decision.Method != "" {
		req.Method = decision.Method
	}
	if decision.URL != "" {
		if u, err := url.Parse(decision.URL); err == nil {
			req.URL = u
		}
	}
	if decision.Headers != "" {
		if h, err := parseHeaderText(decision.Headers); err == nil {
			req.Header = h
		}
	}
	if decision.Body != nil {
		req.Body = io.NopCloser(bytes.NewReader(decision.Body))
		req.ContentLength = int64(len(decision.Body))
		req.Header.Set("Content-Length", strconv.Itoa(len(decision.Body)))
		return true, decision.Body
	}

	return true, body
}

// parseHeaderText turns raw "Key: value\r\n..." text (as edited by hand
// in the UI) back into an http.Header.
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

func writeDropped(w io.Writer) {
	w.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 24\r\n\r\nDropped by interceptor.\n"))
}
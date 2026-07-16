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
	"burpclone/internal/reqedit"
)

// interceptAndApply applies Match & Replace rules first (these run
// automatically on every request, on or off - they're "set and forget",
// not something reviewed manually), then routes through the interceptor
// queue for manual hold/forward/drop if interception is on.
func interceptAndApply(q *intercept.Queue, rules *reqedit.RuleStore, req *http.Request, headerText string, body []byte) (proceed bool, newBody []byte) {
	if rules != nil {
		if newURL := rules.Apply(reqedit.TargetURL, req.URL.String()); newURL != req.URL.String() {
			if u, err := url.Parse(newURL); err == nil {
				req.URL = u
			}
		}
		headerText = rules.Apply(reqedit.TargetHeader, headerText)
		if h, err := parseHeaderText(headerText); err == nil {
			req.Header = h
		}
		body = []byte(rules.Apply(reqedit.TargetBody, string(body)))
	}

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
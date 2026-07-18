package proxy

import (
	"bytes"
	"io"
	"net/http"
)

// BrowseRequest is a simple request fired from the web UI itself (via
// POST /api/browse), so testing doesn't require a separate curl -x
// command or configuring a browser to use the proxy.
type BrowseRequest struct {
	Method  string
	URL     string
	Headers string
	Body    string
}

type BrowseResult struct {
	StatusCode int    `json:"status_code"`
	Headers    string `json:"headers"`
	Body       string `json:"body"`
	Err        string `json:"error,omitempty"`
}

// Browse sends a request through the EXACT SAME pipeline as normal
// proxied traffic (hop-by-hop header stripping, Match & Replace rules,
// Intercept hold/forward/drop, History logging + WebSocket broadcast) -
// so testing from the web UI behaves identically to traffic sent via
// curl -x or a browser with the proxy configured. This is deliberately
// different from Repeater, which bypasses all of that on purpose.
func (p *Proxy) Browse(br BrowseRequest) BrowseResult {
	method := br.Method
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if br.Body != "" {
		bodyReader = bytes.NewReader([]byte(br.Body))
	}

	req, err := http.NewRequest(method, br.URL, bodyReader)
	if err != nil {
		return BrowseResult{Err: "invalid request: " + err.Error()}
	}
	req.Header.Set("User-Agent", "burpclone/1.0")
	req.Header.Set("Accept", "*/*")
	if br.Headers != "" {
		if h, err := parseHeaderText(br.Headers); err == nil {
			req.Header = h
		}
	}
	for _, h := range hopByHopHeaders {
		req.Header.Del(h)
	}

	var reqBody []byte
	if br.Body != "" {
		reqBody = []byte(br.Body)
	}
	var reqHeaderBuf bytes.Buffer
	req.Header.Write(&reqHeaderBuf)

	// This blocks here if Intercept is ON, exactly like a real proxied
	// request would - go to the Intercept tab to Forward/Drop it.
	proceed, editedBody := interceptAndApply(p.opts.Interceptor, p.opts.MatchReplace, req, reqHeaderBuf.String(), reqBody)
	if !proceed {
		p.logEntry(req, nil, reqBody, nil, reqHeaderBuf.String(), "")
		return BrowseResult{Err: "dropped by interceptor"}
	}
	reqBody = editedBody
	if reqBody != nil {
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		req.ContentLength = int64(len(reqBody))
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		p.logEntry(req, nil, reqBody, nil, reqHeaderBuf.String(), "")
		return BrowseResult{Err: "request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respHeaderBuf bytes.Buffer
	resp.Header.Write(&respHeaderBuf)

	p.logEntry(req, resp, reqBody, respBody, reqHeaderBuf.String(), respHeaderBuf.String())

	return BrowseResult{
		StatusCode: resp.StatusCode,
		Headers:    respHeaderBuf.String(),
		Body:       string(respBody),
	}
}
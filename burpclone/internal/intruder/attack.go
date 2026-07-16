// Package intruder implements a single-position payload substitution
// attack (Burp's "Sniper" attack type): take a request template with a
// §payload§ marker, fire it once per payload in a wordlist, and report
// status/length/timing for each so the user can spot anomalies.
package intruder

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const marker = "{{payload}}"

// Attack is the user-supplied config for one Intruder run.
type Attack struct {
	Method      string
	URL         string // may contain marker
	Headers     string // raw "Key: value\r\n..." text, may contain marker
	Body        string // may contain marker
	Payloads    []string
	Concurrency int // workers in parallel; defaults to 5 if <= 0
	DelayMs     int // optional delay between requests per worker, rate limiting
}

// Result is one payload's outcome.
type Result struct {
	Payload    string        `json:"payload"`
	StatusCode int           `json:"status_code"`
	BodyLength int           `json:"body_length"`
	Duration   time.Duration `json:"duration_ms"`
	Err        string        `json:"error,omitempty"`
}

// client is separate from proxy's transport and repeater's client -
// Intruder fires many requests fast, so keep-alives + a generous
// connection pool matter here more than for one-off Repeater sends.
var client = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConnsPerHost: 20,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Run fires one request per payload using a worker pool (same shape as
// the worker-pool pattern in portscanner/webcrawler), streaming each
// Result to resultChan as it completes. Run blocks until all payloads
// are done or ctx is cancelled, then closes resultChan.
func Run(ctx context.Context, attack Attack, resultChan chan<- Result) {
	defer close(resultChan)

	concurrency := attack.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	jobs := make(chan string)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for payload := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				resultChan <- fire(attack, payload)
				if attack.DelayMs > 0 {
					time.Sleep(time.Duration(attack.DelayMs) * time.Millisecond)
				}
			}
		}()
	}

	for _, payload := range attack.Payloads {
		select {
		case <-ctx.Done():
			goto drain
		case jobs <- payload:
		}
	}
drain:
	close(jobs)
	wg.Wait()
}

// fire substitutes payload into every occurrence of marker across
// method/URL/headers/body, sends the request, and returns a Result.
func fire(attack Attack, payload string) Result {
	start := time.Now()

	url := strings.ReplaceAll(attack.URL, marker, payload)
	headersText := strings.ReplaceAll(attack.Headers, marker, payload)
	bodyText := strings.ReplaceAll(attack.Body, marker, payload)

	method := attack.Method
	if method == "" {
		method = http.MethodGet
	}

	var bodyReader io.Reader
	if bodyText != "" {
		bodyReader = bytes.NewReader([]byte(bodyText))
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return Result{Payload: payload, Err: fmt.Sprintf("invalid request: %v", err), Duration: time.Since(start)}
	}

	if headersText != "" {
		for _, line := range strings.Split(headersText, "\r\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			idx := strings.Index(line, ":")
			if idx < 0 {
				continue
			}
			req.Header.Set(strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return Result{Payload: payload, Err: fmt.Sprintf("request failed: %v", err), Duration: time.Since(start)}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{Payload: payload, Err: fmt.Sprintf("failed to read body: %v", err), Duration: time.Since(start)}
	}

	return Result{
		Payload:    payload,
		StatusCode: resp.StatusCode,
		BodyLength: len(respBody),
		Duration:   time.Since(start),
	}
}
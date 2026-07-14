// Package repeater implements Burp's "Repeater" workflow: take a
// previously-logged request (loaded from the store), let the user edit it
// raw, and refire it standalone - independent of the live proxy traffic.
package repeater

import "time"

// Request is what the UI sends when the user hits "Send" in Repeater.
type Request struct {
	Method  string
	URL     string
	Headers map[string][]string // or raw header text, your call
	Body    []byte
}

// Response is what comes back, for the UI to render.
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
	Duration   time.Duration
	Err        error
}

// Send fires a single request directly at the target, bypassing the proxy
// entirely - the browser is not involved at this point, this is just an
// outbound HTTP client call.
//
// TODO(phase 5):
//  1. Because Repeater needs to send requests that might be "slightly
//     invalid" HTTP (weird headers, unusual casing, duplicate headers)
//     for testing purposes, prefer building the raw request bytes
//     manually and writing them over a net.Conn/tls.Conn, rather than
//     using http.Client (which normalizes/validates a lot of this away).
//  2. For HTTPS targets, tls.Dial with InsecureSkipVerify as an option
//     (a proxy/pentest tool talking to arbitrary targets, including ones
//     with self-signed certs, is a legitimate case to expose this).
//  3. Read the raw response, parse status line + headers + body
//     (handle both Content-Length and chunked Transfer-Encoding).
//  4. Return a Response - no logging to store needed here unless you want
//     Repeater history too (nice-to-have, not MVP).
func Send(req Request) Response {
	panic("TODO: implement Send (phase 5)")
}

// Package intercept implements the hold/forward/drop workflow: when
// interception is ON, the proxy pauses each request here and waits for
// the UI to decide what happens to it (Burp's "Intercept is on" toggle +
// forward/drop buttons).
package intercept

import "sync"

// PendingRequest represents one request currently held for review.
type PendingRequest struct {
	ID      int64
	Method  string
	URL     string
	Headers string
	Body    []byte

	// decision is how the proxy goroutine (blocked in Hold, see below)
	// learns what the user decided.
	decision chan Decision
}

// Decision is the outcome the UI sends back for a held request.
type Decision struct {
	Action  Action
	Method  string // allows editing before forwarding
	URL     string
	Headers string
	Body    []byte
}

type Action int

const (
	Forward Action = iota
	Drop
)

// Queue holds all currently-pending requests and tracks whether
// interception is globally on or off.
//
// Reuses the worker-pool-adjacent pattern from webcrawler/portscanner:
// a mutex-protected map, same shape as RobotsChecker.cache, just storing
// pending requests instead of parsed robots.txt rules.
type Queue struct {
	mu      sync.Mutex
	on      bool
	nextID  int64
	pending map[int64]*PendingRequest
}

func NewQueue(startOn bool) *Queue {
	return &Queue{
		on:      startOn,
		pending: make(map[int64]*PendingRequest),
	}
}

// SetOn toggles interception globally (called from the UI's on/off switch).
func (q *Queue) SetOn(on bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.on = on
}

// IsOn reports whether interception is currently enabled.
func (q *Queue) IsOn() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.on
}

// Hold is called by the proxy goroutine handling a live connection. If
// interception is on, it blocks that goroutine until the UI calls Resolve
// with a Decision. If interception is off, it should return an immediate
// Forward decision without blocking.
//
// TODO(phase 4):
//  1. If !q.IsOn(), return Decision{Action: Forward, ...original values} immediately.
//  2. Otherwise: assign an ID, create a buffered decision channel (size 1),
//     store *PendingRequest in q.pending, unlock, then `<-req.decision` to
//     block until Resolve is called from another goroutine (the UI's REST
//     handler for POST /intercept/{id}/forward or /drop).
//  3. Remove from q.pending once resolved.
//
// IMPORTANT: this blocks the proxy's per-connection goroutine, not the
// whole proxy - because each connection already runs in its own
// goroutine (same as the worker-per-job pattern in portscanner), holding
// one request doesn't stall the others.
func (q *Queue) Hold(method, url, headers string, body []byte) Decision {
	panic("TODO: implement Hold (phase 4)")
}

// List returns all currently pending requests, for the UI to display.
func (q *Queue) List() []*PendingRequest {
	panic("TODO: implement List (phase 4)")
}

// Resolve is called by the UI's REST handler when the user clicks
// Forward/Drop (optionally after editing the request).
func (q *Queue) Resolve(id int64, d Decision) error {
	panic("TODO: implement Resolve (phase 4)")
}

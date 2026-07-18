package intercept

import (
	"fmt"
	"sync"
)

type PendingRequest struct {
	ID      int64
	Method  string
	URL     string
	Headers string
	Body    []byte

	decision chan Decision
}

type Decision struct {
	Action  Action
	Method  string
	URL     string
	Headers string
	Body    []byte
}

type Action int

const (
	Forward Action = iota
	Drop
)

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

func (q *Queue) SetOn(on bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.on = on
}

func (q *Queue) IsOn() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.on
}

// Hold blocks the calling goroutine (one per live connection) until the
// UI/REST caller resolves it via Forward or Drop - unless interception is
// off, in which case it returns an immediate Forward decision.
func (q *Queue) Hold(method, url, headers string, body []byte) Decision {
	q.mu.Lock()
	if !q.on {
		q.mu.Unlock()
		return Decision{Action: Forward, Method: method, URL: url, Headers: headers, Body: body}
	}

	q.nextID++
	id := q.nextID
	req := &PendingRequest{
		ID:       id,
		Method:   method,
		URL:      url,
		Headers:  headers,
		Body:     body,
		decision: make(chan Decision, 1),
	}
	q.pending[id] = req
	q.mu.Unlock()

	d := <-req.decision // blocks here until Resolve is called

	q.mu.Lock()
	delete(q.pending, id)
	q.mu.Unlock()

	return d
}

func (q *Queue) List() []*PendingRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	list := make([]*PendingRequest, 0, len(q.pending))
	for _, r := range q.pending {
		list = append(list, r)
	}
	return list
}

func (q *Queue) Resolve(id int64, d Decision) error {
	q.mu.Lock()
	req, ok := q.pending[id]
	q.mu.Unlock()
	if !ok {
		return fmt.Errorf("intercept: no pending request with id %d", id)
	}
	req.decision <- d
	return nil
}
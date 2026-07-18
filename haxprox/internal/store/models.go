package store

import "time"

// Entry represents one logged request/response pair, the core unit of
// "history" in the UI (equivalent to Burp's HTTP history table).
type Entry struct {
	ID         int64
	Timestamp  time.Time
	Method     string
	URL        string
	Host       string
	StatusCode int    // 0 if the request never got a response (dropped/errored)
	ReqHeaders string // stored as raw text; parse on demand in the UI
	ReqBody    []byte
	RespHeaders string
	RespBody    []byte
	Notes       string // free-text tag/comment the user can add later
}

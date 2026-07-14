// Package store handles persistent logging of proxied traffic to SQLite,
// so the UI can browse history (like Burp's HTTP history tab) even after
// a restart.
package store

import (
	"fmt"
	"sync"
)

// DB wraps a *sql.DB with the query helpers this package needs.
//
// TODO(phase 3): swap the in-memory placeholder fields below for a real
// *sql.DB once a driver is chosen (modernc.org/sqlite recommended - pure
// Go, no cgo).
type DB struct {
	// conn *sql.DB

	mu     sync.Mutex
	nextID int64
}

// Open opens (or creates) the SQLite file at path and runs migrations.
//
// Phase 1 placeholder: returns an in-memory-only DB (nothing is persisted
// to disk yet) so the proxy has something to log to while it's being
// tested. Swap this out per the TODO below once phase 3 starts.
//
// TODO(phase 3):
//  1. sql.Open("sqlite", path) (driver name depends on which package you
//     `go get` - modernc.org/sqlite registers as "sqlite")
//  2. Run a CREATE TABLE IF NOT EXISTS entries (...) matching the Entry
//     struct in models.go. Keep it simple - one table is enough for MVP,
//     no need to normalize headers into a separate table yet.
//  3. Return &DB{conn: db}
func Open(path string) (*DB, error) {
	return &DB{}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return nil // no-op until phase 3 wires up a real *sql.DB
}

// Insert logs a new Entry and returns its assigned ID.
//
// Phase 1 placeholder: just hands out an incrementing ID, doesn't persist
// anything. Real INSERT INTO entries (...) lands in phase 3.
func (d *DB) Insert(e *Entry) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nextID++
	e.ID = d.nextID
	return e.ID, nil
}

// List returns entries for the history view, most recent first.
//
// TODO(phase 3): SELECT ... ORDER BY id DESC LIMIT ? OFFSET ? - used by
// the REST API in internal/server for pagination.
func (d *DB) List(limit, offset int) ([]*Entry, error) {
	return nil, fmt.Errorf("store: List not implemented yet (phase 3)")
}

// Get fetches a single entry by ID - used by the Repeater (phase 5) to
// load a request the user wants to re-edit and resend.
func (d *DB) Get(id int64) (*Entry, error) {
	return nil, fmt.Errorf("store: Get not implemented yet (phase 3)")
}

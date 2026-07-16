package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// DB wraps a *sql.DB with the query helpers this package needs.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite file at path and runs migrations.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS entries (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp    DATETIME NOT NULL,
		method       TEXT NOT NULL,
		url          TEXT NOT NULL,
		host         TEXT NOT NULL,
		status_code  INTEGER NOT NULL DEFAULT 0,
		req_headers  TEXT,
		req_body     BLOB,
		resp_headers TEXT,
		resp_body    BLOB,
		notes        TEXT
	);`
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("store: failed to run migration: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

// Insert logs a new Entry and returns its assigned ID.
func (d *DB) Insert(e *Entry) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO entries (timestamp, method, url, host, status_code, req_headers, req_body, resp_headers, resp_body, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp, e.Method, e.URL, e.Host, e.StatusCode,
		e.ReqHeaders, e.ReqBody, e.RespHeaders, e.RespBody, e.Notes,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	e.ID = id
	return id, nil
}

// List returns entries for the history view, most recent first. Bodies
// are omitted here for speed - fetch a single Entry via Get for full detail.
func (d *DB) List(limit, offset int) ([]*Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, timestamp, method, url, host, status_code, req_headers, resp_headers, notes
		 FROM entries ORDER BY id DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		e := &Entry{}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Method, &e.URL, &e.Host, &e.StatusCode, &e.ReqHeaders, &e.RespHeaders, &e.Notes); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Get fetches a single entry by ID, including full request/response bodies.
func (d *DB) Get(id int64) (*Entry, error) {
	e := &Entry{}
	row := d.conn.QueryRow(
		`SELECT id, timestamp, method, url, host, status_code, req_headers, req_body, resp_headers, resp_body, notes
		 FROM entries WHERE id = ?`, id,
	)
	if err := row.Scan(&e.ID, &e.Timestamp, &e.Method, &e.URL, &e.Host, &e.StatusCode, &e.ReqHeaders, &e.ReqBody, &e.RespHeaders, &e.RespBody, &e.Notes); err != nil {
		return nil, err
	}
	return e, nil
}

// All returns every entry, including full request/response bodies, most
// recent first - used by the export endpoint. Unlike List (which is for
// paginated UI display and skips bodies), this pulls everything since an
// export should be a complete record.
func (d *DB) All() ([]*Entry, error) {
	rows, err := d.conn.Query(
		`SELECT id, timestamp, method, url, host, status_code, req_headers, req_body, resp_headers, resp_body, notes
		 FROM entries ORDER BY id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		e := &Entry{}
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Method, &e.URL, &e.Host, &e.StatusCode, &e.ReqHeaders, &e.ReqBody, &e.RespHeaders, &e.RespBody, &e.Notes); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
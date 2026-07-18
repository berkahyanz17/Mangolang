module haxprox

go 1.25.0

// Dependency notes:
// - Phase 1-4 (proxy, CA, SQLite, intercept) can mostly run on stdlib +
//   a SQLite driver. Two common choices:
//     modernc.org/sqlite       -> pure Go, no cgo, easiest to build
//     github.com/mattn/go-sqlite3 -> cgo-based, faster but needs a C compiler
//   Recommendation: start with modernc.org/sqlite to avoid cgo headaches.
// - Phase 6 (UI/WebSocket) needs a WebSocket lib:
//     github.com/gorilla/websocket   (widely used, stable)
//     nhooyr.io/websocket            (more modern, context-based API)
//   Recommendation: gorilla/websocket, more examples/docs available.
//
// Once network access allows `go get`, run:
//   go get modernc.org/sqlite
//   go get github.com/gorilla/websocket
// then `go mod tidy`.

require (
	github.com/gorilla/websocket v1.5.3
	modernc.org/sqlite v1.53.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.44.0 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

module burpclone

go 1.22

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

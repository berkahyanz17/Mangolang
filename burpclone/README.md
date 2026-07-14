# burpclone

MITM proxy + traffic inspector, dibuat dari nol pakai Go — versi mini
dari workflow Burp Suite (Proxy tab + Repeater), sebagai capstone project
setelah `mycli`, `todoapi`, `webcrawler`, dan `portscanner`.

## Status

🚧 Scaffold — semua fungsi inti masih `panic("TODO...")`. Struktur folder
dan urutan implementasi sudah dirancang, tinggal diisi sesuai build order
di bawah.

## Arsitektur

```
burpclone/
├── main.go                  # entrypoint, flag parsing, wiring semua modul
├── internal/
│   ├── ca/                  # root CA + leaf cert signing on-the-fly (per host)
│   ├── proxy/               # TCP listener, CONNECT tunneling, MITM TLS termination
│   ├── intercept/           # hold/forward/drop queue (channel-based)
│   ├── store/                # SQLite logging (request/response history)
│   ├── repeater/             # edit raw request, refire standalone
│   └── server/               # REST API + WebSocket live feed (backend buat web/)
└── web/                      # frontend statis (HTML/JS/CSS)
```

## Build order

| # | Modul | Estimasi | Kenapa duluan/belakangan |
|---|---|---|---|
| 1 | Plain HTTP forward proxy | 2-4 hari | Fondasi paling sederhana, belum ada TLS |
| 2 | HTTPS MITM (CA + dynamic cert) | 1-2 minggu | Paling sulit — TLS server+client dua arah |
| 3 | SQLite logging | 2-3 hari | Butuh proxy jalan dulu biar ada data buat dicatat |
| 4 | Intercept hold/forward | 3-5 hari | Reuse pola worker pool/channel dari webcrawler-portscanner |
| 5 | Repeater | 3-5 hari | Butuh store (buat load request lama) |
| 6 | UI (REST + WebSocket) | 1-2 minggu | Terakhir, nge-expose semua modul di atas |

Total MVP: ~3-4 minggu part-time.

## Perbandingan dengan project Go lain di repo ini

Reuse pola lama:
- Worker pool + channel (dari `webcrawler`/`portscanner`) → interceptor queue
- `sync.Mutex` + map cache (dari `RobotsChecker` di `webcrawler`) → leaf cert cache per host
- `net.DialTimeout` (dari `portscanner`) → forward koneksi ke server asli

Domain baru (belum pernah dipakai di project sebelumnya):
- `crypto/tls`, `crypto/x509` — MITM TLS handshake, root CA & leaf cert generation
- `net.Listen` sebagai **server** (bukan client seperti webcrawler/portscanner)
- Bidirectional tunneling (`io.Copy` dua arah) untuk CONNECT passthrough
- `database/sql` + SQLite driver — belum ada persistent storage di project lain
- `net/http` sebagai server + WebSocket — UI live traffic feed
- Raw HTTP request editing (Repeater) — kontrol byte-level, bukan lewat `http.Client`

## Dependencies

Project ini **tidak** stdlib-only (beda dari `webcrawler`/`portscanner`
yang sengaja dibatasi karena akses jaringan waktu itu terbatas). Begitu
akses `go get` normal:

```bash
go get modernc.org/sqlite       # SQLite driver, pure Go, no cgo
go get github.com/gorilla/websocket
go mod tidy
```

## Menjalankan (setelah modul-modul di atas diimplementasikan)

```bash
go run . -listen :8080 -ui :8000 -ca-dir ./ca-store -db ./burpclone.db
```

Lalu:
1. Set proxy browser ke `127.0.0.1:8080`
2. Import `ca-store/root.pem` ke trust store OS/browser (biar HTTPS gak
   muncul warning "certificate not trusted")
3. Buka `http://127.0.0.1:8000` buat lihat History/Intercept/Repeater

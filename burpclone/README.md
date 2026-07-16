# burpclone

MITM proxy + traffic inspector, dibuat dari nol pakai Go — versi mini
dari workflow Burp Suite (Proxy tab + Repeater), sebagai capstone project
setelah `mycli`, `todoapi`, `webcrawler`, dan `portscanner`.

## Status

✅ **MVP selesai** — semua 6 modul rencana awal udah diimplementasikan dan
diuji jalan end-to-end (HTTP forward, HTTPS MITM, SQLite logging,
intercept hold/forward/drop, Repeater, dan UI web dengan live feed).

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
Detail teknis tiap modul (kenapa dibikin gitu, urutan build, gotcha yang
ketemu pas development) ada di [`ARCHITECTURE.md`](./ARCHITECTURE.md).

## Build order (historis)

| # | Modul | Estimasi awal | Status |
|---|---|---|---|
| 1 | Plain HTTP forward proxy | 2-4 hari | ✅ |
| 2 | HTTPS MITM (CA + dynamic cert) | 1-2 minggu | ✅ |
| 3 | SQLite logging | 2-3 hari | ✅ |
| 4 | Intercept hold/forward | 3-5 hari | ✅ |
| 5 | Repeater | 3-5 hari | ✅ |
| 6 | UI (REST + WebSocket) | 1-2 minggu | ✅ |

## Perbandingan dengan project Go lain di repo ini

Reuse pola lama:
- Worker pool + channel (dari `webcrawler`/`portscanner`) → interceptor queue
- `sync.Mutex` + map cache (dari `RobotsChecker` di `webcrawler`) → leaf cert cache per host
- `net.DialTimeout` (dari `portscanner`) → forward koneksi ke server asli

Domain baru yang dipelajari di project ini:
- `crypto/tls`, `crypto/x509` — MITM TLS handshake, root CA & leaf cert generation on-the-fly (SNI-based)
- `net.Listen` sebagai **server** (bukan client seperti webcrawler/portscanner)
- `database/sql` + SQLite driver (`modernc.org/sqlite`) — persistent storage
- `net/http` sebagai server + WebSocket (`gorilla/websocket`) — UI live traffic feed
- Raw HTTP request editing (Repeater) — kontrol byte-level, bukan lewat `http.Client` default

## Dependencies

```bash
go get modernc.org/sqlite       # SQLite driver, pure Go, no cgo
go get github.com/gorilla/websocket
go mod tidy
```

## Menjalankan

```bash
go run . -listen :8080 -ui :8000 -ca-dir ./ca-store -db ./burpclone.db
```

Lalu:
1. Set proxy browser/OS ke `127.0.0.1:8080` (atau pakai `curl.exe -x http://127.0.0.1:8080 ...` buat testing cepat tanpa ubah setting sistem)
2. Import `ca-store/root.pem` ke trust store OS/browser — di Windows: `certutil -addstore -f "ROOT" .\ca-store\root.pem` (perlu run as Administrator), atau lewat GUI `certlm.msc`
3. Buka `http://127.0.0.1:8000` buat lihat History / Intercept / Repeater

**Catatan curl di Windows:** kalau ketemu error `CRYPT_E_NO_REVOCATION_CHECK`, tambahin flag `--ssl-no-revoke` — ini normal buat CA self-signed tanpa CRL/OCSP endpoint, bukan bug.

## Rencana fitur lanjutan

Di luar scope MVP, urutan berdasarkan value/effort:

| Fitur | Kenapa berguna | Effort |
|---|---|---|
| **Scope / exclude-list** | Skip MITM buat domain tertentu (misal banking apps yang certificate-pinning, jadi request-nya lewat passthrough tunnel biasa daripada di-reject) | Kecil — tambah field `[]string` di `proxy.Options`, cek host sebelum panggil `mitmTLS` di `connect.go` |
| **Export history** | Simpen hasil intercept ke JSON/CSV/format Burp-compatible buat dibagi atau diproses tools lain | Kecil — endpoint baru di `server/http.go`, query `store.List` lalu marshal |
| **Match & Replace** | Auto-replace header/body pattern tertentu di semua trafik (misal strip tracking header, inject auth token otomatis) | Sedang — butuh struct rule (regex in/out), diterapkan di `interceptAndApply` sebelum/sesudah decision |
| **Auth buat UI** | Proteksi `:8000` biar gak sembarang orang di jaringan yang sama bisa buka history/intercept/repeater lo | Sedang — basic auth middleware di `server/http.go`, atau token dari flag CLI |
| **Repeater history** | Simpen riwayat request yang pernah dikirim lewat Repeater (sekarang standalone, sengaja gak nyatet — lihat catatan desain di `ARCHITECTURE.md`) | Sedang — tabel SQLite terpisah, `repeater.Send` opsional nerima `*store.DB` |
| **Scripting/extension** | Custom logic per-request (semacam Burp extension sederhana), misal auto-decode JWT di UI | Besar — butuh desain plugin API sendiri, di luar prioritas MVP |
| **Passthrough tunnel fallback** | Kalau `mitmTLS` gagal handshake (client nolak cert), fallback ke `io.Copy` dua arah instead of langsung putus koneksi | Kecil-sedang — tambahan di `tls.go`, deteksi handshake error lalu re-dial raw |

Kalau mau lanjut salah satu, mulai dari **scope/exclude-list** atau
**export history** — dua-duanya kecil dan langsung kepake buat workflow
sehari-hari sebelum masuk yang lebih kompleks kayak Match & Replace atau
scripting.
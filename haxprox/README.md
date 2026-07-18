# haxprox

MITM proxy + traffic inspector, dibuat dari nol pakai Go — versi mini
dari workflow Burp Suite (Proxy tab + Repeater), sebagai capstone project
setelah `mycli`, `todoapi`, `webcrawler`, dan `portscanner`.

## Status

✅ **MVP selesai** — semua 6 modul rencana awal udah diimplementasikan dan
diuji jalan end-to-end (HTTP forward, HTTPS MITM, SQLite logging,
intercept hold/forward/drop, Repeater, dan UI web dengan live feed).

## Arsitektur
```
haxprox/
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
go run . -listen :8080 -ui :8000 -ca-dir ./ca-store -db ./haxprox.db
```

Lalu:
1. Set proxy browser/OS ke `127.0.0.1:8080` (atau pakai `curl.exe -x http://127.0.0.1:8080 ...` buat testing cepat tanpa ubah setting sistem)
2. Import `ca-store/root.pem` ke trust store OS/browser — di Windows: `certutil -addstore -f "ROOT" .\ca-store\root.pem` (perlu run as Administrator), atau lewat GUI `certlm.msc`
3. Buka `http://127.0.0.1:8000` buat lihat History / Intercept / Repeater

**Catatan curl di Windows:** kalau ketemu error `CRYPT_E_NO_REVOCATION_CHECK`, tambahin flag `--ssl-no-revoke` — ini normal buat CA self-signed tanpa CRL/OCSP endpoint, bukan bug.

## Roadmap fitur lanjutan

Di luar scope MVP, disusun berdasarkan urutan pengerjaan (bukan cuma
value/effort, tapi juga dependency antar fitur):

| # | Fitur | Effort | Status |
|---|---|---|---|
| 1 | **Scope / exclude-list** | ~1-2 hari | ⏳ Next |
| 2 | **Export history** | ~1 hari | ⏳ |
| 3 | **Inspector** (panel parsing request/response + decode/encode cepat: Base64, URL, hex, JWT) | ~2-3 hari | ⏳ |
| 4 | **Match & Replace** | ~3-4 hari | ⏳ |
| 5 | **Passthrough fallback** (otomatis, saat handshake MITM gagal) | ~2-3 hari | ⏳ |
| 6 | **Intruder** (attack type Sniper dulu, single payload position) | ~1 minggu | ⏳ |

### Kenapa urutan ini

- **#1 duluan** — paling kecil, dan langsung kepake tiap hari (banyak app
  modern certificate-pinning, tanpa exclude-list proxy "gagal" terus buat
  domain itu).
- **#2 setelah #1** — independen, quick win, gak ada dependency ke fitur
  lain.
- **#3 di tengah** — Inspector cuma baca data yang udah ada di
  `store.Entry`/response Repeater dan nampilinnya lebih rapi. Gak
  nyentuh `internal/proxy` sama sekali, jadi aman dikerjain kapan pun
  tanpa risk break fitur lain — cocok buat "istirahat" sebelum masuk
  fitur yang lebih kompleks lagi.
- **#4 sebelum #5** — Match & Replace nyentuh `interceptAndApply`, jadi
  dikerjain dulu sebelum Passthrough fallback (yang juga nyentuh alur
  MITM) biar gak numpuk perubahan di area yang sama bersamaan.
- **#5 setelah #1** — secara konsep ini penyempurna dari exclude-list:
  #1 = passthrough manual (user yang nentuin domain mana), #5 =
  passthrough otomatis (proxy yang detect sendiri pas handshake gagal).
  Ngerjain #1 duluan bikin logic passthrough-nya udah dipahami sebelum
  masuk versi otomatis.
- **#6 terakhir** — paling kompleks: butuh placeholder parsing
  (`§payload§`), payload list/wordlist, loop request dengan concurrency
  + rate limiting (reuse pola worker pool dari `portscanner`).

### Keputusan arsitektur: shared `reqedit` helper

Match & Replace (#4) dan Intruder (#6) sama-sama butuh cara nge-parse
dan edit raw request sebagai string/template (cari-ganti pattern buat
M&R, cari-ganti placeholder `§...§` buat Intruder). Daripada duplikat
logic ini di dua tempat, keduanya bakal pakai satu package baru
`internal/reqedit` yang nyediain:
- Parsing header/body request jadi bentuk yang bisa di-edit terprogram
- Substitusi berbasis regex (dipakai M&R) atau berbasis penanda posisi
  (dipakai Intruder)

Ini diputuskan pas #4 mulai dikerjain, biar #6 tinggal reuse tanpa
refactor ulang.

### Fitur yang dipertimbangkan tapi belum masuk roadmap resmi

| Fitur | Kenapa ditunda |
|---|---|
| **Auth buat UI** | Resiko rendah selama dipakai di localhost sendiri; masuk kalau proxy mulai dijalanin di jaringan bareng |
| **Repeater history** | Nice-to-have, gak critical buat workflow inti |
| **Burp Collaborator** | Beda kelas dari fitur lain — butuh infrastruktur eksternal (domain publik + DNS server + VPS terekspos internet), bukan sekadar nambah kode di proxy lokal. Effort ~2-4 minggu dan perlu keputusan infra dulu (domain apa, ada VPS gak) sebelum mulai. Dipertimbangkan setelah 6 fitur di atas selesai. |
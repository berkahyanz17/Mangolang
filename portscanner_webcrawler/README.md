# portscanner

Recon tool 3-in-1 dalam satu binary Go, standard library only: **port scan → web crawl → web misconfig check**. Dibangun bertahap sebagai project belajar Go, sekarang digabung jadi satu pipeline yang saling nyambung.

---

## 1. Peta Keseluruhan

```
portscanner/
├── go.mod                          # identitas module Go (module portscanner)
├── main.go                         # entrypoint CLI, orkestrasi 3 tahap
├── README.md                       # dokumentasi ini
└── internal/
    ├── scanner/
    │   ├── ports.go                 # parsing spec port ("1-1024,80,443")
    │   ├── services.go              # tabel lookup nama service per port
    │   ├── scan.go                  # logic scan inti (worker pool, dial, banner)
    │   └── httpcheck.go             # cek misconfig web dasar (headers, exposed files, cookies)
    └── crawler/
        ├── fetch.go                 # download 1 halaman + extract title/link
        ├── crawl.go                 # orkestrasi crawl (worker pool, BFS, dll)
        └── robots.go                # fetch & cek aturan robots.txt
```

## 2. Alur Data (3 Tahap)

```
main.go
  │  parse flag (-host, -ports, -crawl, -httpcheck, dll)
  ▼
scanner.ParsePorts()  ← ports.go
  │  "1-1024,8080" → []int{1,2,...,1024,8080}
  ▼
scanner.Scan()  ← scan.go
  │  worker pool, tiap worker: net.DialTimeout + banner grab
  ▼
[TAHAP 1 SELESAI] hasil di-collect, di-sort, di-print, disimpen CSV
  │
  │  untuk tiap port yang OPEN dan service-nya http/https:
  ▼
crawler.Crawl()  ← internal/crawler/crawl.go
  │  worker pool, tiap worker: fetch halaman, extract link, ikuti sampai MaxDepth
  ▼
[TAHAP 2 SELESAI] tiap URL yang di-crawl, hasilnya di-print (streaming)
  │
  │  untuk tiap origin (scheme://host:port) yang ketemu di tahap 1:
  ▼
scanner.CheckHTTP()  ← httpcheck.go
  │  cek security headers, exposed files (.env, .git), cookie flags, versi TLS
  ▼
[TAHAP 3 SELESAI] hasil misconfig di-print
```

**Kenapa urutannya begini?** Ini pola recon klasik: *discovery* (port apa yang kebuka) → *enumeration* (endpoint apa aja yang ada) → *assessment* (ada misconfig/vuln gak). Tiap tahap makan output tahap sebelumnya sebagai input — bukan tiga tool lepas yang kebetulan ada di folder sama.

Kalau dibandingin sama versi `webcrawler` yang berdiri sendiri dulu, pola concurrency-nya **mirip banget** dengan `scanner`: parse input → worker pool → channel streaming hasil → main.go nampilin. Bedanya cuma soal *kapan* jumlah kerjaan diketahui — dijelasin di bagian 7.

---

## 3. `go.mod`

```
module portscanner

go 1.22
```

Satu module buat semua sub-package. Import path yang dipakai di `main.go`:
```go
import (
    "portscanner/internal/crawler"
    "portscanner/internal/scanner"
)
```
Karena `crawler` dan `scanner` beda package (`package crawler` vs `package scanner`), gak ada bentrok nama meskipun keduanya sama-sama punya struct `Result` dan `Options` — dipanggil selalu dengan prefix (`scanner.Result` vs `crawler.Result`).

---

## 4. Modul `scanner` — Port Scanning

### `ports.go` — Parsing Spec Port

`ParsePorts(spec string) ([]int, error)` ubah string `-ports` (misal `"22,80,8000-8100"`) jadi `[]int`. Alur: split by koma → deteksi range (`-`) vs single angka → dedup pakai `map[int]bool` → validasi 1-65535.

Dipisah dari `scan.go` karena murni parsing string, gak nyentuh network — gampang di-unit-test sendiri.

### `services.go` — Lookup Nama Service

`map[int]string` port umum → nama service (`80` → `"http"`, `22` → `"ssh"`). **Ini cuma label**, bukan verifikasi beneran — kalau ada service aneh di-listen di port 80, tetap dibilang `"http"`. Verifikasi beneran butuh banner grabbing atau `httpcheck`.

### `scan.go` — Logic Scan Inti

`Scan(host string, ports []int, opts Options) <-chan Result` — semua port yang mau di-scan **udah diketahui dari awal**, jadi channel `jobs` langsung diisi penuh dan ditutup saat itu juga, gak perlu `sync.WaitGroup` buat nge-track kerjaan baru.

`probe()` (private) — `net.DialTimeout("tcp", ...)` buat TCP connect scan (3-way handshake). Berhasil konek = open, gagal/timeout = closed/filtered.

`grabBanner()` — baca apa yang dikirim service begitu konek (SSH, FTP, SMTP suka kirim salam duluan), dengan `SetReadDeadline` 1 detik biar gak block selamanya buat service yang diem (HTTP).

`SortResults()` — urutin hasil by nomor port setelah semua goroutine selesai (hasil paralel gak berurutan secara alami).

### `httpcheck.go` — Web Misconfig Check *(baru)*

`CheckHTTP(host string, port int, scheme string) HTTPCheckResult` — begitu scanner nemu port HTTP/HTTPS kebuka, jalanin check ringan tanpa perlu tool eksternal:

- **Security headers** — cek ada/gaknya `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Strict-Transport-Security`
- **Exposed sensitive files** — probe `/.env`, `/.git/HEAD`, `/.git/config`, `/backup.sql`, dll; kalau balikin 200 OK, ditandain exposed
- **Cookie flags** — cek cookie yang di-set tanpa `Secure`/`HttpOnly`
- **TLS version** — kalau scheme https, catat versi TLS yang dipakai (flag TLS 1.0/1.1 sebagai weak)

`CheckRedirect` di-override supaya gak ikutin redirect — biar hasil check gak nyasar ke halaman lain yang beda header/cookie-nya.

---

## 5. Modul `crawler` — Web Crawling *(digabung dari project `webcrawler`)*

### `fetch.go` — Download & Extract 1 Halaman

Struct `Page{URL, Title, Links, Status}`. Fungsi `Fetch(rawURL string) (Page, error)`:

1. HTTP request manual (`http.NewRequest`) biar bisa set `User-Agent` custom
2. `httpClient` di-reuse (bukan bikin baru tiap request) — aman dipakai lintas goroutine
3. Cek `Content-Type`, skip kalau bukan `text/html`
4. Baca body dibatasi `io.LimitReader(resp.Body, 5*1024*1024)` — safety net 5MB
5. Extract title & link pakai regex (`hrefRe`, `titleRe`) — trade-off sengaja: regex "cukup baik" tanpa perlu dependency eksternal (`golang.org/x/net/html`)
6. Resolve link relatif → absolut pakai `base.Parse(link)` dari `net/url`

### `robots.go` — Cek `robots.txt`

`RobotsChecker` dengan cache per-host (`sync.Mutex`-protected karena dipanggil dari banyak worker). `Allowed(rawURL string) bool` cari aturan `Disallow`/`Allow` paling spesifik sesuai spek resmi robots.txt.

**Fail-open by design**: kalau `robots.txt` gak ketemu/gagal fetch, dianggap semua diizinkan — perilaku standar kebanyakan crawler.

### `crawl.go` — Orkestrasi Crawl

`Crawl(startURL string, opts Options) <-chan Result` — beda penting dari `scanner.Scan()`: jumlah "kerjaan" (link) **gak diketahui dari awal**, baru ketauan pas crawling jalan. Makanya butuh:

- `sync.WaitGroup` (`pending`) buat tracking kerjaan yang "beranak-pinak"
- Job pertama (`enqueue(startURL, 0)`) **harus** dipanggil synchronous, bukan di goroutine baru — kalau enggak, ada race condition channel `jobs` bisa ketutup prematur
- Worker pool ngambil dari channel `jobs`, tiap job: cek `MaxPages`, cek robots.txt, rate limit global (`time.Ticker`), `Fetch()`, lalu `enqueue()` link baru yang ditemuin (filter same-domain + dedup via `visited` map)

`Options{MaxDepth, MaxPages, SameDomainOnly, Delay, Concurrency, RespectRobots}` — semua pengaturan crawl dikumpulin di satu struct.

---

## 6. `main.go` — Orkestrasi 3 Tahap

### a) Flag

```go
host        := flag.String("host", "", "target host")
portsSpec   := flag.String("ports", "1-1024", "ports to scan")
workers     := flag.Int("workers", 100, "scan concurrency")
timeout     := flag.Duration("timeout", 2*time.Second, "dial timeout")
showClosed  := flag.Bool("show-closed", false, "show closed ports too")
output      := flag.String("output", "", "CSV output path")

runCrawl      := flag.Bool("crawl", false, "crawl open http/https ports")
crawlDepth    := flag.Int("crawl-depth", 1, "crawl depth")
crawlMaxPages := flag.Int("crawl-max-pages", 20, "max pages per origin")
crawlWorkers  := flag.Int("crawl-workers", 4, "crawler concurrency")

runHTTPCheck := flag.Bool("httpcheck", false, "run web misconfig check on open http/https ports")

flag.Parse()
```

### b) Tahap 1 — Scan

```go
ports, err := scanner.ParsePorts(*portsSpec)
if err != nil {
    fmt.Println("error:", err)
    os.Exit(1)
}

var results []scanner.Result
for r := range scanner.Scan(*host, ports, scanner.Options{
    Concurrency: *workers,
    Timeout:     *timeout,
    GrabBanner:  true,
}) {
    results = append(results, r)
}
scanner.SortResults(results)

for _, r := range results {
    if r.Open || *showClosed {
        printResult(r)
    }
}
if *output != "" {
    writeCSV(*output, results)
}
```

### c) Tahap 2 — Crawl (opsional, `-crawl`)

```go
if *runCrawl {
    for _, r := range results {
        if !r.Open || (r.Service != "http" && r.Service != "https") {
            continue
        }
        startURL := fmt.Sprintf("%s://%s:%d", r.Service, *host, r.Port)
        fmt.Printf("\n--- Crawling %s ---\n", startURL)

        for cr := range crawler.Crawl(startURL, crawler.Options{
            MaxDepth:       *crawlDepth,
            MaxPages:       *crawlMaxPages,
            SameDomainOnly: true,
            Concurrency:    *crawlWorkers,
            RespectRobots:  true,
        }) {
            fmt.Println(crawler.FormatResult(cr))
        }
    }
}
```

### d) Tahap 3 — HTTP Misconfig Check (opsional, `-httpcheck`)

```go
if *runHTTPCheck {
    fmt.Println("\n--- Web Misconfig Check ---")
    for _, r := range results {
        if !r.Open || (r.Service != "http" && r.Service != "https") {
            continue
        }
        check := scanner.CheckHTTP(*host, r.Port, r.Service)
        fmt.Print(scanner.FormatHTTPCheck(check))
    }
}
```

Ketiga tahap independen lewat flag (`-crawl`, `-httpcheck`) — bisa jalanin scan doang, scan+crawl, scan+httpcheck, atau full pipeline sekaligus.

---

## 7. Perbandingan Pola Concurrency: `scanner` vs `crawler`

| Aspek | `scanner` | `crawler` |
|---|---|---|
| Jumlah kerjaan total | Diketahui dari awal (semua port) | Gak diketahui dari awal (link baru ketemu pas jalan) |
| Cara menutup job channel | Langsung `close(jobs)` setelah diisi | Butuh `sync.WaitGroup` buat nunggu kerjaan baru berhenti muncul |
| Kebutuhan `sync.Mutex` | Gak perlu (`probe()` gak ada shared state) | Perlu (`visited` map, `fetchCount` counter) |
| Urutan hasil | Dikumpulin dulu, di-sort baru ditampilin | Langsung di-stream begitu ada hasil |

Pelajaran: **pola concurrency yang tepat tergantung sifat masalah**. Kerjaan yang jumlahnya pasti dari awal (scan range port) gak butuh mekanisme serumit crawler yang kerjaannya "beranak-pinak" saat jalan.

---

## 8. Cara Pakai

```bash
# Scan doang
go run main.go -host scanme.nmap.org -ports 1-1024

# Scan + crawl port web yang kebuka
go run main.go -host scanme.nmap.org -ports 1-1024 -crawl -crawl-depth 2

# Scan + misconfig check
go run main.go -host scanme.nmap.org -ports 80,443 -httpcheck

# Full pipeline + simpan CSV
go run main.go -host scanme.nmap.org -ports 1-1024 -crawl -httpcheck -output results.csv
```

> Testing cuma boleh ke host yang lo punya izin — `scanme.nmap.org` disediakan resmi buat latihan port scanning.

---

## 9. Ringkasan Konsep Go

| Konsep | Dipakai di mana | Fungsinya |
|---|---|---|
| `struct` | `Page`, `Options`, `job`, `Result`, `HTTPCheckResult` | Mengelompokkan data terkait |
| `goroutine` (`go func()`) | tiap worker di `Scan()` & `Crawl()` | Jalan paralel/asynchronous |
| `channel` (`chan`) | `jobs`, `out` | Komunikasi aman antar-goroutine |
| `sync.WaitGroup` | `pending`, `workerWg` (crawler) | Menunggu goroutine selesai |
| `sync.Mutex` | `visitedMu`, `countMu`, `RobotsChecker.mu` | Mencegah race condition |
| `time.Ticker` | rate limiter di `Crawl()` | Membatasi laju request |
| `net.DialTimeout` | `probe()` | TCP connect scan dengan timeout |
| `regexp` | `fetch.go` | Ekstraksi title/link dari HTML |
| `net/url` | resolve link relatif, parse origin di `httpcheck.go` | Parsing & manipulasi URL |
| `crypto/tls` | `httpcheck.go` | Baca versi TLS dari response |

---

## 10. Roadmap / Belum Selesai

- [ ] `ARCHITECTURE.md` — diagram lengkap alur 3 tahap (belum dibuat)
- [ ] Final zip packaging buat submission
- [ ] Output JSON di samping CSV
- [ ] `httpcheck` bisa nerima list URL hasil crawl (bukan cuma origin) buat check per-endpoint, bukan cuma root `/`

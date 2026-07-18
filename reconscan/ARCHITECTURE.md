# ARCHITECTURE.md — reconscan

Dokumen ini fokus ke **desain arsitektur**, bukan cara pakai (itu ada di `README.md`). Isinya: kenapa dipecah begini, gimana data ngalir antar modul, dan keputusan desain yang diambil sengaja.

---

## 1. Prinsip Desain

Tiga hal yang jadi dasar semua keputusan di project ini:

1. **Standard library only** — gak ada dependency eksternal sama sekali, karena keterbatasan network access saat development. Semua fitur (HTTP client, TCP dial, regex, TLS inspection) pakai `net`, `net/http`, `crypto/tls`, `regexp`, `net/url` bawaan Go.
2. **Separation of concerns per package** — tiap package (`scanner`, `crawler`) punya satu tanggung jawab jelas, gak saling tau detail implementasi satu sama lain, cuma komunikasi lewat tipe data publik (`Result`, `Options`).
3. **Pipeline, bukan tiga tool lepas** — `main.go` yang jadi orkestrator, nyambungin output satu tahap jadi input tahap berikutnya. Tiap tahap tetap bisa dimatiin lewat flag (`-crawl`, `-httpcheck`) kalau cuma butuh sebagian.

---

## 2. Diagram Arsitektur

```
┌─────────────────────────────────────────────────────────────────┐
│                            main.go                               │
│                     (CLI flags + orkestrasi)                     │
└───────────────────────────┬───────────────────────────────────────┘
                             │
                 ┌───────────▼────────────┐
                 │   TAHAP 1: DISCOVERY    │
                 │   package scanner       │
                 │                         │
                 │  ParsePorts(spec)       │
                 │        │                │
                 │        ▼                │
                 │  Scan(host, ports) ──┐  │
                 │        │             │  │
                 │        ▼             │  │
                 │  worker pool         │  │
                 │  (net.DialTimeout)   │  │
                 │        │             │  │
                 │        ▼             │  │
                 │  grabBanner()        │  │
                 │        │             │  │
                 │        ▼             │  │
                 │  chan Result ────────┘  │
                 └───────────┬────────────┘
                             │ []Result{Port, Open, Service, Banner}
                             │ (filter: Open == true && Service ∈ {http, https})
                 ┌───────────▼────────────┐
                 │  TAHAP 2: ENUMERATION   │
                 │   package crawler       │
                 │                         │
                 │  Crawl(originURL) ───┐  │
                 │        │             │  │
                 │        ▼             │  │
                 │  job queue (BFS)     │  │
                 │        │             │  │
                 │        ▼             │  │
                 │  robots.Allowed()    │  │
                 │        │             │  │
                 │        ▼             │  │
                 │  Fetch() ────────────┘  │
                 │        │                │
                 │        ▼                │
                 │  chan Result (stream)   │
                 └───────────┬────────────┘
                             │ (paralel, independen dari tahap 3)
                 ┌───────────▼────────────┐
                 │  TAHAP 3: ASSESSMENT    │
                 │   package scanner       │
                 │   (httpcheck.go)        │
                 │                         │
                 │  CheckHTTP(origin) ──┐  │
                 │        │             │  │
                 │        ▼             │  │
                 │  GET / ──────────────┘  │
                 │        │                │
                 │        ▼                │
                 │  header/cookie/TLS      │
                 │  + probe exposed paths  │
                 └───────────┬────────────┘
                             │
                             ▼
                    HTTPCheckResult → print
```

**Penting:** tahap 2 dan 3 sama-sama makan output tahap 1 (list port yang open), tapi **gak saling bergantung satu sama lain** — keduanya independen, cuma kebetulan dijalanin berurutan di `main.go` untuk kesederhanaan kode. Kalau salah satu flag (`-crawl` atau `-httpcheck`) mati, yang lain tetap jalan normal.

---

## 3. Kenapa Dipecah 2 Package (`scanner` vs `crawler`), Bukan 1?

| Alasan | Penjelasan |
|---|---|
| **Model kerjaan beda** | `scanner` tau semua "kerjaan" (port) dari awal. `crawler` nemuin kerjaan baru (link) di tengah proses. Ini bikin mekanisme penutupan channel-nya beda total (lihat bagian 4) — makin masuk akal dipisah daripada dipaksa satu abstraksi. |
| **Siklus hidup data beda** | `scanner.Result` itu flat (1 struct per port, gak ada relasi antar-hasil). `crawler.Result` punya relasi pohon (halaman A nemuin link ke halaman B) yang butuh state tambahan (`visited` map, depth tracking). |
| **Reusability** | Kalau nanti mau bikin tool lain yang cuma butuh crawl doang (tanpa port scan), package `crawler` bisa langsung dipindah tanpa nyeret kode scanner yang gak relevan. |

`httpcheck.go` sengaja ditaro di package `scanner` (bukan bikin package ke-3), karena secara konsep dia masih "mengassess hasil scan" — 1 origin, 1 check, gak ada concurrency job-queue seruwet crawler.

---

## 4. Perbandingan Model Concurrency

Ini bagian paling penting buat dipahami — dua model beda buat dua sifat masalah beda:

### `scanner.Scan()` — kerjaan diketahui semua dari awal

```go
jobs := make(chan int, len(ports))
for _, p := range ports {
    jobs <- p
}
close(jobs)   // ← langsung ditutup, gak nunggu apa-apa
```

Karena `len(ports)` udah pasti dari awal, channel bisa diisi penuh dan ditutup di tempat. Worker tinggal `range jobs` sampai habis, otomatis stop.

### `crawler.Crawl()` — kerjaan "beranak-pinak" saat proses jalan

```go
var pending sync.WaitGroup

enqueue := func(u string, depth int) {
    pending.Add(1)
    jobs <- job{url: u, depth: depth}
}

enqueue(startURL, 0)  // ← WAJIB synchronous, bukan di goroutine baru

go func() {
    pending.Wait()   // nunggu SEMUA kerjaan (termasuk yang baru ketemu) beres
    close(jobs)
}()
```

Di sini channel `jobs` **gak bisa** langsung ditutup setelah job pertama, karena tiap halaman yang di-fetch bisa nemuin link-link baru yang juga perlu masuk antrian. `sync.WaitGroup` dipakai sebagai "penghitung kerjaan belum selesai" — begitu hitungan balik ke nol (semua halaman udah di-proses dan gak ada link baru lagi), baru channel ditutup.

**Bug klasik yang dihindari:** `enqueue(startURL, 0)` harus dipanggil **synchronous** sebelum goroutine penutup channel jalan. Kalau dipanggil di goroutine terpisah, ada race condition: goroutine penutup bisa keburu cek `pending.Wait()` == 0 (karena job pertama belum sempat `Add(1)`) dan nutup channel sebelum crawling beneran mulai.

### Kenapa `probe()` (scanner) gak butuh Mutex, tapi worker crawler butuh?

`probe()` di `scanner` gak nulis ke shared state apapun — tiap panggilan kerja sepenuhnya sama data lokalnya sendiri (`host`, `port` yang di-passing sebagai parameter, hasil di-return, bukan ditulis ke variable bersama).

Worker `crawler` nulis ke **shared state** yang diakses banyak goroutine sekaligus:
- `visited map[string]bool` — dua goroutine bisa nemuin link yang sama bersamaan, harus dicek "udah pernah apa belum" dengan aman → `sync.Mutex`
- `fetchCount int` — counter global buat `MaxPages`, ditambah dari banyak goroutine → `sync.Mutex`
- `RobotsChecker.cache` — di-baca/ditulis dari banyak worker sekaligus → `sync.Mutex`

Ini prinsip umum Go: **butuh Mutex kalau ada data yang ditulis bersama oleh >1 goroutine**; kalau tiap goroutine cuma kerja sama data lokalnya sendiri, gak perlu sinkronisasi apapun.

---

## 5. Kenapa Hasil Scan Dikumpulin Dulu (Bukan Streaming), tapi Crawl Streaming Langsung?

```go
// scanner: dikumpulin dulu
var results []scanner.Result
for r := range scanner.Scan(...) {
    results = append(results, r)
}
scanner.SortResults(results)   // ← baru bisa sort setelah semua data lengkap
```

```go
// crawler: langsung di-print begitu ada hasil
for cr := range crawler.Crawl(...) {
    fmt.Println(crawler.FormatResult(cr))
}
```

Alasannya murni soal **urutan tampilan yang diinginkan**:
- Port scan enak dilihat **terurut dari nomor kecil ke besar** — itu cuma bisa dilakuin kalau semua data udah lengkap dulu (gak bisa "sort sebagian").
- Crawl gak butuh urutan tertentu (halaman mana duluan gak penting secara fungsional), jadi lebih enak langsung stream — user bisa lihat progress real-time tanpa nunggu semua halaman selesai di-crawl.

---

## 6. Kenapa `httpcheck` Cuma Cek Origin (`scheme://host:port`), Bukan Tiap URL Hasil Crawl?

Keputusan desain saat ini: `CheckHTTP()` cuma nge-hit root path (`/`) dari tiap origin yang ditemuin scanner, **bukan** tiap URL yang ditemuin crawler. Ini simplifikasi yang disengaja untuk versi awal:

- Security header & TLS version biasanya konsisten di seluruh origin (server-level config), jadi cek 1x di root udah cukup representative.
- Exposed path check (`.env`, `.git/HEAD`, dll) juga origin-level, bukan per-halaman.

**Keterbatasan yang disadari** (dicatat di roadmap `README.md`): kalau ada endpoint spesifik yang beda config-nya (misal API subdirectory dengan CORS berbeda), versi saat ini gak bakal nangkep itu. Perbaikan ke depan: terima list URL hasil `crawler.Crawl()` sebagai input `CheckHTTP`, bukan cuma origin — supaya per-endpoint check juga jalan.

---

## 7. Alur Error Handling

Prinsip umum: **gagal di 1 bagian gak boleh nge-crash keseluruhan pipeline.**

| Kegagalan | Perilaku |
|---|---|
| Port gagal di-dial (`probe`) | `Result{Open: false}`, lanjut ke port berikutnya — bukan fatal error |
| `robots.txt` gagal di-fetch | Dianggap "semua diizinkan" (fail-open), crawling tetap lanjut |
| Satu halaman gagal di-fetch (`crawler.Fetch`) | `Result{Err: ...}` dikirim ke channel, worker lain tetap jalan |
| `httpcheck` gagal connect ke origin | `HTTPCheckResult{Err: ...}`, `FormatHTTPCheck` print "skipped", lanjut ke port berikutnya |

Ini penting khususnya buat `-ports 1-65535` scan range gede — satu port yang nge-hang gak boleh bikin seluruh scan macet nunggu dia.

---

## 8. Batasan yang Disadari (Known Limitations)

- **TCP connect scan, bukan SYN scan** — lebih lambat dan lebih gampang ke-log di server target dibanding SYN scan (yang butuh raw socket, gak available lewat `net` standard library tanpa privilege khusus).
- **Service detection cuma dari nomor port** (`services.go`) — bukan verifikasi protokol beneran. Service aneh di port non-standar gak ke-detect dengan benar tanpa banner grabbing tambahan.
- **HTML parsing pakai regex**, bukan parser DOM beneran — bisa salah di HTML yang malformed atau ada komentar yang keliatan kayak tag.
- **`httpcheck` origin-level saja** — lihat bagian 6.
- **Rate limiting crawler bersifat global**, bukan per-domain — kalau suatu saat mau crawl multi-domain sekaligus dalam 1 run, perlu penyesuaian supaya tiap domain punya limiter sendiri.
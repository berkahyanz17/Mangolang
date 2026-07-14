# webcrawler

Web crawler/scraper sederhana pakai Go, **stdlib only** (`net/http` +
`regexp`) — tanpa dependency eksternal, jadi `go build` langsung jalan
tanpa perlu download apa-apa.

Fiturnya:
- Fetch halaman, extract `<title>` dan semua link (`<a href="...">`)
- Crawl berantai (breadth-first) sampai kedalaman (`depth`) yang lo tentuin
- Bisa dibatasi cuma follow link di domain yang sama
- **Concurrent fetching** — beberapa halaman di-fetch sekaligus lewat worker pool
- **Cek `robots.txt` otomatis** — halaman yang di-disallow bakal di-skip
- Delay antar-request (global rate limit, tetep sopan walau workers banyak)
- Simpen hasil ke CSV (opsional)

## Struktur

```
webcrawler/
├── main.go                       # CLI: parsing flag, jalanin crawl, print + save
├── go.mod
└── internal/
    └── crawler/
        ├── fetch.go               # fetch 1 halaman + extract title/link
        └── crawl.go               # BFS crawl logic (depth, visited set, dll)
```

## Cara menjalankan

```bash
go build -o webcrawler.exe .
```

### Contoh pakai

```bash
# crawl 1 halaman doang (depth 0), gak follow link sama sekali
.\webcrawler.exe -url https://example.com -depth 0

# follow link sampai 1 tingkat, maksimal 20 halaman
.\webcrawler.exe -url https://example.com -depth 1 -max-pages 20

# simpen hasil ke CSV
.\webcrawler.exe -url https://example.com -depth 1 -output hasil.csv

# jangan batasi ke domain yang sama (ikutin link keluar juga)
.\webcrawler.exe -url https://example.com -depth 1 -same-domain=false
```

### Semua flag

| Flag           | Default | Keterangan                                         |
|-----------------|---------|------------------------------------------------------|
| `-url`          | (wajib) | URL awal buat mulai crawl                             |
| `-depth`        | `1`     | Berapa "lompatan" link yang di-follow dari halaman awal |
| `-max-pages`    | `20`    | Batas aman total halaman yang di-fetch                |
| `-same-domain`  | `true`  | Cuma follow link di domain yang sama                   |
| `-delay-ms`     | `300`   | Jeda antar-request (ms), global — biar sopan ke server |
| `-output`       | (kosong)| Path file CSV buat nyimpen hasil (opsional)            |
| `-workers`      | `4`     | Jumlah goroutine yang fetch halaman secara paralel     |
| `-respect-robots`| `true` | Cek `robots.txt` dulu, skip URL yang di-disallow       |

## Cara kerja singkat

1. `main.go` parse flag, panggil `crawler.Crawl(url, opts)`
2. `Crawl` jalan pakai **worker pool**: sejumlah `-workers` goroutine
   nge-fetch halaman secara paralel dari satu antrian (channel) yang
   sama, bukan satu-satu berurutan kayak versi awal.
3. Tiap halaman yang berhasil/gagal/di-skip di-fetch dikirim lewat
   **channel** ke `main.go`, yang langsung print ke terminal dan (kalau
   diminta) nulis baris baru ke CSV
4. `Fetch()` di `fetch.go` yang beneran download halaman: dia pake
   `regexp` buat nyari pola `<title>...</title>` dan
   `<a href="...">` — bukan HTML parser beneran (kayak `x/net/html`),
   tapi cukup buat kebanyakan halaman HTML standar. Trade-off-nya: bisa
   miss/salah pada HTML yang aneh/malformed, tapi gak butuh dependency
   luar.
5. `robots.go` fetch & cache `robots.txt` per-host, cek apakah suatu URL
   boleh di-crawl sebelum benar-benar di-fetch. Kalau di-disallow, URL
   itu di-skip (muncul di output sebagai `SKIPPED ... blocked by
   robots.txt`), bukan error fatal.

### Soal concurrency (buat yang mau paham lebih dalam)

`crawl.go` pakai beberapa primitif Go buat ngatur banyak goroutine yang
kerja bareng:

- **`chan job`** — antrian kerja bersama; semua worker `range` dari
  channel yang sama, Go otomatis bagi-bagi job-nya (gak ada dua worker
  yang dapet job yang sama)
- **`sync.WaitGroup` (`pending`)** — nge-track "berapa job yang masih
  ngantri/lagi dikerjain". Begitu itungannya balik ke nol, artinya
  crawl udah selesai total, jadi channel job-nya ditutup
- **`sync.Mutex`** (dua: `visitedMu`, `countMu`) — ngelindungin data yang
  dipakai bareng-bareng (peta URL yang udah dikunjungin, counter jumlah
  halaman yang di-fetch), biar gak ada dua goroutine yang nulis
  bersamaan (race condition)
- **`time.Ticker`** dipake sebagai **global rate limiter** — walaupun
  `-workers` di-set tinggi, semua worker tetep berbagi satu "jatah
  waktu" yang sama buat `-delay-ms`, jadi nambahin worker bikin lebih
  banyak koneksi paralel, bukan nge-spam server lebih cepet

Ini contoh bagus buat belajar goroutine + channel + WaitGroup + Mutex —
4 hal yang paling sering muncul kalau lo mulai nulis kode Go yang
concurrent.

## Etika crawling (penting!)

- Selalu delay antar-request (`-delay-ms`), jangan hajar server orang
  dengan request bertubi-tubi
- Cek `robots.txt` situs target secara manual dulu sebelum crawl skala
  besar (tool ini **belum** otomatis cek robots.txt)
- Jangan crawl situs yang gak lo punya izin buat di-scrape, terutama
  yang butuh login atau ada Terms of Service yang melarang scraping
- Tool ini didesain buat belajar/testing di situs milik sendiri atau
  situs publik yang memang boleh di-scrape

## Langkah selanjutnya (kalau mau lanjutin)

- Ganti regex parsing jadi HTML parser beneran (`golang.org/x/net/html`)
  biar lebih robust — butuh akses internet buat `go get`
- Extract data lain juga (gambar, meta description, dll), bukan cuma
  title & link
- Dukung wildcard (`*`, `$`) di parsing robots.txt (sekarang cuma
  prefix-matching sederhana)
- Tambah retry otomatis kalau fetch gagal karena timeout sementara

# webcrawler

Web crawler/scraper sederhana pakai Go, **stdlib only** (`net/http` +
`regexp`) — tanpa dependency eksternal, jadi `go build` langsung jalan
tanpa perlu download apa-apa.

Fiturnya:
- Fetch halaman, extract `<title>` dan semua link (`<a href="...">`)
- Crawl berantai (breadth-first) sampai kedalaman (`depth`) yang lo tentuin
- Bisa dibatasi cuma follow link di domain yang sama
- Delay antar-request (biar sopan, gak nge-DDoS server orang)
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
| `-delay-ms`     | `300`   | Jeda antar-request (ms) — biar sopan ke server         |
| `-output`       | (kosong)| Path file CSV buat nyimpen hasil (opsional)            |

## Cara kerja singkat

1. `main.go` parse flag, panggil `crawler.Crawl(url, opts)`
2. `Crawl` jalan di goroutine terpisah, pake BFS (queue) — mulai dari
   URL awal, tiap iterasi fetch 1 halaman, ambil semua link-nya, masukin
   ke antrian buat di-fetch berikutnya (kalau belum ngelewatin `depth`
   dan belum pernah dikunjungin)
3. Tiap halaman yang berhasil/gagal di-fetch dikirim lewat **channel**
   ke `main.go`, yang langsung print ke terminal dan (kalau diminta)
   nulis baris baru ke CSV
4. `Fetch()` di `fetch.go` yang beneran download halaman: dia pake
   `regexp` buat nyari pola `<title>...</title>` dan
   `<a href="...">` — bukan HTML parser beneran (kayak `x/net/html`),
   tapi cukup buat kebanyakan halaman HTML standar. Trade-off-nya: bisa
   miss/salah pada HTML yang aneh/malformed, tapi gak butuh dependency
   luar.

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

- Tambah cek `robots.txt` otomatis sebelum crawl
- Ganti regex parsing jadi HTML parser beneran (`golang.org/x/net/html`)
  biar lebih robust — butuh akses internet buat `go get`
- Tambah concurrency (fetch beberapa halaman sekaligus, bukan satu-satu)
  pake goroutine pool + rate limiter
- Extract data lain juga (gambar, meta description, dll), bukan cuma
  title & link

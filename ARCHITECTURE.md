# webcrawler — Dokumentasi Lengkap Repo

Dokumen ini jelasin **seluruh isi repo** `webcrawler`, file per file, fungsi
per fungsi — biar lo paham bukan cuma "cara jalaninnya" tapi juga "kenapa
kodenya begini". Cocok dibaca bareng sambil buka kode aslinya di editor.

---

## 1. Peta Keseluruhan

```
webcrawler/
├── go.mod                          # identitas module Go
├── main.go                         # entrypoint CLI
├── README.md                       # dokumentasi ringkas + cara pakai
└── internal/
    └── crawler/
        ├── fetch.go                 # download 1 halaman + extract title/link
        ├── crawl.go                 # orkestrasi crawl (worker pool, BFS, dll)
        └── robots.go                # fetch & cek aturan robots.txt
```

Alur data secara garis besar:

```
main.go
  │  parse flag (-url, -depth, -workers, dll)
  ▼
crawler.Crawl()  ← crawl.go
  │  bikin worker pool, tiap worker manggil...
  ▼
crawler.Fetch()  ← fetch.go
  │  download halaman, extract title & link
  ▼
  (link-link baru dimasukin lagi ke antrian crawl.go)
  │
  ▼
hasil dikirim balik lewat channel ke main.go → di-print + disimpen ke CSV
```

Kenapa dipisah jadi 3 file di `internal/crawler/`? Ini prinsip Go yang
disebut **separation of concerns**:
- `fetch.go` cuma tau cara "ambil 1 halaman", gak peduli soal crawling
  berantai
- `robots.go` cuma tau cara "cek robots.txt", gak peduli soal fetch
  halaman
- `crawl.go` yang "ngatur" — manggil `Fetch()` dan `robots.Allowed()`
  berulang-ulang sesuai strategi (BFS, depth limit, concurrency)

Kalau nanti mau ganti cara fetch (misal pakai HTML parser beneran), lo
cuma perlu ubah `fetch.go` — `crawl.go` dan `main.go` gak perlu disentuh
sama sekali.

---

## 2. `go.mod`

```
module webcrawler

go 1.22
```

- `module webcrawler` — nama "identitas" project ini. Ini yang dipakai
  di statement import, misal `"webcrawler/internal/crawler"` di
  `main.go` artinya "package `crawler` yang ada di folder
  `internal/crawler` milik module `webcrawler`".
- `go 1.22` — versi minimum Go yang dibutuhkan.

---

## 3. `main.go` — Entrypoint & CLI

Ini "pintu masuk" program. Isinya 3 bagian:

### a) Parsing flag command-line

```go
url := flag.String("url", "", "starting URL to crawl (required)")
depth := flag.Int("depth", 1, "...")
```

Package `flag` (standard library) yang bikin `-url`, `-depth`, dll bisa
dipanggil dari terminal. `flag.String("url", "", "...")` artinya:
- nama flag: `url` (dipanggil sebagai `-url`)
- default value: `""` (kosong)
- deskripsi: buat ditampilin kalau user manggil `-h`

Semua fungsi `flag.XXX(...)` mengembalikan **pointer** (`*string`,
`*int`, `*bool`), makanya di bagian bawah kode ini selalu ada tanda
bintang `*url`, `*depth` — itu buat "membuka" pointer-nya jadi nilai
aslinya.

`flag.Parse()` — wajib dipanggil setelah semua flag didefinisikan,
sebelum flag-flag itu dipakai. Ini yang beneran baca `os.Args` dan isi
nilai ke variabel-variabel di atas.

### b) Validasi & setup

```go
if *url == "" {
    fmt.Println("usage: ...")
    os.Exit(1)
}
```

Kalau user gak kasih `-url`, program keluar dengan pesan error (exit
code 1 = ada kesalahan, konvensi umum di command-line tools).

```go
opts := crawler.Options{...}
```

Semua flag yang udah di-parse dikumpulin jadi satu struct `Options` (dari
package `crawler`), lalu dilempar ke `crawler.Crawl()`. Ini contoh pola
umum di Go: daripada 1 fungsi punya 7 parameter yang bikin bingung
urutannya, mending dibungkus 1 struct.

### c) Menjalankan crawl & menangani hasil

```go
for r := range crawler.Crawl(*url, opts) {
    fmt.Println(crawler.FormatResult(r))
    ...
}
```

`crawler.Crawl(...)` mengembalikan sebuah **channel** (`<-chan
Result`). `for r := range someChannel` adalah cara Go buat "terima data
terus-menerus dari channel sampai channel-nya ditutup". Ini penting
dipahami: `main.go` gak perlu tau kapan crawling selesai — dia cuma
nunggu data masuk lewat channel, dan loop-nya otomatis berhenti begitu
`crawl.go` menutup channel-nya (nanti dijelasin di bagian `crawl.go`).

Setiap hasil (`r`) yang diterima:
1. Di-print ke terminal lewat `crawler.FormatResult(r)`
2. Kalau `-output` diisi, ditulis juga sebagai baris baru di file CSV

---

## 4. `internal/crawler/fetch.go` — Download & Extract 1 Halaman

### Struct `Page`

```go
type Page struct {
    URL    string
    Title  string
    Links  []string
    Status int
}
```

Ini "bentuk data" hasil nge-fetch 1 halaman: URL-nya, judul halaman
(dari tag `<title>`), semua link yang ditemukan, dan HTTP status code
(200, 404, dst).

### Regex yang dipakai

```go
hrefRe  = regexp.MustCompile(`(?i)<a\s+[^>]*href\s*=\s*["']([^"'#]+)["']`)
titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
```

- `(?i)` = case-insensitive (`<A HREF=` juga ke-detect, bukan cuma
  `<a href=`)
- `(?s)` = "dot matches newline" — buat `<title>` yang isinya lebih dari
  1 baris
- `hrefRe` nyari pola `<a ... href="sesuatu">` dan nge-capture bagian
  "sesuatu"-nya (grup di dalam kurung `(...)`)
- `[^"'#]+` artinya "karakter apa aja selain tanda kutip dan `#`" — ini
  buat exclude anchor link kayak `#section2` dari hasil, karena itu
  bukan URL ke halaman lain

**Kenapa pakai regex, bukan HTML parser beneran?** Karena parser HTML
proper (`golang.org/x/net/html`) itu library eksternal yang butuh
`go get` dari internet — dan kayak yang udah kejadian pas awal, jaringan
kadang dibatasi/gak stabil. Regex ini "cukup baik" buat kebanyakan HTML
normal, tapi bisa salah di kasus HTML yang aneh (misal ada komentar HTML
yang isinya kayak tag beneran). Trade-off yang disengaja.

### Fungsi `Fetch(rawURL string) (Page, error)`

Alurnya:

1. **Bikin HTTP request manual** (`http.NewRequest`, bukan langsung
   `http.Get`) — ini biar bisa nambahin header `User-Agent` custom,
   buat identifikasi diri secara jujur ke server (etika crawling).

2. **Kirim request** lewat `httpClient` (variabel package-level yang
   di-reuse tiap kali `Fetch` dipanggil — ini best practice di Go,
   jangan bikin `http.Client` baru tiap request, karena `http.Client`
   udah dirancang buat dipakai berulang & aman dipakai dari banyak
   goroutine sekaligus).

3. **Cek Content-Type** — kalau bukan `text/html` (misal PDF, gambar),
   langsung return error, gak usah diproses lebih jauh.

4. **Baca body dengan batas ukuran**:
   ```go
   body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
   ```
   `io.LimitReader` membatasi maksimal 5MB dibaca. Ini "safety net" —
   kalau ada halaman yang gede banget atau server ngirim data tanpa
   henti, program gak bakal makan memory tak terbatas.

5. **Extract title** pakai `titleRe.FindStringSubmatch(html)` — cari
   match pertama, ambil grup capture ke-1 (`m[1]`), lalu
   `collapseWhitespace` buat ngerapiin whitespace/newline berlebih di
   dalam title.

6. **Extract & resolve semua link**:
   ```go
   base, _ := url.Parse(rawURL)
   ...
   resolved, err := base.Parse(link)
   ```
   Ini bagian penting: link di HTML sering ditulis **relatif**, misal
   `href="/about"` atau `href="../contact"`. `base.Parse(link)` (dari
   package `net/url`) yang nge-convert link relatif itu jadi URL
   absolut berdasarkan URL halaman saat ini. Contoh: kalau `base` =
   `https://example.com/blog/post1` dan link-nya `../about`, hasilnya
   jadi `https://example.com/about`.

   Ada juga filter: skip `javascript:...`, skip `mailto:...`, skip
   selain skema `http`/`https`, dan pakai `map[string]bool` (`seen`)
   biar link duplikat di 1 halaman gak dimasukin dua kali.

### Fungsi kecil `collapseWhitespace`

Cuma `strings.Fields` (pecah by whitespace apapun) lalu `strings.Join`
lagi pakai spasi tunggal — trik umum buat "merapikan" teks yang ada
banyak newline/tab/spasi berlebih jadi satu baris rapi.

---

## 5. `internal/crawler/robots.go` — Cek `robots.txt`

### Kenapa perlu?

`robots.txt` adalah file konvensi yang situs pasang di
`https://situs.com/robots.txt` buat ngasih tau crawler bagian mana yang
boleh/gak boleh diakses. Crawler yang "sopan" harus cek ini dulu sebelum
nge-fetch halaman.

### Struct `RobotsChecker`

```go
type RobotsChecker struct {
    mu     sync.Mutex
    cache  map[string]*robotsRules
    client *http.Client
}
```

- `cache` — nyimpen hasil parse `robots.txt` per-host, biar gak perlu
  fetch ulang tiap kali ada URL baru dari host yang sama
- `mu sync.Mutex` — karena `RobotsChecker` ini dipanggil dari banyak
  goroutine (worker) sekaligus di `crawl.go`, `cache`-nya harus
  dilindungi biar gak race condition (dua goroutine nulis ke map yang
  sama secara bersamaan = crash/data corrupt di Go)

### Fungsi `Allowed(rawURL string) bool`

Alur singkatnya:
1. Parse URL buat dapetin `scheme` dan `host`
2. Ambil rules-nya (fetch kalau belum ada di cache) lewat `rulesFor`
3. Cari aturan `Disallow`/`Allow` yang paling **spesifik** (path
   ter-panjang yang cocok) — ini sesuai spek resmi robots.txt: kalau ada
   `Disallow: /private` dan `Allow: /private/public`, maka
   `/private/public/page` tetap **boleh** diakses karena aturan
   `Allow`-nya lebih spesifik (lebih panjang).

### Fungsi `fetchAndParse(scheme, host string) *robotsRules`

Ini yang beneran download `robots.txt` dan parse isinya baris per
baris:

```go
scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
    line := strings.TrimSpace(scanner.Text())
    ...
    parts := strings.SplitN(line, ":", 2)
    field := strings.ToLower(strings.TrimSpace(parts[0]))
    value := strings.TrimSpace(parts[1])

    switch field {
    case "user-agent":
        relevant = value == "*"
    case "disallow":
        if relevant { rules.disallow = append(...) }
    case "allow":
        if relevant { rules.allow = append(...) }
    }
}
```

Logikanya: robots.txt berisi "grup" aturan, tiap grup diawali baris
`User-agent: nama-bot`. Kode ini cuma peduli sama grup yang
`User-agent: *` (berlaku buat semua bot, karena crawler kita gak
punya nama khusus terdaftar). Begitu ketemu baris `User-agent:` baru
yang bukan `*`, flag `relevant` jadi `false`, jadi baris
`Disallow`/`Allow` sesudahnya (buat bot lain) diabaikan.

**Fail-open by design**: kalau `robots.txt` gak ketemu (404) atau gagal
di-fetch (timeout, dll), fungsi ini return `nil`, dan `Allowed()`
menganggap `nil` = "semua diizinkan". Ini perilaku standar kebanyakan
crawler — kalau gak ada aturan eksplisit, defaultnya boleh akses.

---

## 6. `internal/crawler/crawl.go` — Orkestrasi Crawl (Bagian Paling Kompleks)

Ini file paling "berat" secara konsep karena ngatur **concurrency**
(banyak hal jalan bersamaan). Dipecah step by step:

### Struct `Options`

```go
type Options struct {
    MaxDepth       int
    MaxPages       int
    SameDomainOnly bool
    Delay          time.Duration
    Concurrency    int
    RespectRobots  bool
}
```

Semua "pengaturan" crawl dikumpulin di sini, diisi dari flag CLI di
`main.go`.

### Struct `job`

```go
type job struct {
    url   string
    depth int
}
```

Ini representasi "1 pekerjaan": satu URL yang perlu di-fetch, beserta
informasi "seberapa jauh" dia dari URL awal (depth).

### Fungsi `Crawl(startURL string, opts Options) <-chan Result`

Mengembalikan **channel** (bukan slice/array biasa) — kenapa? Karena
crawling bisa makan waktu lama (banyak halaman), dan kita mau
`main.go` bisa mulai nge-print hasil **begitu ada yang selesai**,
bukan nunggu SEMUA halaman selesai baru dapet hasilnya sekaligus. Ini
pola umum di Go buat streaming data dari satu goroutine ke goroutine
lain.

Di dalam fungsi ini (yang jalan di goroutine terpisah lewat `go func()
{...}()`), ada beberapa bagian penting:

#### a) Job queue (antrian kerja)

```go
jobs := make(chan job, 1000)
```

Channel buffered dengan kapasitas 1000 — ini "antrian" tempat semua
worker ambil pekerjaan. Kenapa buffered (bukan unbuffered)? Supaya kalau
ada banyak link ditemukan sekaligus, mereka bisa langsung "dimasukin ke
antrian" tanpa nge-block nunggu ada worker yang nganggur.

#### b) Melacak "berapa kerjaan yang belum selesai"

```go
var pending sync.WaitGroup

enqueue := func(u string, depth int) {
    pending.Add(1)
    jobs <- job{url: u, depth: depth}
}
```

`sync.WaitGroup` itu kayak "penghitung" — `Add(1)` nambah hitungan,
`Done()` (dipanggil nanti setelah 1 job selesai diproses) ngurangin
hitungan. Ini dipakai buat tau **kapan semua pekerjaan (termasuk yang
baru ditemukan di tengah jalan) udah beres**, tanpa perlu tau di awal
berapa total halaman yang bakal di-crawl (karena jumlahnya baru
ketauan pas crawling jalan — makin banyak link ditemukan, makin banyak
kerjaan baru).

```go
go func() {
    pending.Wait()
    close(jobs)
}()
```

Goroutine terpisah ini nunggu (`Wait()`) sampai hitungan `pending`
balik ke nol, lalu nutup channel `jobs`. Begitu channel `jobs` ditutup,
semua worker yang lagi `range jobs` otomatis berhenti loop-nya.

> ⚠️ **Detail penting yang gampang ke-miss**: `enqueue(startURL, 0)`
> buat job pertama **harus** dipanggil secara langsung (synchronous),
> BUKAN di dalam goroutine baru. Kalau dijalanin di goroutine terpisah,
> ada kemungkinan (race condition) goroutine "penutup channel" di atas
> keburu ngecek `pending.Wait()` sebelum job pertama sempat
> `Add(1)` — jadinya channel ditutup prematur padahal belum ada kerjaan
> sama sekali yang jalan. Ini contoh bug concurrency klasik yang halus.

#### c) Worker pool

```go
for i := 0; i < opts.Concurrency; i++ {
    workerWg.Add(1)
    go func() {
        defer workerWg.Done()
        for j := range jobs {
            // proses 1 job...
        }
    }()
}
```

Ini yang bikin crawler-nya **concurrent**: bikin sejumlah `-workers`
goroutine, semua nge-`range` dari channel `jobs` yang sama. Go otomatis
"membagi" job-job itu ke worker yang nganggur — dua worker gak akan
pernah kebagian job yang sama.

Di dalam tiap worker, buat 1 job:

1. **Cek batas `-max-pages`** (pakai `countMu` buat lindungin counter
   `fetchCount` dari race condition antar-worker)
2. **Cek robots.txt** (kalau `-respect-robots` aktif) — kalau
   di-disallow, kirim `Result` dengan error `ErrRobotsDisallowed`, skip
   fetch beneran
3. **Tunggu giliran rate limiter** (`<-ticker.C`) — ini yang bikin
   `-delay-ms` berlaku **global** ke semua worker sekaligus, bukan
   per-worker. Jadi kalau `-workers 10 -delay-ms 300`, total request
   per detik tetap dibatasi ~3.3/detik (1000ms/300ms), cuma sekarang
   ada 10 koneksi yang "gantian" pakai jatah itu — hasilnya lebih
   efisien (gak ada waktu nganggur nunggu 1 request selesai baru mulai
   yang berikutnya) tapi tetap sopan ke server.
4. **`Fetch(j.url)`** — panggil fungsi dari `fetch.go`
5. **Kirim `Result` ke channel `out`** — ini yang diterima `main.go`
6. **Kalau belum nyampe `MaxDepth`**, ambil semua link dari halaman
   ini, filter (same-domain check kalau aktif, sudah-pernah-dikunjungi
   check pakai `visitedMu` + map `visited`), lalu `enqueue()` link baru
   itu buat di-proses worker lain nanti

#### d) Menutup channel `out` di akhir

```go
workerWg.Wait()
```
di akhir fungsi (sebelum `defer close(out)` ke-trigger) — ini nunggu
**semua worker goroutine bener-bener selesai** (bukan cuma "job kosong")
sebelum channel `out` ditutup. Kalau `out` ditutup terlalu cepat
padahal masih ada worker yang lagi ngirim data ke situ, program bakal
panic.

### Fungsi `FormatResult(r Result) string`

Cuma fungsi kecil buat format 1 baris output yang enak dibaca di
terminal — ngecek apakah ada error (tampilkan "SKIPPED ... alasan") atau
sukses (tampilkan title + jumlah link).

---

## 7. Ringkasan Konsep Go yang Dipelajari dari Repo Ini

| Konsep                  | Dipakai di mana                          | Fungsinya                                    |
|--------------------------|--------------------------------------------|-----------------------------------------------|
| `struct`                 | `Page`, `Options`, `job`, `Result`         | Mengelompokkan data terkait                   |
| `interface` implisit     | `error` (built-in interface)               | Standar penanganan error di Go                |
| `pointer` (`*T`)         | hasil `flag.String()`, dll                 | Mengakses/mengubah nilai dari fungsi lain     |
| `goroutine` (`go func()`)| tiap worker, `Crawl()`                     | Menjalankan kode secara paralel/asynchronous  |
| `channel` (`chan`)       | `jobs`, `out`                              | Komunikasi aman antar-goroutine                |
| `sync.WaitGroup`         | `pending`, `workerWg`                      | Menunggu sejumlah goroutine selesai            |
| `sync.Mutex`             | `visitedMu`, `countMu`, di `RobotsChecker` | Mencegah race condition pada data bersama      |
| `time.Ticker`            | rate limiter di `Crawl()`                  | Membatasi laju request secara berkala          |
| `regexp`                 | `fetch.go`                                  | Ekstraksi pola teks dari HTML                  |
| `net/url`                | resolve link relatif → absolut             | Parsing & manipulasi URL                       |
| closure (fungsi anonim)  | `enqueue := func(...) {...}`               | Membungkus logic + akses variabel luar         |

---

## 8. Kalau Mau Eksperimen Sendiri

Beberapa hal kecil yang bisa dicoba buat latihan tanpa merusak yang
sudah ada:

1. **Ubah `FormatResult`** biar nunjukin juga HTTP status code di output
   terminal (data-nya udah ada di `r.Status`, tinggal ditambahin ke
   string format-nya)
2. **Tambah flag `-verbose`** yang kalau aktif, nge-print semua link
   yang ditemukan di tiap halaman (bukan cuma jumlahnya)
3. **Coba ubah `MaxPages` jadi 1 dan `Concurrency` jadi 10** — perhatiin
   kalau cuma ada 1 halaman yang boleh di-fetch, nambahin worker gak
   akan bikin lebih cepat (karena gak ada kerjaan buat dibagi)
4. **Baca ulang bagian "worker pool" sambil coba gambar diagram di
   kertas** — gambar 1 kotak channel `jobs`, beberapa kotak "worker",
   dan panah ke channel `out`. Ini cara paling efektif buat ngerti alur
   concurrency yang gak keliatan langsung dari baca kode doang.

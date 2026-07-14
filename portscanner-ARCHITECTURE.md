# portscanner — Dokumentasi Lengkap Repo

Dokumen ini jelasin **seluruh isi repo** `portscanner`, file per file,
fungsi per fungsi. Cocok dibaca bareng sambil buka kode aslinya di
editor.

---

## 1. Peta Keseluruhan

```
portscanner/
├── go.mod                          # identitas module Go
├── main.go                         # entrypoint CLI
├── README.md                       # dokumentasi ringkas + cara pakai
└── internal/
    └── scanner/
        ├── ports.go                 # parsing spec port ("1-1024,80,443")
        ├── services.go              # tabel lookup nama service per port
        └── scan.go                  # logic scan inti (worker pool, dial, banner)
```

Alur data secara garis besar:

```
main.go
  │  parse flag (-host, -ports, -workers, dll)
  ▼
scanner.ParsePorts()  ← ports.go
  │  ubah string "1-1024,8080" jadi []int{1,2,...,1024,8080}
  ▼
scanner.Scan()  ← scan.go
  │  bikin worker pool, tiap worker manggil...
  ▼
scanner.probe() (internal)
  │  net.DialTimeout ke tiap port, cek berhasil/gagal
  │  lookup nama service via services.go
  ▼
hasil dikirim balik lewat channel ke main.go → di-print + disimpen ke CSV
```

Kalau dibandingin sama project `webcrawler` yang udah lo bikin
sebelumnya, pola-nya **mirip banget**: parse input → worker pool
concurrent → channel buat streaming hasil → main.go yang nampilin. Ini
bagus disadari — begitu lo paham 1 pola ini, lo bisa pakai lagi buat
banyak masalah lain (crawler, scanner, downloader paralel, dll).

---

## 2. `go.mod`

```
module portscanner

go 1.22
```

Sama kayak project Go lain: `module portscanner` itu identitas dipakai
di import path (`"portscanner/internal/scanner"`), `go 1.22` versi
minimum Go yang dibutuhin.

---

## 3. `internal/scanner/ports.go` — Parsing Spec Port

### Fungsi `ParsePorts(spec string) ([]int, error)`

Ini yang ngubah string yang user ketik di `-ports` (misal
`"22,80,8000-8100"`) jadi slice angka `[]int` yang bisa di-loop.

Alurnya:
1. `strings.Split(spec, ",")` — pecah dulu berdasarkan koma, jadi
   `["22", "80", "8000-8100"]`
2. Buat tiap bagian, cek apakah mengandung `-` (berarti range):
   - Kalau ada `-`: split lagi jadi start & end, parse ke `int`, lalu
     loop `for p := start; p <= end; p++` buat masukin semua angka di
     antaranya
   - Kalau gak ada `-`: langsung parse 1 angka
3. Pakai `map[int]bool` (`seen`) buat **dedup** — kalau user nulis
   `"80,80,80-82"`, port 80 gak bakal muncul dobel di hasil akhir
4. `validatePort` mastiin angkanya masuk akal (1-65535, batas port TCP
   yang valid)

Kenapa fungsi ini dipisah jadi file sendiri (`ports.go`), bukan digabung
ke `scan.go`? Karena ini murni soal **parsing string**, gak ada urusan
sama network/scanning sama sekali — cocok dites sendiri (unit test)
tanpa perlu jaringan/koneksi apapun.

---

## 4. `internal/scanner/services.go` — Lookup Nama Service

Ini file paling sederhana: cuma `map[int]string` yang isinya nomor
port umum → nama service konvensionalnya (`80` → `"http"`, `22` →
`"ssh"`, dst), dan 1 fungsi `ServiceName(port int) string` buat
nge-lookup ke map itu.

**Penting dipahami**: ini cuma **label**, bukan verifikasi beneran.
Kalau ada service aneh yang sengaja dijalanin di port 80 (misal SSH
di-listen di port 80), `ServiceName(80)` tetap bakal bilang `"http"`,
padahal isinya bukan HTTP. Buat tau beneran apa yang jalan di suatu
port, butuh cara lain (misal banner grabbing, yang dijelasin di bagian
`scan.go`).

---

## 5. `internal/scanner/scan.go` — Logic Scan Inti

Ini file paling penting, mirip pola concurrency yang sama kayak di
`crawl.go` (project `webcrawler`), tapi lebih sederhana karena semua
port udah diketahui dari awal (beda sama crawler yang link barunya baru
ketemu pas jalan).

### Struct `Result` dan `Options`

```go
type Result struct {
    Port    int
    Open    bool
    Service string
    Banner  string
    Err     error
}

type Options struct {
    Concurrency int
    Timeout     time.Duration
    GrabBanner  bool
}
```

Sama kayak pola sebelumnya: `Result` buat "bentuk data 1 hasil", `Options`
buat "pengaturan" yang dikumpulin di 1 struct daripada parameter
terpisah-pisah.

### Fungsi `Scan(host string, ports []int, opts Options) <-chan Result`

```go
jobs := make(chan int, len(ports))
for _, p := range ports {
    jobs <- p
}
close(jobs)
```

Beda penting dari `webcrawler`: di sini semua "pekerjaan" (nomor port
yang mau di-scan) **udah diketahui semuanya dari awal** — beda sama
crawler yang link barunya baru ketemu pas proses jalan. Makanya channel
`jobs` bisa langsung diisi penuh dan **langsung ditutup** saat itu juga
— gak perlu `sync.WaitGroup` buat nge-track "masih ada kerjaan baru gak"
kayak di `crawl.go`. Ini penyederhanaan yang valid karena sifat
masalahnya emang beda.

```go
var wg sync.WaitGroup
for i := 0; i < opts.Concurrency; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for port := range jobs {
            out <- probe(host, port, opts)
        }
    }()
}
wg.Wait()
```

Pola worker pool standar: `-workers` goroutine, semua `range` dari
channel `jobs` yang sama (Go otomatis bagi kerjaannya), tiap worker
manggil `probe()` buat 1 port, kirim hasilnya ke channel `out`.
`wg.Wait()` di akhir mastiin semua worker beneran selesai sebelum
`defer close(out)` jalan.

### Fungsi `probe(host string, port int, opts Options) Result` (private, huruf kecil)

Ini "otak" dari 1 pengecekan port:

```go
conn, err := net.DialTimeout("tcp", address, opts.Timeout)
if err != nil {
    return Result{Port: port, Open: false, ...}
}
defer conn.Close()
```

`net.DialTimeout("tcp", ...)` mencoba bikin **koneksi TCP penuh**
(3-way handshake: SYN → SYN-ACK → ACK) ke `host:port`. Ini yang disebut
**TCP connect scan** — teknik paling dasar dan paling "jujur" dalam
port scanning:
- Kalau **berhasil konek** → port itu ada yang "dengerin" (listening) →
  dianggap **open**
- Kalager **gagal** (connection refused, atau timeout karena di-filter
  firewall) → dianggap **closed/filtered**

Kenapa disebut fungsi private (huruf kecil `probe`, bukan `Probe`)?
Karena ini detail implementasi internal — kode di luar package
`scanner` (misal `main.go`) gak perlu dan gak seharusnya manggil
`probe()` langsung, cukup lewat `Scan()` yang jadi "pintu masuk resmi"
package ini.

### Fungsi `grabBanner(conn net.Conn) string`

```go
conn.SetReadDeadline(time.Now().Add(1 * time.Second))
reader := bufio.NewReader(conn)
line, err := reader.ReadString('\n')
```

Banner grabbing itu teknik "dengerin" apa yang dikirim service begitu
kita konek, **tanpa** kita kirim apa-apa duluan. Banyak service jaman
dulu (SSH, FTP, SMTP, POP3) didesain buat langsung ngirim pesan
"salam pembuka" begitu ada yang konek — misal SSH ngirim
`SSH-2.0-OpenSSH_8.2` duluan sebelum nunggu perintah apapun.

`SetReadDeadline` penting banget di sini — servis yang **gak** ngirim
banner otomatis (misal HTTP, yang nunggu kita kirim `GET /` duluan)
bakal bikin `ReadString` nge-block selamanya kalau gak dikasih batas
waktu. Deadline 1 detik ini yang mastiin `probe()` tetap "gerak maju"
walau service-nya diem aja.

### Fungsi `SortResults(results []Result)`

```go
sort.Slice(results, func(i, j int) bool {
    return results[i].Port < results[j].Port
})
```

Karena hasil scan datang dari banyak goroutine yang jalan paralel,
urutan hasil yang diterima `main.go` **gak berurutan** berdasarkan
nomor port (port 443 bisa aja selesai sebelum port 80, tergantung mana
yang responnya duluan). `sort.Slice` dipakai di `main.go` **setelah**
semua hasil terkumpul, biar output-nya rapi urut dari port kecil ke
besar.

---

## 6. `main.go` — Entrypoint & CLI

### a) Parsing flag

Sama kayak project sebelumnya, pakai package `flag`. Yang agak beda:
`-ports` defaultnya string `"1-1024"` (bukan angka tunggal), karena
formatnya fleksibel (single/range/list).

### b) Validasi & scan

```go
ports, err := scanner.ParsePorts(*portsSpec)
```

Kalau format `-ports` salah ketik (misal `"abc"`), error-nya langsung
ketangkep di sini dengan pesan yang jelas, sebelum proses scan beneran
dimulai.

```go
var results []scanner.Result
for r := range scanner.Scan(*host, ports, opts) {
    results = append(results, r)
}
```

Beda dari `webcrawler` yang langsung print tiap hasil begitu diterima,
di sini semua hasil **dikumpulin dulu** ke slice (`results = append(...)`)
sebelum di-print. Kenapa? Karena scanner mau nampilin hasil **terurut**
berdasarkan nomor port (`scanner.SortResults(results)`), dan itu cuma
bisa dilakuin kalau semua data udah lengkap — beda sama crawler yang
gak butuh urutan tertentu jadi bisa langsung streaming print.

### c) Print & simpan CSV

```go
for _, r := range results {
    if r.Open {
        openCount++
        printResult(r)
    } else if *showClosed {
        printResult(r)
    }
}
```

Default-nya cuma nampilin port yang **open** (biar output gak
kepenuhan ratusan baris "CLOSED" kalau scan range gede) — kecuali
`-show-closed` diaktifin.

`writeCSV` fungsi kecil yang nulis semua `results` (baik open maupun
closed) ke file CSV kalau `-output` diisi — beda dari filter tampilan
terminal, CSV selalu nyimpen semua data biar lengkap buat dianalisis
nanti.

---

## 7. Perbandingan Pola Concurrency: `portscanner` vs `webcrawler`

Ini worth diperhatiin karena nunjukin **kapan pola yang lebih sederhana
cukup**, dan kapan butuh yang lebih canggih:

| Aspek                        | `portscanner`                          | `webcrawler`                              |
|-------------------------------|------------------------------------------|----------------------------------------------|
| Jumlah "kerjaan" total        | Sudah diketahui dari awal (semua port)   | Gak diketahui dari awal (link baru ketemu pas jalan) |
| Cara menutup job channel      | Langsung `close(jobs)` setelah diisi     | Butuh `sync.WaitGroup` buat nunggu semua "kerjaan baru" berhenti muncul |
| Kebutuhan `sync.Mutex`        | Gak perlu (gak ada shared state yang ditulis bareng) | Perlu (`visited` map, `fetchCount` counter ditulis banyak goroutine) |
| Urutan hasil                  | Dikumpulin dulu, di-sort baru ditampilin | Langsung di-stream begitu ada hasil          |

Pelajaran pentingnya: **pola concurrency yang tepat tergantung sifat
masalahnya**. Jangan asal contek pola yang sama ke semua kasus — kalau
kerjaannya udah pasti jumlahnya dari awal (kayak scan range port),
gak perlu mekanisme serumit crawler yang kerjaannya "beranak-pinak"
saat jalan.

---

## 8. Ringkasan Konsep Go yang Dipelajari dari Repo Ini

| Konsep                  | Dipakai di mana                     | Fungsinya                                     |
|--------------------------|----------------------------------------|--------------------------------------------------|
| `net.DialTimeout`        | `probe()`                              | Coba konek TCP dengan batas waktu                 |
| `net.JoinHostPort`       | `probe()`                              | Gabungin host+port jadi `"host:port"` dengan aman (termasuk IPv6) |
| `conn.SetReadDeadline`   | `grabBanner()`                         | Cegah `Read` nge-block selamanya                  |
| worker pool sederhana    | `Scan()`                                | Proses banyak port paralel tanpa nested loop      |
| `sort.Slice`             | `SortResults()`                        | Urutin slice berdasarkan kriteria custom           |
| `bufio.Reader`           | `grabBanner()`                         | Baca data dari koneksi baris per baris             |
| fungsi private (huruf kecil) | `probe()`, `grabBanner()`           | Sembunyiin detail implementasi dari luar package  |

---

## 9. Kalau Mau Eksperimen Sendiri

1. **Tambah `-timeout-ms` yang lebih kecil** (misal `200`) terus scan
   range gede (`1-65535`) — perhatiin trade-off: makin kecil timeout,
   makin cepat tapi makin gampang salah nge-anggep port open jadi
   closed (kalau jaringannya lagi lambat)
2. **Coba scan `scanme.nmap.org`** (situs resmi yang emang disediakan
   buat latihan port scanning, legal dipakai) dan bandingin hasilnya
   sama scan `nmap` beneran kalau lo punya akses ke tools itu
3. **Tambah opsi output JSON** di samping CSV — latihan bikin fungsi
   baru mirip `writeCSV` tapi pakai `encoding/json`
4. **Coba pahami kenapa `probe()` gak butuh mutex** sama sekali,
   padahal jalan di banyak goroutine — hint: perhatiin apakah ada
   variable yang **ditulis bareng-bareng** oleh lebih dari 1 goroutine
   di fungsi itu (jawabannya: gak ada, tiap panggilan `probe()` cuma
   kerja sama data lokalnya sendiri)

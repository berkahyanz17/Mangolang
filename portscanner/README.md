# portscanner

TCP port scanner sederhana pakai Go, **stdlib only** (`net`) — tanpa
dependency eksternal.

Fiturnya:
- **TCP connect scan** — cek port terbuka/tertutup pakai koneksi TCP
  penuh (gak butuh privilege khusus/raw socket, beda sama SYN scan ala
  nmap yang butuh root)
- **Concurrent scanning** — banyak port dicek sekaligus lewat worker pool
- **Service name lookup** — kasih label nama service umum (http, ssh,
  mysql, dll) berdasarkan nomor port
- **Banner grabbing** (opsional) — coba baca "salam pembuka" yang
  dikirim service begitu konek (SSH, FTP, SMTP biasanya kirim ini)
- Dukung range port (`1-1024`), list (`80,443`), atau kombinasi
  (`22,80,8000-8100`)
- Export hasil ke CSV

## Struktur

```
portscanner/
├── main.go                       # CLI: parsing flag, print hasil, save CSV
├── go.mod
└── internal/
    └── scanner/
        ├── scan.go                 # logic scan inti (worker pool, dial, banner)
        ├── ports.go                # parser spec port ("1-1024,80,443")
        └── services.go             # tabel lookup nama service per port
```

## Cara menjalankan

```bash
go build -o portscanner.exe .
```

### Contoh pakai

```bash
# scan port umum (1-1024) di localhost
.\portscanner.exe -host localhost

# scan port tertentu aja
.\portscanner.exe -host 192.168.1.1 -ports "22,80,443"

# scan range custom + banner grabbing
.\portscanner.exe -host scanme.nmap.org -ports "1-1000" -banner

# tampilkan juga port yang closed (default cuma nampilin yang open)
.\portscanner.exe -host localhost -ports "20-30" -show-closed

# simpen hasil ke CSV
.\portscanner.exe -host localhost -output hasil.csv
```

### Semua flag

| Flag           | Default   | Keterangan                                          |
|-----------------|-----------|--------------------------------------------------------|
| `-host`         | (wajib)   | Target yang mau di-scan (IP atau hostname)              |
| `-ports`        | `1-1024`  | Spec port: single/range/list, bisa dikombinasi          |
| `-workers`      | `100`     | Jumlah probe paralel                                    |
| `-timeout-ms`   | `1500`    | Timeout koneksi per port (ms)                           |
| `-banner`       | `false`   | Coba ambil banner dari port yang open                   |
| `-show-closed`  | `false`   | Tampilkan juga port closed (default cuma yang open)     |
| `-output`       | (kosong)  | Path file CSV buat nyimpen hasil                        |

## Cara kerja singkat

1. `main.go` parse flag, parse spec port lewat `scanner.ParsePorts()`
2. `scanner.Scan()` bikin worker pool — tiap worker ambil 1 nomor port
   dari antrian, coba `net.DialTimeout("tcp", host:port, timeout)`
3. Kalau koneksi **berhasil** → port dianggap **open**, koneksi langsung
   ditutup lagi (kita cuma mau tau kebuka apa nggak, gak beneran ngobrol
   sama service-nya) — kecuali `-banner` aktif, baru coba baca sedikit
   data dari koneksi itu sebelum ditutup
4. Kalau koneksi **gagal** (connection refused / timeout) → port
   dianggap **closed/filtered**
5. Semua hasil dikumpulin, di-sort berdasarkan nomor port (karena
   worker yang paralel bikin urutan hasil datang gak berurutan), lalu
   di-print + disimpen ke CSV kalau diminta

## ⚠️ Penting: Etika & Legalitas Port Scanning

- **Cuma scan target yang lo punya izin buat di-scan** — server/network
  milik sendiri, lab CTF, atau situs yang memang disediakan buat testing
  (misal `scanme.nmap.org` yang emang dipasang publik buat latihan nmap)
- Scan port ke server/network orang lain **tanpa izin** bisa melanggar
  hukum di banyak negara (termasuk UU ITE di Indonesia), walaupun cuma
  "iseng lihat port kebuka apa nggak"
- Tool ini dibuat buat tujuan edukasi — belajar konsep networking &
  concurrency di Go — bukan buat reconnaissance tanpa izin

## Langkah selanjutnya (kalau mau lanjutin)

- Tambah deteksi OS/service fingerprinting yang lebih akurat (kirim
  probe spesifik per protokol, bukan cuma baca banner pasif)
- Tambah dukungan scan UDP (butuh pendekatan berbeda dari TCP connect)
- Tambah opsi output JSON selain CSV
- Progress bar real-time (jumlah port yang udah di-scan dari total)

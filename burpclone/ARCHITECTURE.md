# ARCHITECTURE.md — burpclone

Dokumen ini jelasin **kenapa** tiap modul dibikin dengan cara tertentu,
bukan cuma **apa** isinya (itu udah ada di README + komentar kode). Kalau
lo lupa alasan suatu keputusan desain 6 bulan dari sekarang, ini
tempatnya nyari jawaban.

## Alur request end-to-end

### Plain HTTP
```
Browser/curl -x  →  proxy.ListenAndServe (accept loop)
→  handleConn (baca 1 request via http.ReadRequest)
→  handlePlainHTTP
├─ strip hop-by-hop headers
├─ interceptAndApply (blok kalau intercept ON)
├─ transport.RoundTrip (forward ke server asli)
├─ resp.Write(conn) balik ke client
└─ p.logEntry (simpan ke SQLite + broadcast WS)
```
### HTTPS (MITM)
```
Browser/curl -x  →  handleConn deteksi method == CONNECT
→  handleConnect
├─ reply "200 Connection Established"
└─ mitmTLS
├─ tls.Server(conn, GetCertificate: ...)
│     └─ ca.GetOrCreateLeaf(SNI hostname)
├─ tlsConn.Handshake()
├─ loop: http.ReadRequest dari tlsConn
│     ├─ interceptAndApply
│     ├─ transport.RoundTrip (request ke real server, TLS terpisah)
│     ├─ resp.Write(tlsConn) balik ke browser
│     └─ p.logEntry
```
Poin penting: **ada dua koneksi TLS yang independen** — satu antara
browser↔proxy (pakai leaf cert palsu yang kita generate), satu lagi
antara proxy↔server asli (pakai `transport.RoundTrip` biasa, TLS normal
ke server sungguhan). Proxy jadi jembatan yang bisa baca isi keduanya
dalam bentuk cleartext karena dia yang pegang kunci di kedua sisi.

## Kenapa `Transport.RoundTrip`, bukan `http.Client`

`http.Client` secara default:
- Follow redirect otomatis (3xx) — proxy gak boleh gitu, browser yang
  harus liat redirect-nya dan yang mutusin mau ngikutin atau enggak
- Nyimpen cookie jar sendiri kalau dikonfigurasi — bisa nyampur antar
  request yang harusnya independen

`Transport.RoundTrip` cuma ngirim **satu** request, balikin **satu**
response, gak ada magic tambahan. Itu yang proxy butuhin — one-to-one
mapping antara request masuk dan request keluar.

## Kenapa CA pakai ECDSA (bukan RSA)

Sama kayak yang dipraktekin di PKI lab (prime256v1) — ECDSA P256 lebih
cepat digenerate dibanding RSA 2048/4096, dan buat leaf cert yang
di-generate on-the-fly setiap ketemu host baru, kecepatan generate itu
penting (biar gak ada delay kerasa pas pertama kali buka suatu situs).
Browser modern semua udah support ECDSA tanpa masalah.

## Kenapa leaf cert di-cache per host

Tanpa cache, tiap kali browser buka koneksi baru ke host yang sama
(misal tiap request AJAX ke domain yang sama), proxy bakal generate +
sign cert baru tiap kali — buang-buang CPU dan bikin request lebih
lambat. Cache-nya sengaja **in-memory doang** (gak dipersist ke disk),
soalnya leaf cert itu murah digenerate ulang dan gak ada alasan buat
disimpen lintas restart (beda sama root CA yang emang harus persist,
karena itu yang di-trust user).

## Kenapa Repeater "standalone" — gak lewat proxy/intercept/store

Ini keputusan desain yang paling gampang disalahpahami, jadi dicatet di
sini. Burp asli juga gitu — Repeater itu tool independen buat
re-test **satu** request berkali-kali dengan variasi payload, tanpa:
- Ketahan sama Intercept kalau lagi ON (bayangin lagi coba 20 variasi
  payload SQLi, tapi tiap kirim harus klik Forward manual)
- Nyampur ke History bareng trafik browser biasa (bikin history
  berantakan, susah bedain mana trafik asli mana trafik testing)

`repeater.Send` sengaja pakai `http.Client` terpisah
(`InsecureSkipVerify: true`, gak follow redirect) yang gak tersentuh
sama pipeline `proxy` sama sekali. Kalau nanti mau nambah "Repeater
history" (lihat README bagian fitur lanjutan), itu harus jadi tabel
SQLite **terpisah**, bukan numpang ke tabel `entries` yang sama.

## Kenapa body di-buffer penuh ke memory (`io.ReadAll`)

Supaya bisa **dicatat ke store** dan **di-forward** dari data yang sama
(soalnya `io.Reader` cuma bisa dibaca sekali). Trade-off: response gede
(download file besar dll) bakal makan RAM sebanding ukurannya. Ini
lumrah buat MVP, dan Burp asli juga punya limit ukuran serupa yang bisa
dikonfigurasi. Kalau mau diperbaiki nanti, bisa pakai `io.TeeReader` +
size limit, stop nyatet body kalau lebih dari N MB.

## Kenapa `handleConnect` reject dulu sebelum akhirnya di-fix di phase 2

(Catatan historis) Pas phase 1 selesai, `handleConnect` sengaja dibikin
balikin `501` yang rapi buat request HTTPS — bukan `panic()`. Ini
penting karena **panic di goroutine tanpa `recover` bakal nge-crash
seluruh proses**, bukan cuma request itu doang. Jadi walaupun HTTPS
belum diimplementasi, satu request HTTPS gak boleh sampai
mematikan seluruh proxy yang lagi nampung koneksi lain.

## Concurrency model

Satu goroutine per koneksi TCP yang diterima (`go p.handleConn(conn)`),
sama kayak pola worker-per-job di `portscanner`. Bedanya di sini "job"-
nya adalah koneksi yang bisa hidup lama (keep-alive, MITM loop baca
banyak request), bukan satu tugas pendek terus selesai.

`intercept.Queue.Hold` itu **memblokir goroutine per-koneksi**, bukan
seluruh proxy — karena tiap koneksi udah punya goroutine sendiri,
nahan satu request buat direview user gak ngeblok koneksi/request lain
yang lagi jalan bersamaan.

## Kenapa SQLite driver `modernc.org/sqlite` (bukan `mattn/go-sqlite3`)

`mattn/go-sqlite3` pakai cgo, artinya butuh C compiler ke-install pas
`go build` — nambah friksi setup terutama di Windows. `modernc.org/sqlite`
itu pure Go (SQLite di-transpile ke Go), jadi `go build` biasa langsung
jalan tanpa dependency toolchain tambahan. Trade-off performanya kecil
buat use-case tool kayak ini (bukan aplikasi yang butuh query database
jutaan kali per detik).
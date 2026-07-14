# Belajar Dasar Go (Golang) untuk Pemula

Panduan ini buat lo yang baru pertama kali nyoba Go. Fokusnya ke hal-hal
paling dasar: gimana caranya nulis, ngompile, dan jalanin kode Go.

---

## 1. Struktur Project Paling Sederhana

Go project minimal cuma butuh 2 hal:

```
hello/
├── go.mod      # identitas project (nama module + versi Go)
└── main.go     # kode program
```

### `go.mod`

File ini dibuat otomatis lewat command, isinya kayak gini:

```
module hello

go 1.22
```

Buat generate ini, jalanin di dalam folder project:

```bash
go mod init hello
```

`hello` di situ nama module lo (biasanya nama project atau path repo,
misal `github.com/username/hello`).

### `main.go`

```go
package main

import "fmt"

func main() {
	fmt.Println("Halo, dunia!")
}
```

Penjelasan tiap baris:
- `package main` — wajib ada di file yang mau dijadiin executable. Kalau
  bukan `main`, file itu jadi "library", bukan program yang bisa dijalanin
  langsung.
- `import "fmt"` — narik package standar `fmt` (format), isinya fungsi
  buat nge-print, format string, dll.
- `func main()` — ini titik masuk program. Wajib ada persis satu di
  package `main`. Go bakal mulai eksekusi dari sini.
- `fmt.Println(...)` — print teks ke terminal, otomatis ganti baris di
  akhir.

---

## 2. Cara Menjalankan Kode

Ada 2 cara utama:

### a) `go run` — langsung eksekusi tanpa nyimpen binary

```bash
go run main.go
```

Cocok buat coba-coba cepat pas development. Setiap kali jalanin, Go
compile ulang di belakang layar terus langsung run, tapi gak nyimpen file
executable-nya.

### b) `go build` — compile jadi file executable

```bash
go build -o hello .
```

- `-o hello` nentuin nama file hasil compile (`hello.exe` kalau di
  Windows)
- `.` artinya "compile package di folder ini"

Abis itu jalanin filenya:

```bash
./hello        # Linux/Mac
hello.exe      # Windows (atau .\hello.exe di PowerShell)
```

Bedanya sama `go run`: hasil `go build` adalah file yang bisa lo distribusi
ke orang lain / komputer lain (asal OS & arsitektur CPU sama), tanpa
mereka perlu install Go.

---

## 3. Variabel & Tipe Data Dasar

```go
package main

import "fmt"

func main() {
	// deklarasi eksplisit
	var nama string = "Berkah"
	var umur int = 20

	// short declaration (paling sering dipake)
	kota := "Batam"
	tinggiBadan := 170.5
	sudahLulus := false

	fmt.Println(nama, umur, kota, tinggiBadan, sudahLulus)
}
```

Tipe dasar yang paling sering dipake:

| Tipe      | Contoh          | Keterangan                     |
|-----------|-----------------|---------------------------------|
| `string`  | `"halo"`        | teks                            |
| `int`     | `42`            | bilangan bulat                  |
| `float64` | `3.14`          | bilangan desimal                |
| `bool`    | `true`/`false`  | benar/salah                     |

`:=` cuma bisa dipake di dalam fungsi, dan Go otomatis nebak tipenya dari
nilai yang dikasih (ini disebut *type inference*).

---

## 4. Fungsi

```go
func tambah(a int, b int) int {
	return a + b
}

func main() {
	hasil := tambah(3, 5)
	fmt.Println(hasil) // 8
}
```

- Parameter: `(a int, b int)` — nama lalu tipe, bisa disingkat jadi
  `(a, b int)` kalau tipenya sama.
- Return type ditulis setelah kurung parameter: `int` di atas artinya
  fungsi ini mengembalikan sebuah `int`.
- Fungsi bisa return lebih dari satu nilai — ini pola yang sangat umum di
  Go, biasanya dipake buat error handling:

```go
func bagi(a, b int) (int, error) {
	if b == 0 {
		return 0, fmt.Errorf("tidak bisa membagi dengan nol")
	}
	return a / b, nil
}

func main() {
	hasil, err := bagi(10, 2)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Hasil:", hasil)
}
```

Pola `if err != nil { ... }` ini bakal sering banget lo lihat di kode Go —
ini cara Go handle error, gak pake try/catch kayak bahasa lain.

---

## 5. Percabangan & Perulangan

### If/else

```go
umur := 17
if umur >= 18 {
	fmt.Println("Dewasa")
} else {
	fmt.Println("Belum dewasa")
}
```

### For (satu-satunya bentuk loop di Go, gak ada `while`)

```go
// mirip loop biasa
for i := 0; i < 5; i++ {
	fmt.Println(i)
}

// mirip while
n := 0
for n < 3 {
	fmt.Println("n =", n)
	n++
}

// infinite loop
for {
	fmt.Println("jalan terus sampai di-break")
	break
}
```

---

## 6. Slice (mirip array/list)

```go
buah := []string{"apel", "jeruk", "mangga"}

for i, b := range buah {
	fmt.Println(i, b)
}

buah = append(buah, "pisang") // tambah elemen baru
fmt.Println(len(buah))         // jumlah elemen
```

`[]string{...}` itu slice — kayak array tapi ukurannya bisa berubah
(dinamis). Ini yang paling sering dipake, bukan array biasa.

---

## 7. Struct (buat bikin tipe data sendiri)

```go
type Mahasiswa struct {
	Nama  string
	NIM   string
	IPK   float64
}

func main() {
	m := Mahasiswa{
		Nama: "Berkah",
		NIM:  "12345",
		IPK:  3.8,
	}
	fmt.Println(m.Nama, m.IPK)
}
```

Struct itu kayak `class` sederhana di bahasa lain (tanpa inheritance).
Field diakses pake titik: `m.Nama`.

---

## 8. Package & Import

Satu project Go biasanya dipecah jadi beberapa **package** (folder =
package). Contoh struktur:

```
myapp/
├── go.mod
├── main.go
└── internal/
    └── math/
        └── math.go
```

`internal/math/math.go`:
```go
package math

func Tambah(a, b int) int {
	return a + b
}
```

`main.go`:
```go
package main

import (
	"fmt"
	"myapp/internal/math" // path-nya: <nama module>/<folder>
)

func main() {
	fmt.Println(math.Tambah(2, 3))
}
```

Aturan penting: cuma fungsi/variabel yang **huruf awalnya kapital**
(`Tambah`, bukan `tambah`) yang bisa diakses dari package lain — ini
disebut *exported*.

---

## 9. Dependency Eksternal (library orang lain)

Kalau butuh library luar (misal Cobra, Gin, dll):

```bash
go get github.com/nama/package
```

Ini bakal:
1. Download package-nya
2. Nambahin ke `go.mod`
3. Bikin/update `go.sum` (checksum buat keamanan)

Lalu tinggal `import` seperti biasa di kode.

> Catatan: ini butuh koneksi internet ke Go module proxy
> (`proxy.golang.org`) atau langsung ke repo-nya (GitHub, dll). Kalau
> jaringan lo dibatasi/offline, `go get` bakal gagal — kayak yang kejadian
> pas gue coba bikinin project pake Cobra kemarin.

---

## 10. Command Go yang Paling Sering Dipake

| Command                  | Fungsi                                          |
|---------------------------|--------------------------------------------------|
| `go mod init <nama>`      | Bikin project baru + `go.mod`                    |
| `go run <file>.go`        | Compile + langsung jalanin, gak nyimpen binary   |
| `go build -o nama .`      | Compile jadi file executable                     |
| `go get <package>`        | Install dependency eksternal                     |
| `go mod tidy`             | Bersihin/sinkronin dependency di `go.mod`         |
| `go fmt ./...`            | Rapiin format kode otomatis                       |
| `go test ./...`           | Jalanin unit test                                 |
| `go version`              | Cek versi Go yang ke-install                     |

---

## 11. Alur Kerja Tipikal

1. `go mod init <nama-project>` — sekali di awal
2. Tulis kode di `main.go` (atau file/package lain)
3. `go run main.go` buat coba cepat, atau `go build -o app .` buat bikin
   executable
4. Kalau butuh library luar → `go get`, terus `import`
5. Ulangi edit-compile-run sampai fix

---

## 12. Latihan Kecil

Coba bikin file `latihan.go`, isi program yang:
1. Minta input nama dari user (`fmt.Scanln`)
2. Print "Halo, <nama>!"

Hint:
```go
package main

import "fmt"

func main() {
	var nama string
	fmt.Print("Masukkan nama: ")
	fmt.Scanln(&nama)
	fmt.Printf("Halo, %s!\n", nama)
}
```

Jalanin dengan `go run latihan.go`, coba ketik nama lo, lihat hasilnya.

---

## Referensi Lanjutan

- Dokumentasi resmi: https://go.dev/doc/
- Tour interaktif langsung di browser: https://go.dev/tour/
- Effective Go (best practices): https://go.dev/doc/effective_go

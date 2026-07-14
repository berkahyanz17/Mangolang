# todoapi

REST API sederhana untuk mengelola daftar to-do, dibangun **hanya pakai
Go standard library** (`net/http`) — tanpa framework eksternal (Gin,
Echo, dll), jadi bisa langsung `go build` tanpa perlu download apa-apa
dari internet.

Data disimpan **in-memory** (hilang kalau server di-restart). Ini bagus
buat belajar dulu; nanti gampang di-swap ke database beneran (lihat
bagian "Langkah selanjutnya" di bawah).

## Struktur

```
todoapi/
├── main.go                     # entrypoint, setup server + routing
├── go.mod
└── internal/
    └── todo/
        ├── model.go             # struct Todo
        ├── store.go             # in-memory storage (thread-safe)
        └── handlers.go          # HTTP handler tiap endpoint
```

## Cara menjalankan

```bash
go build -o todoapi .
./todoapi          # Linux/Mac
todoapi.exe        # Windows
```

Atau langsung tanpa build:
```bash
go run main.go
```

Server jalan di `http://localhost:8080`.

## Endpoint

| Method | Path          | Body                              | Keterangan            |
|--------|---------------|------------------------------------|------------------------|
| GET    | `/health`     | -                                   | Cek server hidup       |
| GET    | `/todos`      | -                                   | List semua todo        |
| POST   | `/todos`      | `{"title": "..."}`                  | Bikin todo baru         |
| GET    | `/todos/{id}` | -                                   | Ambil satu todo        |
| PUT    | `/todos/{id}` | `{"title": "...", "done": true}`   | Update todo             |
| DELETE | `/todos/{id}` | -                                   | Hapus todo              |

## Contoh testing pakai `curl`

```bash
# cek server hidup
curl http://localhost:8080/health

# bikin todo baru
curl -X POST http://localhost:8080/todos -d '{"title":"Belajar Go"}'

# lihat semua todo
curl http://localhost:8080/todos

# lihat todo id 1
curl http://localhost:8080/todos/1

# update todo id 1 jadi selesai
curl -X PUT http://localhost:8080/todos/1 -d '{"title":"Belajar Go","done":true}'

# hapus todo id 1
curl -X DELETE http://localhost:8080/todos/1
```

Kalau gak punya `curl` di Windows, bisa pakai
[Postman](https://www.postman.com/downloads/) atau extension **REST
Client** / **Thunder Client** di VS Code — lebih enak buat pemula karena
ada UI-nya.

## Cara kerja singkat

1. `main.go` bikin `Store` (penyimpanan in-memory) dan `Handler` (logic
   HTTP), lalu daftarin semua route ke `http.ServeMux`.
2. Setiap request masuk → dicocokin ke route (`GET /todos`, dll) → masuk
   ke fungsi handler yang sesuai di `internal/todo/handlers.go`.
3. Handler manggil method di `Store` (`Create`, `Get`, `Update`,
   `Delete`) buat baca/ubah data, lalu balikin response JSON.
4. `Store` pakai `sync.Mutex` biar aman kalau ada banyak request
   bersamaan (concurrent-safe).

Ini pola umum di Go: pisahin **handler** (urusan HTTP: parsing request,
nulis response) dari **store/logic** (urusan data). Enak buat testing dan
gampang di-scale.

## Langkah selanjutnya (kalau mau lanjutin)

- Ganti `Store` in-memory jadi SQLite/PostgreSQL biar data gak hilang
  pas restart
- Tambah validasi input lebih ketat
- Tambah middleware logging (log tiap request masuk)
- Tambah auth (misal JWT) kalau butuh proteksi endpoint
- Kalau nanti punya akses network yang lebih luas, bisa upgrade router-nya
  ke `chi` atau `gin` buat fitur routing lebih canggih (tapi untuk skala
  kecil-menengah, `net/http` bawaan udah lebih dari cukup)

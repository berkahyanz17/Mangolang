# Planning Notes — mycli & todoapi Enhancements

Tanggal: 18 Juli 2026

## mycli → Markov Chain Text Generator

**Konsep:** pecah training text jadi n-gram (order-N window), simpan mapping
`"kata_a kata_b" → [daftar kata yang pernah nyusul]`, generate teks baru
dengan random-pick berbobot dari mapping tiap step. Ini versi statistik
sederhana dari konsep "next-token prediction" ala LLM neneral net.

### Struktur yang diusulkan
```
mycli/
├── internal/
│   └── markov/
│       ├── chain.go        # struct Chain{order int, table map[string][]string}
│       ├── train.go        # Train(text string) — tokenize + build table
│       ├── generate.go     # Generate(seed string, n int) string
│       └── persist.go      # Save/Load table ke JSON
└── cmd/
    └── markov.go           # subcommand: `mycli learn` & `mycli generate`
```

### Subcommand yang diusulkan
| Command | Fungsi |
|---|---|
| `mycli learn <file.txt> --order 2 --model model.json` | Baca corpus, build chain, simpan ke JSON |
| `mycli generate --seed "kata awal" --words 50 --model model.json` | Load model, generate teks N kata |
| `mycli markov-stats --model model.json` | (opsional) tampilin ukuran vocab, jumlah state |

### Keputusan yang perlu diambil
1. Order default: 2 (bigram, simpel) vs 3 (trigram, lebih koheren tapi butuh corpus lebih besar)
2. Tokenisasi: per kata (`strings.Fields`) vs per kata + punctuation sebagai token terpisah
3. Sumber corpus buat testing: teks bebas, atau kumpulan writeup Medium sendiri (biar model "belajar gaya nulis sendiri")
4. Random source: `math/rand` stdlib cukup, gak perlu library eksternal

**Estimasi:** selesai dalam satu sesi coding, gak perlu dipecah fase.

---

## todoapi → App Template Beneran (bukan vulnerable target)

Dipecah fase, mengikuti pattern build-order yang sudah dipakai di
reconscan/haxprox.

| # | Fase | Kerjaan |
|---|---|---|
| 1 | Persistence | Ganti in-memory map → SQLite (`modernc.org/sqlite`, reuse pattern dari haxprox) |
| 2 | Data model upgrade | Tambah field: `due_date`, `priority`, `tags/category`, `user_id` |
| 3 | Auth | JWT-based login/register (reuse pattern dari HealthSync), todo terikat ke user |
| 4 | Query & filter | `?done=false&sort=due_date&priority=high`, pagination `?limit&offset` |
| 5 | Validation & error handling | Structured error response, proper HTTP status codes |
| 6 | Frontend (opsional) | Static HTML/JS simple, atau React kecil ala mini-HealthSync |
| 7 | Test | Table-driven test buat `store` dan `handlers` |

### Keputusan yang perlu diambil
1. Tetap API-only, atau langsung include frontend juga (full-stack)?
2. Auth: reuse pola HealthSync (JWT + bcrypt) atau coba pendekatan baru?
3. Target akhir: portfolio standalone, atau latihan pattern yang dipakai ulang di project lain?

---

## Catatan lain
- `todoapi` **bukan** diarahkan jadi vulnerable target (opsi itu ditolak) — fokus ke app template yang bisa berkembang jadi produk beneran.
- `haxprox` (dulu `burpclone`) sudah selesai di-rename, commit `a11d686` di local clone, tinggal di-push.

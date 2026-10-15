# Backend FaceLink AI (Go)

Fondasi backend untuk demo **satu kelas, satu terminal** (FL-005). Pada tahap ini
belum ada pengenalan wajah: yang dibangun adalah kerangka layanan, basis data,
migrasi berversi, serta titik kesehatan yang dapat diuji.

## Status implementasi

| Bagian | Status |
| --- | --- |
| Konfigurasi berbasis lingkungan + validasi ketat | Ada |
| Log terstruktur JSON dengan request ID | Ada |
| SQLite (WAL, foreign key, busy timeout) | Ada |
| Runner migrasi berversi + checksum | Ada |
| Router chi, enveloppe galat konsisten, middleware | Ada |
| `/api/v1/healthz` (liveness) dan `/api/v1/readyz` (readiness) | Ada |
| Penghentian rapi (graceful shutdown) | Ada |
| Tabel `face_templates`, enkripsi biometrik, KeyProvider | **Belum** — di luar FL-005 (D-030) |

Migrasi `00001_init_core_schema.sql` membuat 13 tabel domain **non-biometrik**.
Tabel `face_templates` dan `face_enrollment_samples` sengaja **tidak** dibuat.

## Prasyarat

- **Go 1.27 atau lebih baru** (dikembangkan dan diuji pada Go 1.27.0).
- Tidak butuh CGO. Driver SQLite yang dipakai murni Go (`modernc.org/sqlite`),
  sehingga `gcc` **tidak** diperlukan untuk sekadar membangun dan menguji.
  (Uji `-race` tetap butuh `CGO_ENABLED=1` dan `gcc`; lihat bagian Pengujian.)

Tidak ada langganan, API berbayar, atau layanan cloud yang diperlukan.

## Menjalankan

```powershell
cd backend

# Terapkan migrasi ke basis data.
go run ./cmd/server -migrate

# Periksa konfigurasi tanpa menjalankan layanan.
go run ./cmd/server -check-config

# Jalankan layanan.
go run ./cmd/server
```

Nilai bawaan yang aman: mendengarkan di `127.0.0.1:8080` dengan basis data
`data/facelink.db`. Konfigurasi **menolak** alamat non-loopback dan menolak jalur
basis data di dalam folder sinkronisasi cloud (OneDrive, Dropbox, Google Drive,
iCloud) karena sinkronisasi dapat merusak berkas WAL.

Semua variabel lingkungan didokumentasikan di `.env.example` pada akar repositori.
Backend tidak memuat `.env` otomatis; ekspor variabelnya lebih dulu.

### Memeriksa dari terminal lain

```powershell
curl.exe http://127.0.0.1:8080/api/v1/healthz
curl.exe http://127.0.0.1:8080/api/v1/readyz
```

`/readyz` melaporkan `ai_engine: "unavailable"` bila `FACELINK_AI_ENGINE_URL`
kosong. Itu **bukan** kegagalan: absensi manual harus tetap berjalan, jadi mesin
AI yang belum siap tidak menggagalkan kesiapan backend.

## Tata letak

```
backend/
├── cmd/server/          # titik masuk: flag -check-config dan -migrate
├── internal/config/     # pembacaan lingkungan, validasi, Redacted()
├── internal/database/   # pembukaan SQLite, WAL, foreign key, checkpoint
├── internal/httpapi/    # router, endpoint kesehatan, enveloppe galat
├── internal/logging/    # log JSON, larangan mencetak data biometrik
├── internal/migrations/ # runner migrasi berversi + checksum SHA-256
└── migrations/          # berkas SQL, pola 00001_nama.sql
```

## Pengujian

```powershell
cd backend

gofmt -l .            # harus tidak menghasilkan keluaran
go vet ./...
go build ./...
go test ./... -count=1
```

Uji `-race` memerlukan `CGO_ENABLED=1` **dan** `gcc` di `PATH`. Pada mesin tanpa
keduanya, perintah itu gagal pada tahap kompilasi `runtime/cgo`; itu keterbatasan
lingkungan, bukan kegagalan kode.

## Keamanan dan privasi

- Layanan hanya mendengarkan di loopback. Mesin AI lokal tidak boleh terbuka
  bebas ke jaringan (AGENTS.md §2).
- Log tidak boleh memuat JPEG mentah, embedding, kunci, token, atau detail
  biometrik. Pengujian kebocoran di `internal/httpapi/leak_test.go` menembak
  server HTTP nyata dan memeriksa badan respons, tajuk, serta log akses.
- Data biometrik tidak boleh masuk riwayat Git. `.gitignore` mengabaikan SQLite
  beserta sidecar `-wal`/`-shm`, karena sidecar dapat memuat sisa baris yang
  sudah dihapus.
- Absensi otomatis dari pengenalan wajah belum ada pada tahap ini.

Dokumen rujukan: `docs/API_CONTRACT.md` (kontrak endpoint),
`docs/DATA_MODEL.md` (skema dan transaksi), `docs/SECURITY_PRIVACY.md`
(aturan data), `docs/DECISIONS.md` (keputusan, termasuk D-030).

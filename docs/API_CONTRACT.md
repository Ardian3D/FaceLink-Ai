# Kontrak API FaceLink AI

**Versi:** Draf 0.2 · **Status:** BELUM DIIMPLEMENTASIKAN, BELUM MENJADI OPENAPI TERVERIFIKASI.

Kontrak ini menjadi acuan yang harus ditinjau **sebelum** membuat handler Go/Python atau layar React Native yang memanggilnya. Nama endpoint dan bentuk respons masih dapat direvisi dengan persetujuan.

## 1. Aturan umum

- Jalur dasar: `/api/v1`.
- Data biasa: JSON UTF-8; unggahan kamera: `image/jpeg` atau multipart sesuai kontrak yang nanti disahkan.
- Identitas record: UUID. Waktu: ISO 8601 UTC (`2026-10-15T08:00:00Z`); aplikasi menampilkan zona lokal.
- Semua endpoint selain login dan pemeriksaan kesehatan minimal memerlukan autentikasi sesuai peran.
- Token operator dan token perangkat **terpisah**; perangkat hanya boleh mengirim untuk dirinya/sesi yang sah.
- Batas ukuran JPEG, validasi format, pembatasan laju, dan batas waktu harus diterapkan.
- Respons tidak boleh membocorkan embedding, gambar mentah, hash password, token rahasia, atau tebakan identitas yang meragukan.
- Jangan mencatat payload biometrik di log; sertakan `request_id` yang tidak mengandung data pribadi.

## 2. Format kesalahan

```json
{
  "error": {
    "code": "SESSION_CLOSED",
    "message": "Sesi absensi sudah ditutup.",
    "request_id": "xxxxxxxx-xxxx-4xxx-8xxx-xxxxxxxxxxxx"
  }
}
```

Kode HTTP yang direncanakan: `400` format buruk, `401` belum autentikasi, `403` tidak berwenang, `404` tidak ditemukan, `409` konflik status/duplikasi operasi, `413` JPEG terlalu besar, `422` tidak valid menurut aturan domain, `429` terlalu sering, `503` mesin AI tidak tersedia.

## 3. Endpoint operator (usulan)

Semua jalur di bawah diawali `/api/v1`.

| Metode | Jalur | Tujuan |
|---|---|---|
| `POST` | `/auth/login` | Masuk operator (dengan pembatasan percobaan) |
| `GET` | `/me` | Identitas operator yang sedang login |
| `GET`, `POST` | `/people` | Daftar/tambah peserta demo |
| `GET`, `PATCH`, `DELETE` | `/people/{person_id}` | Lihat/perbarui/hapus sesuai aturan retensi |
| `POST` | `/people/{person_id}/consent` | Catat persetujuan biometrik versi tertentu |
| `POST` | `/people/{person_id}/consent/withdraw` | Cabut izin dan proses penghapusan template |
| `POST` | `/people/{person_id}/face-enrollments` | Mulai enrollment terotorisasi |
| `DELETE` | `/people/{person_id}/face-enrollments` | Hapus template wajah yang tersimpan |
| `GET`, `POST` | `/classes` | Daftar/buat kelas demo |
| `PUT` | `/classes/{class_id}/members/{person_id}` | Hubungkan peserta dengan kelas |
| `GET`, `POST` | `/sessions` | Daftar/buat sesi absensi |
| `POST` | `/sessions/{session_id}/open` | Buka sesi absensi |
| `POST` | `/sessions/{session_id}/close` | Tutup sesi absensi |
| `GET` | `/sessions/{session_id}/attendance` | Roster beserta catatan dan status turunan |
| `POST` | `/sessions/{session_id}/manual-checkins` | Absen/koreksi manual dengan alasan |
| `GET` | `/sessions/{session_id}/export.csv` | Unduh rekap CSV tanpa biometrik |
| `GET` | `/sessions/{session_id}/events` | Pembaruan status (stream atau alternatif polling) |
| `GET` | `/healthz` | Kesehatan proses minimal, tanpa informasi rahasia |

## 4. Endpoint terminal kamera (usulan)

`POST /api/v1/devices/{device_id}/recognition-attempts`

**Prasyarat:** kredensial perangkat sah, terdaftar aktif, terkait sesi yang **terbuka**, dan input JPEG valid. Batas awal seperti 1 MB per gambar hanya kandidat yang perlu diuji bersama pilihan kamera.

**Kategori keluaran:**

| `outcome` | Makna | Dampak absensi |
|---|---|---|
| `recognized` | Pencocokan dan seluruh validasi berhasil | Satu catatan hadir dibuat |
| `already_checked_in` | Peserta telah dicatat di sesi yang sama | Catatan pertama dipertahankan |
| `retry` | Blur, tanpa wajah, lebih dari satu wajah, atau kualitas tidak cukup | Tidak membuat catatan |
| `unknown` | Tidak ditemukan kecocokan yang memenuhi syarat | Tidak membuat catatan |
| `manual_review` | Hasil ambigu atau kebijakan meminta pemeriksaan | Tidak membuat catatan |
| `error` | Gangguan sistem atau koneksi | Tidak membuat catatan |

Contoh respons **ilustrasi saja**, bukan endpoint yang sudah berjalan:

```json
{
  "outcome": "recognized",
  "attendance": {
    "session_id": "11111111-1111-4111-8111-111111111111",
    "person_id": "22222222-2222-4222-8222-222222222222",
    "status": "present",
    "source": "face",
    "checked_in_at": "2026-10-15T08:00:00Z"
  }
}
```

Jangan menampilkan daftar kandidat atau skor internal pada layar terminal yang dapat dilihat umum. Dashboard operator yang sudah masuk boleh menampilkan nama peserta setelah validasi check-in.

## 5. Alur enrollment

1. Operator memilih peserta dengan persetujuan aktif.
2. Backend membuka percobaan enrollment berjangka waktu dengan ID khusus.
3. Adapter webcam menangkap 3–10 sampel untuk dievaluasi kualitasnya.
4. Pekerja AI menerima gambar sementara, memvalidasi satu wajah, dan menghasilkan template berlabel versi model.
5. Template disimpan secara terlindungi; sampel yang ditolak diberi alasan untuk perbaikan.
6. Tidak boleh menyediakan endpoint unggah tak terbatas yang memungkinkan orang mendaftarkan wajah ke identitas orang lain.

## 6. Kontrak internal backend dan mesin AI (kandidat)

`POST /internal/v1/face/enroll` dan `/internal/v1/face/match` hanya boleh dapat diakses melalui `127.0.0.1` atau mekanisme IPC, dan harus mempunyai batas waktu serta autentikasi jika terpisah proses.

Masukan: JPEG sementara, `correlation_id`, versi skema, dan konteks pemrosesan yang diperlukan. Keluaran: kode hasil, informasi kualitas, versi model, kandidat teratas dan kandidat kedua beserta skor **hanya untuk backend**, serta alasan penolakan. Backend menerapkan kebijakan persetujuan, sesi, kelas, margin kecocokan, dan pencatatan.

## 7. Idempotensi dan transaksi

- Batasi unik `(session_id, person_id)` di database.
- Untuk percobaan dari terminal, tambahkan `request_id`/kunci idempotensi agar retry jaringan tidak menghasilkan efek samping berulang.
- Penutupan sesi dan perubahan consent wajib diperiksa pada transaksi penulisan kehadiran, bukan hanya pada awal request.
- Jangan mengubah waktu check-in pertama jika ada request kedua.

## 8. Pengujian kontrak wajib

Operator/perangkat tidak berizin; izin biometrik dicabut; sesi ditutup; kiriman simultan; JPEG terpotong; gambar terlalu besar; wajah asing; lebih dari satu wajah; AI timeout; database gagal; koneksi mobile putus lalu kembali; export tanpa embedding; serta penarikan consent yang membatalkan akses matching.

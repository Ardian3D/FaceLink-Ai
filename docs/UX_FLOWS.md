# Alur Pengguna dan UI/UX React Native

**Status:** Rancangan v0.2, belum berupa tampilan atau aplikasi jadi.  
**Platform utama:** Android melalui React Native + Expo + TypeScript. Dashboard berfokus pada operator pada MVP.

## 1. Tujuan pengalaman pengguna

Operator dapat menjalankan sesi absensi satu kelas tanpa perlu memahami AI atau pengaturan perangkat yang rumit. Peserta memahami kapan mereka boleh menghadap kamera, bagaimana status absensi diketahui, dan apa yang harus dilakukan jika pengenalan gagal.

## 2. Struktur navigasi operator

| Layar | Isi utama | Tindakan utama |
|---|---|---|
| Masuk | Form autentikasi dan kesalahan yang aman | Login |
| Beranda | Kelas aktif, status server/kamera, ringkasan sesi | Pilih kelas atau buka sesi |
| Kelas & Peserta | Daftar peserta dengan kode demo dan status persetujuan | Tambah peserta, enrollment berizin |
| Enrollment | Penjelasan consent, kualitas kamera, progres sampel | Mulai/batalkan enrollment, hapus template |
| Sesi Absensi | Mulai/tutup sesi, waktu, daftar hadir, peserta belum tercatat | Jalankan sesi, lihat hasil |
| Pemeriksaan Manual | Peserta, alasan, sumber data, tindakan koreksi | Check-in manual atau koreksi |
| Rekap & Laporan | Jumlah hadir/terlambat/belum tercatat dan riwayat | Ekspor CSV |
| Perangkat & Pengaturan | Koneksi backend, status webcam/ESP32, aturan tampilan | Periksa koneksi dan konfigurasi |

**Akun mahasiswa tidak wajib pada MVP.** Bila nanti ditambah, peserta hanya bisa melihat statusnya sendiri dan mengelola persetujuannya, tidak seluruh daftar peserta.

**Peran yang berlaku pada MVP hanya `operator`, `admin`, dan `device`.** Tidak ada peran `dosen` maupun `mahasiswa` di sistem: "dosen" adalah `operator`, dan "mahasiswa" belum memiliki akun. Matriks izin lengkap beserta perbedaan `operator` dan `admin` ada di `docs/AUTHORIZATION.md`. Peran keempat (`participant`) hanya dipertimbangkan pasca-MVP dan menuntut revisi dokumen otorisasi itu.

## 3. Keadaan UI yang wajib berbeda

- **Memuat:** tampilkan indikator tanpa membuat asumsi hasil.
- **Siap:** koneksi lokal dan sesi valid.
- **Sedang memproses:** kamera/AI berjalan; cegah pengiriman berulang tanpa kontrol.
- **Berhasil:** hasil sah dan diterima backend, bukan semata-mata kemunculan wajah di kamera.
- **Sudah absen:** informasi bahwa peserta telah tercatat, tanpa membuat row baru.
- **Perlu ulang:** contoh wajah blur, pantulan kacamata, posisi wajah tidak tepat.
- **Butuh operator:** hasil ambigu, peserta tidak mendaftar, persetujuan belum aktif.
- **Offline/gangguan:** koneksi server atau mesin AI bermasalah; tampilkan pesan yang dapat ditindaklanjuti.

Jangan memakai warna saja untuk menyampaikan status: sertakan ikon dan teks jelas.

## 4. Alur demo kelas pertama

1. Operator login dan melihat kesehatan server lokal.
2. Operator memilih kelas, meninjau 5–10 peserta demo yang telah memberi persetujuan.
3. Jika ada peserta belum enrollment, operator membuka layar enrollment berizin.
4. Operator membuka sesi dengan aturan waktu keterlambatan.
5. Peserta berdiri bergantian di depan webcam, status muncul pada terminal serta halaman sesi.
6. Peserta mencoba absen kedua kali; aplikasi menampilkan **sudah absen**.
7. Operator mencoba contoh kasus gagal dan menggunakan absensi manual bila perlu.
8. Operator menutup sesi dan mengunduh CSV.

## 5. Skenario peserta berkacamata

- Sediakan petunjuk posisi wajah dan pencahayaan yang mengurangi pantulan.
- Beri pilihan mencoba ulang jika confidence tidak mencukupi, **tanpa menampilkan skor teknis atau tebakan identitas ke layar umum**.
- Rekam hasil pengujian berkacamata secara agregat; jangan menyembunyikan kasus gagal.
- Jika tetap gagal, operator dapat mencatat kehadiran secara manual setelah pemeriksaan yang sesuai.

## 6. Pencabutan persetujuan

Operator membuka profil peserta → memeriksa permintaan pencabutan → sistem memperlihatkan konsekuensi (pengenalan wajah berhenti, manual tetap bisa) → memproses penghapusan template dan cache → memberi konfirmasi berdasarkan hasil server. Jangan menyatakan sudah terhapus sebelum server mengonfirmasi.

## 7. Pedoman visual

- Tampilan bersih, mudah dibaca saat demo kelas, dan tidak membutuhkan animasi berat.
- Kontras memadai; tombol dengan label jelas; ukuran target sentuh ramah mobile.
- Tampilkan status koneksi/kamera dan waktu sesi yang konsisten dalam zona Indonesia.
- Hindari menampilkan seluruh nama peserta di layar kamera publik; informasi lengkap hanya pada dashboard operator yang sudah masuk.
- Tidak perlu fitur ratusan kelas atau dashboard enterprise sebelum MVP stabil.

## 8. Kriteria penerimaan UX

Pengguna dapat menyelesaikan alur login → kelas → enrollment berizin → mulai sesi → hasil → manual → tutup sesi → ekspor, mengetahui gangguan tanpa tertipu hasil palsu, dan memahami perbedaan gagal pengenalan dan tidak hadir. Navigasi dan tampilan diuji pada HP Android sungguhan sebelum presentasi.

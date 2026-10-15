# Roadmap Pengembangan FaceLink AI

Roadmap adalah **urutan pekerjaan beserta gerbang penerimaan**, bukan janji tanggal selesai. Satu tugas/branch yang dapat ditinjau dalam satu waktu. Jangan lanjut fase bila syarat fase saat ini belum teruji.

| Fase | ID tugas | Hasil yang ditargetkan | Syarat dapat melanjutkan |
|---|---|---|---|
| 0 — Fondasi | FL-000–009 | PRD, keputusan, aturan agent, aturan Git, kontrak awal | Pemilik menyetujui kebutuhan; repo bersih dan bebas data rahasia |
| 1 — Backend | FL-010–019 | Backend modular, auth, database/migrasi, orang, kelas, sesi, absensi manual, tes | Otorisasi lulus, transaksi mencegah duplikasi, check-in manual bekerja |
| 2 — Mesin AI lokal | FL-020–029 | Pemuatan model, kualitas gambar, enrollment, matching, penarikan consent, evaluasi | Wajah tidak diketahui/ambigu tidak pernah otomatis dicatat; sampel izin aktif dapat diuji |
| 3 — React Native | FL-030–039 | Navigasi, login operator, sesi, roster, enrollment dan laporan | Android dapat membuka layar dan menampilkan data mock/API secara benar |
| 4 — Integrasi software | FL-040–049 | Webcam → backend → AI → database → mobile, pembaruan status, uji gagal | Alur dari persetujuan hingga rekap berjalan tanpa hardware ESP32 |
| 5 — Hardware IoT | FL-050–059 | Pilih board tepat, beli komponen sesuai anggaran, firmware, LED/buzzer | Alur yang sama bekerja lewat ESP32-S3 CAM, pemutusan koneksi tertangani |
| 6 — Validasi dan rilis demo | FL-060–069 | Pengukuran akurasi/latensi, uji berkacamata, keamanan, APK, video presentasi | APK terpasang dan demo berulang berhasil dengan laporan keterbatasan |
| 7 — Cloud opsional | FL-070+ | Tinjauan biaya dan ancaman, rancangan migrasi, eksperimen deployment | **Hanya** jika diminta setelah kelulusan seluruh uji lokal |

## Tugas pertama yang disarankan

**FL-001 — Fondasi Git dan pemeriksaan dokumentasi:** periksa repo sebenarnya, tinjau README dan aturan agent, putuskan cara kolaborasi, siapkan `.gitignore` untuk Go/Python/React Native **sebelum source atau data dibuat**, lalu jalankan pemeriksaan agar tidak ada data pribadi/rahasia. **Belum membuat aplikasi pada FL-001**.

## Cara serah-terima tiap tugas

Catat: tujuan → kriteria penerimaan → ruang lingkup berkas → perintah pengujian → hasil aktual → perubahan keputusan/dokumen → tugas berikutnya. Status selesai harus didasarkan pada hasil yang dapat diulang, bukan deskripsi agent.

# Rencana Pengujian dan Gerbang Kualitas FaceLink AI

**Status:** Rencana pengujian, bukan laporan hasil. Angka akurasi/latensi hanya boleh dimasukkan setelah uji yang terukur.

## 1. Tingkatan pengujian

| Tingkat | Tujuan | Bukti |
|---|---|---|
| Unit backend | Aturan domain sesi, izin, duplikasi, keterlambatan dan auth | Perintah tes dan hasil |
| Integrasi database | Constraint unik, migrasi, transaksi, rollback | Tes otomatis pada database uji |
| Mesin AI | Pemeriksaan JPEG, kualitas, deteksi satu wajah, hasil matching | Matriks skenario + log nonpribadi |
| Kontrak API | Status HTTP, bentuk JSON, hak akses dan batas input | Pengujian kontrak |
| Mobile | Navigasi, input, penyajian data, offline/retry | Typecheck dan uji perangkat/emulator |
| End-to-end webcam | Enrollment → absensi → rekap tanpa ESP32 | Rekaman langkah demo dan catatan tes |
| IoT | Koneksi Wi-Fi, capture, LED/buzzer, timeout | Hasil pengujian perangkat fisik |
| Privasi/keamanan | Penghapusan template, akses tidak sah, kebocoran data | Checklist dan tes negatif |

## 2. Skenario penerimaan minimum

| ID | Skenario | Harapan |
|---|---|---|
| T-01 | Operator salah kata sandi | Tidak dapat masuk; pesan tidak membocorkan informasi |
| T-02 | Operator tanpa izin mengakses sesi | `401/403`, tidak ada data dibocorkan |
| T-03 | Peserta belum consent mencoba enrollment | Ditolak |
| T-04 | Foto tanpa wajah | `retry`, tanpa check-in |
| T-05 | Foto dua wajah | `retry`, tanpa check-in |
| T-06 | Wajah asing | `unknown`, tanpa check-in |
| T-07 | Kecocokan ambigu | `manual_review`, tanpa check-in |
| T-08 | Peserta sah pada sesi terbuka | Satu catatan hadir/terlambat sesuai aturan |
| T-09 | Peserta yang sama mengulang | `already_checked_in`, waktu pertama tidak berubah |
| T-10 | Dua request bersamaan dari peserta yang sama | Tepat satu row di database |
| T-11 | Sesi sudah ditutup | Tidak ada check-in baru lewat perangkat |
| T-12 | Persetujuan dicabut | Tidak bisa matching; template hilang dari cache |
| T-13 | AI mati atau timeout | Tidak membuat kehadiran palsu; UI memberi petunjuk |
| T-14 | JPEG terpotong atau sangat besar | Ditolak tanpa proses berbahaya |
| T-15 | Operator memperbaiki absensi manual | Harus login, memberi alasan, ada audit |
| T-16 | Ekspor CSV | Data kehadiran benar dan bebas biometrik |
| T-17 | HP kehilangan koneksi | Status offline jelas; pulih tanpa check-in duplikat |
| T-18 | Peserta memakai kacamata bening | Hasil diuji dan dilaporkan, tanpa jaminan lulus |
| T-19 | Foto wajah pada layar HP/hasil cetak | Keterbatasan spoofing didokumentasikan |
| T-20 | APK dipasang ke HP Android | Instalasi dan alur utama berjalan pada jaringan lokal |
| T-21 | Kunci aktif tidak ada saat backend dinyalakan | Penyalaan **gagal dengan pesan jelas**; backend **tidak** membuat kunci baru sendiri dan **tidak** membuat template tironan |
| T-22 | Berkas kunci disalin ke mesin/akun lain | Dekripsi gagal; tidak ada template yang terbaca dan tidak ada yang ditebak. Hasil dicatat apa adanya, termasuk bila gagal pada tahap yang tidak terduga |
| T-23 | Berkas kunci dihapus, database utuh | Template dihapus, peristiwa tercatat di `audit_events` tanpa isi embedding, peserta dapat hadir lewat jalur manual |
| T-24 | `aad` pada baris diubah (uji perusakan) | Verifikasi AAD gagal; baris **tidak** dipakai dan backend mengembalikan “tidak dapat diverifikasi” |
| T-25 | `key_version` baris tidak sesuai kunci yang tersedia | Baris tidak dapat didekripsi; backend tidak menurunkan versi secara diam-diam dan tidak mencoba kunci lain di luar versi yang ditunjuk |
| T-26 | Cadangan database tanpa berkas kunci dipulihkan | Kehadiran dan nama tetap terbaca; kolom `ciphertext` **tidak** dapat dibaca. Ini harus dilaporkan sebagai keterbatasan, bukan kegagalan tak terduga |
| T-27 | Pencabutan persetujuan lalu pemulihan kunci | Template **tidak** kembali; pencabutan tetap bersifat penghapusan |
| T-28 | Pemeriksaan kebocoran berkas kunci | Berkas kunci tidak ada di dalam repositori, tidak ada di direktori yang disinkronkan awan, dan tidak ada pada keluaran log |

## 3. Protokol evaluasi wajah

- Uji memakai peserta yang memahami dan menyetujui penggunaan datanya; gunakan ruang demo dengan pengawasan.
- Cakup **orang yang sama** dalam beberapa kondisi dan **orang berbeda**, termasuk yang tidak didaftarkan.
- Catat jumlah sampel, percobaan, model/versi, kondisi cahaya, kacamata, sudut, penolakan kualitas, kesalahan pencocokan, dan waktu respons.
- Ukur **salah-terima** (orang salah dianggap cocok) dan **salah-tolak** (orang benar tidak diterima). Jelaskan jika jumlah data terlalu kecil untuk menghitung laju yang stabil.
- Ambang kecocokan ditentukan melalui eksperimen; jangan memaksakan angka benchmark ke semua situasi.
- Rekam hasil dalam bentuk agregat dan kode peserta anonim; jangan unggah wajah nyata dalam laporan publik.
- Untuk spoofing, jelaskan metode uji dan keterbatasannya, bukan sekadar label “aman”.

## 4. Kriteria kualitas non-AI

- Semua tes unit dan kontrak yang berhubungan dengan perubahan lulus.
- Linter/typecheck berhasil dan dependensi diberi versi yang dapat direproduksi.
- Kegagalan server/perangkat tidak menyebabkan kesalahan status kehadiran.
- Tidak ada data biometrik, password, token atau gambar orang sungguhan di repository/log.
- Semua fitur P0 dari PRD memiliki bukti uji atau dinyatakan belum selesai secara terbuka.
- Jalur manual dan pencabutan persetujuan sudah diuji.

## 5. Template laporan hasil pengujian

```text
ID pengujian:
Tanggal dan pelaksana:
Commit/branch:
Perangkat dan versi software:
Kondisi awal serta data uji (anonim/sintetis):
Langkah atau perintah yang dijalankan:
Harapan:
Hasil aktual:
Lulus/Gagal/Terhambat:
Bukti yang boleh dibagikan:
Bug/risiko dan tindak lanjut:
```

**Tidak boleh** mengisi hasil lulus atau akurasi dengan angka rekaan. Laporan akhir presentasi harus membedakan bukti nyata, demo terbatas, dan kemampuan yang baru direncanakan.

## 6. Catatan FL-004: batas pengujian pengelolaan kunci

T-21 sampai T-28 di atas adalah **skenario yang direncanakan**, bukan hasil yang sudah dijalankan. Ketentuan yang mengikat:

1. Skenario kunci **baru boleh dijalankan setelah D-030 disetujui** dan implementasi penyimpanan biometrik diizinkan. Sebelum itu, tidak ada kode yang boleh menulis `face_templates`.
2. Hasil uji kehilangan kunci dan penggantian perangkat harus **diukur pada mesin demo sungguhan**, dengan Windows dan akun yang akan dipakai. Jangan menyimpulkan perilaku DPAPI dari dokumentasi lalu menuliskannya sebagai hasil uji.
3. Skenario T-22 harus menguji dua kondisi terpisah: (a) berkas kunci disalin ke **mesin lain**, dan (b) berkas kunci disalin ke **akun lain pada mesin yang sama**. Keduanya dapat berbeda hasilnya.
4. Bila mesin demo memakai akun dengan profil roaming, hasilnya **tidak** dapat digeneralisasi ke mesin lain; catat kondisi profil bersama hasilnya.
5. **Tidak boleh** ada klaim bahwa data biometrik aman terhadap serangan yang sudah menguasai akun Windows. Skenario seperti itu **di luar jangkauan** rancangan MVP dan harus disebutkan sebagai keterbatasan, bukan diuji lalu dinyatakan lulus.
6. Uji negatif lebih penting daripada uji positif di area ini: yang harus dibuktikan adalah backend **menolak bekerja** saat kunci hilang atau tidak cocok, bukan bahwa backend bekerja saat semuanya benar.

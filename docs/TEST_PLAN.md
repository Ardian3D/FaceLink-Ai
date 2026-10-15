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
| T-21 | Kunci aktif tidak ada saat backend dinyalakan | **Layanan biometrik gagal secara aman**: menolak bekerja dengan pesan jelas dan **tidak** membuat kunci baru sendiri serta **tidak** membuat template tironan. **Backend inti dan absensi manual tetap berjalan** selama database, autentikasi operator, dan komponen yang dibutuhkan sehat (paparan §5e butir 1–2; D-030 butir 10). Matinya seluruh backend karena kunci tidak ada dinyatakan **kegagalan**, bukan keamanan |
| T-22 | Berkas kunci disalin ke mesin/akun lain | Dekripsi gagal; tidak ada template yang terbaca dan tidak ada yang ditebak. Hasil dicatat apa adanya, termasuk bila gagal pada tahap yang tidak terduga |
| T-23 | Berkas kunci dihapus, database utuh | Penyalaan berikutnya menolak **layanan biometrik** dengan pesan jelas sementara absensi manual tetap melayani. **Tidak ada penghapusan template otomatis**: kondisi diperiksa lebih dahulu dan baris ditandai tidak dapat didekripsi, bukan dimusnahkan (kondisi A→C, `docs/SECURITY_PRIVACY.md` §5d.1–§5d.2). Status kegagalan **teramati dan dapat diaudit** lewat `audit_events` tanpa memuat potret, embedding, atau `ciphertext` |
| T-24 | `aad` pada baris diubah (uji perusakan) | Verifikasi AAD gagal; baris **tidak** dipakai dan backend mengembalikan “tidak dapat diverifikasi” |
| T-25 | `key_version` baris tidak sesuai kunci yang tersedia | Baris tidak dapat didekripsi; backend tidak menurunkan versi secara diam-diam dan tidak mencoba kunci lain di luar versi yang ditunjuk |
| T-26 | Cadangan database tanpa berkas kunci dipulihkan | Kehadiran dan nama tetap terbaca; kolom `ciphertext` **tidak** dapat dibaca. Ini harus dilaporkan sebagai keterbatasan, bukan kegagalan tak terduga |
| T-27 | Pencabutan persetujuan lalu pemulihan kunci | Template **tidak** kembali; pencabutan tetap bersifat penghapusan, dan pemulihan kunci tidak boleh menghidupkan kembali baris yang sudah dicabut |
| T-28 | Pemeriksaan kebocoran berkas kunci | Berkas kunci tidak ada di dalam repositori, tidak ada di direktori yang disinkronkan awan, dan tidak ada pada keluaran log |
| T-29 | Kondisi A — kunci sementara tidak tersedia | Berkas kunci dikunci proses lain / media dilepas / profil belum termuat. **Jumlah baris `face_templates` tidak berubah** dan tidak ada penghapusan yang dijalankan; operasi yang butuh template ditolak dan dicatat |
| T-30 | Kondisi B — kunci ada tetapi tidak dapat dipakai | `auth_tag` dirusak atau `key_version` ditunjuk ke kunci yang rusak. Baris **dikarantina**, tidak ditampilkan dan tidak dihitung cocok; **tidak ada** penghapusan, dan backend memeriksa salinan kunci lain lebih dahulu |
| T-31 | Kondisi C — kunci dipastikan tidak dapat dipulihkan | Seluruh salinan sah diperiksa dan habis. **Baru di sini** penghapusan diizinkan, dijalankan operator, dan tercatat di `audit_events`; kehadiran sebelumnya tetap sah dan tidak ditarik kembali |
| T-32 | Penghapusan otomatis tidak pernah terjadi | Dengan tugas terjadwal aktif dan kunci tidak tersedia, **tidak ada** baris yang terhapus tanpa tindakan operator |
| T-33 | Sisa biometrik setelah penghapusan | Setelah penghapusan template, periksa **database**, **`-wal`**, **`-shm`**, **cache**, **berkas log/kesalahan**, dan **salinan cadangan yang dikelola sistem ini**. Tidak ada sisa yang dapat dipulihkan dari lokasi yang **dikelola**; bila ada, penghapusan dinyatakan **belum selesai** dan dilaporkan sebagai bug. **Tidak ada klaim** penghapusan fisik absolut atau ketidakmungkinan pemulihan forensik — media penyimpanan (blok SSD, TRIM, pemetaan ulang) dan salinan di luar jangkauan sistem **di luar cakupan uji ini** dan disebutkan sebagai keterbatasan (paparan §5f) |
| T-34 | Absensi manual ketika tidak ada satu pun template | Seluruh template tidak tersedia (kondisi A atau C). Peserta **tetap** dapat hadir melalui jalur manual, dan kehadiran itu berlaku sama sahnya; tidak ada penolakan kehadiran |
| T-35 | Enrollment ulang dengan persetujuan baru | Peserta diminta ulang persetujuannya; bila menolak, **tidak ada** template baru dan tidak ada konsekuensi merugikan; `audit_events` mencatat status persetujuan tanpa potret atau embedding |
| T-36 | Kesiapan dipisahkan: absensi manual tanpa layanan biometrik | Layanan biometrik dimatikan atau kuncinya tidak tersedia. Absensi manual **tetap** melayani peserta, kehadiran tercatat, dan **tidak ada** pemeriksaan keamanan yang dilemahkan agar layanan hidup (paparan §5e butir 4) |
| T-37 | Urutan pemulihan setelah gangguan kunci | Setelah insiden kunci diselesaikan, **absensi manual dipulihkan lebih dahulu**; layanan biometrik menyusul. Kelas tidak menunggu penyelesaian insiden |
| T-38 | Tidak ada klaim penghapusan berlebihan | Laporan hasil penghapusan menyebut lokasi yang **belum** terjangkau (media penyimpanan, cadangan di luar jangkauan) apa adanya; frasa “terhapus permanen” dan “tidak dapat dipulihkan secara forensik” **tidak muncul** |

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

T-21 sampai T-38 di atas adalah **skenario yang direncanakan**, bukan hasil yang sudah dijalankan. Ketentuan yang mengikat:

1. Skenario kunci **baru boleh dijalankan setelah ada tugas implementasi yang disetujui pemilik**. D-030 **sudah disetujui pada FL-004S**, tetapi persetujuan itu berlaku atas **rancangan**; sampai tugas implementasi disetujui, tetap tidak ada kode yang boleh menulis `face_templates`.
2. Hasil uji kehilangan kunci dan penggantian perangkat harus **diukur pada mesin demo sungguhan**, dengan Windows dan akun yang akan dipakai. Jangan menyimpulkan perilaku DPAPI dari dokumentasi lalu menuliskannya sebagai hasil uji.
3. Skenario T-22 harus menguji dua kondisi terpisah: (a) berkas kunci disalin ke **mesin lain**, dan (b) berkas kunci disalin ke **akun lain pada mesin yang sama**. Keduanya dapat berbeda hasilnya.
4. Bila mesin demo memakai akun dengan profil roaming, hasilnya **tidak** dapat digeneralisasi ke mesin lain; catat kondisi profil bersama hasilnya.
5. **Tidak boleh** ada klaim bahwa data biometrik aman terhadap serangan yang sudah menguasai akun Windows. Skenario seperti itu **di luar jangkauan** rancangan MVP dan harus disebutkan sebagai keterbatasan, bukan diuji lalu dinyatakan lulus.
6. Uji negatif lebih penting daripada uji positif di area ini: yang harus dibuktikan adalah **layanan biometrik menolak bekerja** saat kunci hilang atau tidak cocok, bukan bahwa layanan itu bekerja saat semuanya benar. Penolakan itu **tidak boleh** menjalar menjadi kegagalan backend inti atau absensi manual (§5e butir 1–2; T-21, T-36).
7. **Ketiga kondisi §5d.1 harus dapat dibedakan pada pengujian nyata** (T-29, T-30, T-31). Bila backend tidak sanggup membedakan kondisi A dan B dari kondisi C, itu **temuan kegagalan**, bukan kelulusan — perilaku yang benar saat tidak yakin adalah **berhenti dan melaporkan**, bukan menghapus.
8. **Periode pemulihan 30 hari bukan objek uji kelulusan.** Yang diuji adalah bahwa penghapusan hanya terjadi setelah kondisi C ditetapkan dan tidak pernah dipicu otomatis (T-32). Jangan menuliskan hasil uji seolah masa 30 hari menjamin template dapat dipulihkan.
9. **Uji T-33 wajib memeriksa lebih dari berkas `facelink.db`**: berkas `-wal`, `-shm`, salinan cadangan, dan cache juga diperiksa. Penghapusan yang hanya membersihkan tabel induk dinyatakan **belum selesai**. Cakupan yang dimaksud adalah **lokasi yang dikelola sistem ini**; media penyimpanan dan salinan di luar jangkauan sistem **bukan** objek uji kelulusan dan harus disebut sebagai keterbatasan apa adanya (T-38, `docs/SECURITY_PRIVACY.md` §5f).
10. **Fallback absensi manual diuji sebagai jalur normal, bukan hanya saat darurat** (T-34). Bila jalur manual hanya bekerja ketika template masih ada, itu kegagalan yang harus dilaporkan.
11. **Dua klarifikasi FL-004S menjadi syarat lulus yang eksplisit.** (a) **T-36** harus membuktikan kesiapan dipisahkan: dengan layanan biometrik dimatikan atau kunci tidak tersedia, absensi manual **tetap berjalan** dan tidak ada pemeriksaan keamanan yang dilonggarkan untuk itu (`docs/SECURITY_PRIVACY.md` §5e). (b) **T-38** harus membuktikan tidak ada klaim penghapusan yang berlebihan: laporan akhir menyebut lokasi yang **belum** terjangkau (media penyimpanan, cadangan di luar jangkauan) apa adanya (`docs/SECURITY_PRIVACY.md` §5f).
12. **Kegagalan layanan biometrik bukan alasan melumpuhkan pemeriksaan keamanan.** Bila implementasi “berhasil” dengan cara menonaktifkan pemeriksaan agar layanan tetap hidup, hasil itu dinyatakan **gagal** sesuai `docs/SECURITY_PRIVACY.md` §5e butir 4.
13. **T-21 dan T-23 mengukur pemisahan kesiapan, bukan matinya backend.** Backend inti dan absensi manual **harus tetap berjalan** saat kunci tidak tersedia maupun saat berkas kunci dihapus; yang berhenti hanya layanan biometrik. Hasil uji yang menunjukkan seluruh backend menolak melayani dinyatakan **kegagalan**, bukan kehati-hatian berlebih.

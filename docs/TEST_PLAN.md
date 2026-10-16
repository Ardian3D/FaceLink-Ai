# Rencana Pengujian dan Gerbang Kualitas FaceLink AI

**Status:** Rencana pengujian, bukan laporan hasil. Angka akurasi/latensi hanya boleh dimasukkan setelah uji yang terukur.
**Revisi FL-010U (16 Oktober 2026):** T-01 disesuaikan ke Google Sign-In (bukan kata sandi) dan ditambahkan §8 untuk tiga role manusia (`admin`, `dosen`, `mahasiswa`) beserta autentikasi mesin `device`. Cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`). Seluruh skenario pada §8 berstatus **BELUM TERVERIFIKASI**.
**Revisi FL-010V (16 Oktober 2026):** §4 ditambahi persyaratan **log akses harus mencatat status respons sebenarnya** (`503` batas waktu dan `500` panic) sebagai syarat tugas penguatan **FL-005S** (kini **SELESAI secara implementasi pada 16 Oktober 2026** — lihat revisi berikut); §8 ditambahi **GA-13..GA-18** (§8.3) dan **TA-23..TA-26** (§8.4); status audit pada §8.2 butir 5 dinaikkan menjadi **BERLAKU menurut D-036**. Seluruh skenario baru berstatus **BELUM TERVERIFIKASI**; matriks 53 endpoint, nama peran, nama tabel, dan kode kesalahan **tidak berubah**.
**Revisi FL-005S (16 Oktober 2026):** §4 diperbarui — syarat **log akses mencatat status respons sebenarnya** kini **SUDAH DIPENUHI** oleh **FL-005S** (implementasi pada cabang `fix/fl-005s-http-access-logging`), dengan 12 skenario pada `backend/internal/httpapi/accesslog_test.go`. Status **BELUM TERVERIFIKASI** skenario pada §2, §7, dan §8.1–§8.4 (termasuk GA/TA) **tidak berubah**, dan matriks 53 endpoint, nama peran, nama tabel, serta kode kesalahan **tidak berubah**.

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
| T-01 | Login dengan **Google Sign-In** (bukan kata sandi) memakai ID token yang tidak sah/tidak dapat diverifikasi, atau akun tanpa akses | Tidak dapat masuk; `401 IDENTITY_TOKEN_INVALID` / `403 IDENTITY_NOT_LINKED` dengan pesan generik yang tidak membocorkan informasi |
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
- **Log akses harus mencatat status respons yang sebenarnya (FL-010V, 16 Oktober 2026) — SUDAH DIPENUHI oleh FL-005S (16 Oktober 2026).** Setiap respons yang dikirim backend Go — termasuk **`503` batas waktu** dan **`500` hasil pemulihan panic** — wajib tercatat dengan status yang benar-benar dikirim, bukan status lain. Log yang melaporkan `200` untuk respons `503` batas waktu, atau tidak mencatat respons `500` hasil pemulihan panic, dinyatakan **tidak memenuhi** kriteria ini. Persyaratan ini adalah **syarat tugas penguatan FL-005S** (penguatan log respons & pemulihan panic); **FL-005S SELESAI secara implementasi pada 16 Oktober 2026** di cabang `fix/fl-005s-http-access-logging`, sehingga kriteria ini **tidak lagi berstatus belum dikerjakan**. Perubahan `backend/internal/httpapi/router.go` menetapkan urutan middleware **satu kali** pada fungsi `chain` (`RequestID → echoRequestID → accessLog → recoverer → timeout → bodyLimit`), sehingga status pada access log adalah status final yang benar-benar dikirim klien (`503` batas waktu dan `500` panic tercatat) dan barisnya ditulis **tepat satu kali per permintaan** saat respons selesai. Skenario access log yang kini ada pada `backend/internal/httpapi/accesslog_test.go` (berkas uji baru, 12 pengujian) adalah: (1) `200`; (2) `404`; (3) `405`; (4) `413`; (5) `500` saat panic dipulihkan; (6) `503` saat batas waktu; (7) kecocokan status saat handler menulis berdekatan dengan batas waktu; (8) baris log sudah ditulis sebelum handler lambat selesai; (9) satu baris per permintaan; (10) log tidak memuat rahasia; (11) kontrak respons tidak berubah; (12) permintaan yang dibatalkan klien tetap satu baris. **Gate lokal lulus:** `gofmt` bersih, `go vet`, `go build`, `go test -count=1 ./...`, `go test -race -count=1 ./...`, dan `go test -race -count=3` pada pengujian access log, ditambah smoke test pada server sungguhan (access log `200`/`404`/`405`/`413` sesuai status yang diterima klien). Jalur `503` dan `500` diuji **in-process** (tidak ada endpoint produk yang lambat atau panic) dan **tidak ada endpoint baru** yang ditambahkan. **Belum di-merge** ke `main` sampai PR ditinjau, dan CI GitHub Actions belum dilaporkan lulus; karena itu keandalan logging keamanan (audit D-036) baru boleh dinyatakan terverifikasi secara operasional setelah PR di-merge dan pengujian nyata dijalankan. Tidak ada kode yang diubah pada FL-010V; catatan ini hanya mendaftarkan syarat dan statusnya.

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

## 7. Catatan FL-010: skenario penolakan akses dan otorisasi

FL-010 menetapkan matriks peran dan izin di `docs/AUTHORIZATION.md`. Skenario negatifnya bernomor **TA-01 sampai TA-15** dan **melengkapi** T-01, T-02, dan T-15 di §2 — bukan menggantinya.

| ID | Skenario | Harapan |
|---|---|---|
| TA-01 | Tanpa token memanggil `GET /people` | `401 UNAUTHENTICATED`; tanpa data |
| TA-02 | Token kedaluwarsa memanggil `GET /sessions` | `401 TOKEN_EXPIRED` |
| TA-03 | Token perangkat memanggil `/people`, `/sessions`, `/devices`, `/audit-events` | `403 FORBIDDEN_ROLE` pada keempat jalur — *FL-010U: tidak ada lagi role `operator`; peran yang harus ditolak di sini adalah `admin`, `dosen`, dan `mahasiswa`* |
| TA-04 | Token operator memanggil `recognition-attempts` | `403 FORBIDDEN_ROLE` (bukan `200`) — *FL-010U: istilah "token operator" **digantikan**; yang dimaksud token peran manusia `admin`/`dosen`/`mahasiswa` (lihat §8)* |
| TA-05 | Token perangkat memakai `device_id` perangkat lain | `403`; audit `auth.device.mismatch` (usulan, `docs/AUTHORIZATION.md` §9 butir 4) |
| TA-06 | `operator` memanggil `GET`/`POST /users` | `403 FORBIDDEN_ROLE`; audit `authz.denied` (usulan) — *FL-010U: istilah `operator` **digantikan**; baris ini berlaku untuk peran manusia `dosen` dan `mahasiswa` (lihat §8)* |
| TA-07 | `operator` memanggil `DELETE /people` dan `POST /devices` | `403 FORBIDDEN_ROLE` — *FL-010U: dibaca sebagai `dosen`/`mahasiswa`; kedua jalur kini hanya `admin`* |
| TA-08 | `operator` mencoba mengubah `role` dirinya sendiri | Tidak ada endpoint yang mengizinkan; peran tidak berubah — *FL-010U: dibaca sebagai salah satu peran manusia `admin`/`dosen`/`mahasiswa`* |
| TA-09 | Koreksi manual untuk peserta bukan anggota kelas | `422 CLASS_MISMATCH`; tanpa baris kehadiran |
| TA-10 | Koreksi manual tanpa `reason` | Ditolak (`400`/`422 REASON_REQUIRED`) |
| TA-11 | Perangkat sesi A mengirim percobaan untuk sesi B | `422 DEVICE_SESSION_MISMATCH` |
| TA-12 | Perangkat tercabut mengirim `recognition-attempts` | `403 DEVICE_REVOKED` |
| TA-13 | Akses objek kelas lain oleh operator | `404 NOT_FOUND`; tidak membocorkan keberadaan — *FL-010U: dibaca sebagai `dosen` di luar kelas yang ditugaskan (lihat §8, TA-16)* |
| TA-14 | `GET /audit-events` setelah koreksi manual | `safe_metadata` bebas token/sandi/JPEG/embedding |
| TA-15 | Komponen autentikasi dimatikan, lalu `POST /manual-checkins` | Ditolak (gagal-aman); jalur manual hanya lewat endpoint terautentikasi yang sehat |

**Ketentuan yang mengikat untuk TA-01 sampai TA-15:**

1. Skenario ini adalah **rencana**, bukan hasil. Statusnya **BELUM TERVERIFIKASI** sampai kode autentikasi/otorisasi ditulis pada tugas implementasi tersendiri.
2. Uji negatif adalah inti: yang dibuktikan adalah **penolakan**, bukan keberhasilan jalur yang sah.
3. `401` dan `403` harus dibedakan sesuai `docs/AUTHORIZATION.md` §6.1; jawaban tidak boleh membocorkan keberadaan sumber daya lain.
4. Setiap skenario audit (TA-05, TA-06, TA-14) harus memeriksa isi `safe_metadata`, bukan hanya keberadaan barisnya.
5. TA-15 menguji `docs/SECURITY_PRIVACY.md` §5e butir 4: layanan boleh turun, pemeriksaan keamanan **tidak** boleh dilonggarkan agar layanan tampak hidup.

**Catatan FL-010V (16 Oktober 2026):** pada TA-05 dan TA-06 di atas, kata "(usulan)" bagi `auth.device.mismatch` dan `authz.denied` **tidak lagi berlaku**: menurut **D-036** seluruh daftar audit keamanan — termasuk kedua event itu dan event pada §8.1 — berstatus **BERLAKU** (`docs/SECURITY_PRIVACY.md` §6). Baris skenario TA-05/TA-06 sendiri **tidak diubah**; hanya status auditnya. Kode audit belum ditulis, sehingga hasil uji tetap **BELUM TERVERIFIKASI**; keandalan pencatatannya kini bersandar pada **FL-005S yang SELESAI secara implementasi pada 16 Oktober 2026** (§4), yang masih menunggu PR di-merge sebelum boleh dinyatakan terverifikasi secara operasional.

**Catatan FL-010R (16 Oktober 2026):** TA-04 menguji penolakan token operator **dan** token admin pada `recognition-attempts` — jalur itu bersifat `device` saja, sehingga rekap `docs/AUTHORIZATION.md` §4.12 menyatakan `admin` mengakses 40 endpoint (bukan 43). Bila implementasi kelak menerima token admin pada jalur itu, bukan hanya TA-04 yang gagal, tetapi juga angka rekap §4.12 menjadi keliru.

---

## 8. Catatan FL-010U (16 Oktober 2026): autentikasi Google dan tiga role

**Diperluas pada FL-010V (16 Oktober 2026):** ditambahkan §8.3 (GA-13..GA-18) dan §8.4 (TA-23..TA-26); status audit pada §8.2 butir 5 dinaikkan menjadi **BERLAKU (D-036)**. Isi §8.1 dan §8.2 **tidak diubah** selain catatan status audit tersebut.

FL-010U menetapkan tiga role manusia (`admin`, `dosen`, `mahasiswa`) yang masuk dengan **Google Sign-In**, serta `device` sebagai autentikasi mesin terpisah yang tidak memakai Google. Akun `mahasiswa` ditautkan ke baris peserta oleh `admin` dan hanya melihat datanya sendiri. Matriks izin ada di `docs/AUTHORIZATION.md` §2 dan §4; nama endpoint dan kode kesalahan ada di `docs/API_CONTRACT.md`. Skenario §8 **melengkapi** §2 (T-xx) dan §7 (TA-01..TA-15), bukan menggantinya. Cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`).

**Status: SELURUH skenario §8 (GA-01..GA-18 dan TA-16..TA-26) berstatus BELUM TERVERIFIKASI.** Tidak satu pun sudah dijalankan, dan tidak ada hasil yang boleh dilaporkan sebagai lulus sebelum kode autentikasi Google, sesi, dan otorisasi tiga role ditulis pada tugas implementasi tersendiri. **FL-010V (16 Oktober 2026)** menambahkan GA-13..GA-18 (§8.3) dan TA-23..TA-26 (§8.4); perluasan skenario **tidak** mengubah status ini dan **tidak** mengubah matriks 53 endpoint, nama peran, nama tabel, maupun kode kesalahan.

Tiga hal yang dinyatakan sebagai **istilah yang digantikan**: (a) `operator` bukan lagi peran; pada §2 (T-02, T-15) dan §7 (TA-04, TA-06, TA-07, TA-08, TA-13) istilah itu dibaca sebagai peran manusia `admin`/`dosen`/`mahasiswa` sesuai matriks peran — pada TA-13 dibaca `dosen` di luar kelas yang ditugaskan; (b) pada §6, `operator` di T-21, T-31, dan T-32 dibaca sebagai `admin`, karena operasi kunci dan template berada pada jalur `admin`; (c) "kata sandi" pada T-01 diganti **ID token Google**. Catatan angka rekap FL-010R di akhir §7 adalah **catatan historis** untuk matriks 43 endpoint (`operator`/`admin`/`device`) yang sudah **digantikan**; matriks yang berlaku sekarang adalah **53 endpoint** pada `docs/AUTHORIZATION.md` §4, dan rekap §4.14 dokumen itu sudah dihitung ulang secara programatis pada FL-010U (29/14/3/1/3/1/2 = 53; per peran: `admin` 46, `dosen` 18, `mahasiswa` 7, `device` 1, publik 2).

### 8.1 Skenario autentikasi Google, akun, dan sesi (GA-01..GA-12)

| ID | Skenario | Harapan |
|---|---|---|
| GA-01 | ID token dengan `aud` (audience) di luar daftar client ID resmi | `401 IDENTITY_TOKEN_INVALID`; tidak ada sesi diterbitkan dan tidak ada baris akun yang ditulis |
| GA-02 | ID token dengan `iss` di luar `https://accounts.google.com`/`accounts.google.com` | `401 IDENTITY_TOKEN_INVALID` |
| GA-03 | ID token bertanda tangan salah, `alg` bukan RS256 (`none`/HS256), atau `kid` tidak dikenal | `401 IDENTITY_TOKEN_INVALID`; kunci diambil/dirotasi lewat JWKS dan tidak ada kunci yang dipercaya buta |
| GA-04 | ID token kedaluwarsa melewati toleransi selisih waktu | `401 IDENTITY_TOKEN_INVALID` |
| GA-05 | Klaim `nonce` tidak cocok (bila aplikasi memakai nonce) atau `email_verified` bukan `true` | `401 IDENTITY_TOKEN_INVALID` |
| GA-06 | Login pertama akun `invited` dengan email sama persis (huruf kecil, tanpa spasi tepi) dan `email_verified=true` | Identitas `(provider='google', subject=sub)` tertaut, status akun menjadi `active`, sesi diterbitkan, dan role diambil dari database. `sub` berbeda dengan email yang sama → akun **tidak** ditautkan |
| GA-07 | Akun berstatus `suspended` mencoba masuk | Ditolak `403 IDENTITY_NOT_LINKED` dengan **satu pesan generik**; tidak ada sesi; audit usulan `auth.login.denied` dengan `reason='suspended'` dan tanpa email |
| GA-08 | Identitas Google yang **tidak punya baris `users`** (termasuk Google pertama di sistem) | **Tidak ada akun dan tidak ada role yang dibuatkan**; tidak ada pendaftaran mandiri; `403 IDENTITY_NOT_LINKED` dengan pesan generik; audit usulan `auth.login.denied` dengan `reason='no_account'` |
| GA-09 | `POST /auth/refresh` memakai token penyegar yang sudah dirotasi | **Seluruh sesi akun dicabut**; `401 SESSION_REVOKED`; audit usulan `auth.session.reuse_detected` |
| GA-10 | `POST /auth/logout` | Sesi itu dicabut (`204`); pemanggilan kedua tetap `204`; token akses dan penyegar sesi itu tidak dapat dipakai lagi |
| GA-11 | `admin` menonaktifkan akun (`status='suspended'`) saat sesinya masih berjalan | Seluruh sesi akun itu dicabut; permintaan berikutnya `401 SESSION_REVOKED` karena role dan status dibaca ulang dari database, bukan dari klaim token |
| GA-12 | `admin` mencoba menonaktifkan/menurunkan peran admin aktif terakhir, atau akunnya sendiri | `409 LAST_ADMIN_PROTECTED` / `409 SELF_ACCOUNT_PROTECTED`; tidak ada perubahan status atau role, dan tidak ada jalur massal |

GA-01 sampai GA-08 adalah uji negatif/positif pada jalur `POST /auth/google`; GA-09 sampai GA-12 menguji siklus sesi dan lifecycle akun. Semuanya **BELUM TERVERIFIKASI**. Kata "usulan" pada kolom audit GA-07/GA-08/GA-09 **tidak lagi berlaku sejak FL-010V (16 Oktober 2026)**: `auth.login.denied` dan `auth.session.reuse_detected` berstatus **BERLAKU** menurut D-036 (`docs/SECURITY_PRIVACY.md` §6), sementara **kode auditnya belum ditulis dan belum diuji**.

### 8.2 Skenario aturan scope dan role baru (TA-16..TA-22)

| ID | Skenario | Harapan |
|---|---|---|
| TA-16 | `dosen` meminta kelas, sesi, atau koreksi manual di luar kelas yang ditugaskan | `404 NOT_FOUND` (bukan `403`), tanpa membocorkan keberadaan objek; `GET /classes` hanya mengembalikan kelas yang ditugaskan |
| TA-17 | `mahasiswa` mencoba membaca data peserta lain (objek atau penunjuk milik orang lain) | Objek di luar cakupan pemanggil dijawab `404 NOT_FOUND`; endpoint yang memang bukan hak peran `mahasiswa` dijawab `403 FORBIDDEN_ROLE`; tidak ada data peserta lain, embedding, atau template yang tersaji |
| TA-18 | Akun `mahasiswa` yang belum ditautkan ke baris peserta memanggil `GET /me/classes`, `GET /me/schedule`, `GET /me/attendance`, dan `GET /me/consent` | `409 ACCOUNT_NOT_LINKED` pada keempat jalur; tidak ada data pribadi yang tersaji |
| TA-19 | `dosen` memanggil operasi yang kini hanya `admin`: `GET /audit-events`, `GET /sessions/{session_id}/export.csv`, seluruh `/devices*`, dan `GET`/`POST /users` | `403 FORBIDDEN_ROLE`; **`GET /devices` tidak lagi dapat dipanggil `dosen`**; tidak ada data tersaji |
| TA-20 | Klien mengirim `role` pada permintaan, atau ID token Google memuat klaim tambahan (mis. role/email admin) | Role tetap dibaca dari database; klaim dari token atau klien **diabaikan**; tidak ada endpoint yang mengubah role akun sendiri, dan ID token Google tidak pernah dipakai sebagai sesi aplikasi |
| TA-21 | `admin` mencoba menonaktifkan atau menurunkan peran **admin aktif terakhir**, lalu mencoba melakukannya pada **akunnya sendiri** | `409 LAST_ADMIN_PROTECTED` dan `409 SELF_ACCOUNT_PROTECTED`; tidak ada perubahan status/role dan tidak ada jalur massal (melengkapi GA-12 dari sisi matriks peran) |
| TA-22 | Token penyegar lama dikirim **setelah** rotasi | `401 SESSION_REVOKED`; seluruh sesi akun dicabut; `audit_events` mencatat pemakaian ulang (melengkapi GA-09 dari sisi sesi) |

TA-16 sampai TA-22 menguji penerapan **matriks peran** (`docs/AUTHORIZATION.md` §4–§5); GA-01 sampai GA-12 menguji **autentikasi Google, lifecycle akun, dan siklus sesi** (§8.1). Keduanya saling melengkapi dan tidak menggantikan satu sama lain.

**Ketentuan yang mengikat untuk GA-01..GA-12 dan TA-16..TA-22:**

1. Semuanya **rencana, bukan hasil**: status **BELUM TERVERIFIKASI** sampai kode autentikasi/otorisasi ditulis pada tugas implementasi tersendiri. Tidak ada laporan yang boleh menyebut salah satunya "lulus" sebelum dijalankan pada mesin uji.
2. Uji negatif adalah inti: yang dibuktikan adalah **penolakan** (token tidak sah, akun tanpa akses, akun dinonaktifkan, objek di luar cakupan), bukan keberhasilan jalur yang sah.
3. Pembedaan `401`/`403`/`404` mengikuti `docs/AUTHORIZATION.md` §6.1; objek di luar cakupan pemanggil (kelas, sesi, atau data orang lain) dijawab `404 NOT_FOUND` agar tidak membocorkan keberadaan sumber daya, sedangkan endpoint yang bukan hak peran dijawab `403 FORBIDDEN_ROLE`.
4. Semua penolakan terkait keberadaan/status akun memakai **satu pesan generik** (`403 IDENTITY_NOT_LINKED`) dan auditnya memakai `reason` ber-kode (`no_account`, `not_linked`, `suspended`, `email_mismatch`), **bukan** email. `safe_metadata` tidak pernah memuat email, ID token, `sub`, token sesi, JPEG, atau embedding.
5. **Audit keamanan (diperbarui FL-010V, 16 Oktober 2026):** daftar perluasan audit **BERLAKU** menurut **D-036** (disetujui pemilik), bukan lagi usulan. Daftar nama event final: `auth.google.succeeded`, `auth.google.linked`, `auth.login.denied` (dengan `reason` ber-kode: `no_account`, `not_linked`, `suspended`, `email_mismatch`), `auth.session.refreshed`, `auth.session.revoked`, `auth.session.reuse_detected`, `auth.logout`, `authz.denied`, `auth.device.mismatch`, `user.bootstrapped`, `user.invited`, `user.role_changed`, `user.status_changed`, `person.account_linked`, `person.account_unlinked`, `class.dosen_assigned`, `class.dosen_removed`, ditambah `consent.granted` dan `consent.withdrawn` yang sudah berlaku sebelumnya. Larangan isi audit: `audit_events` dan `safe_metadata` **tidak boleh** memuat password, access token, refresh token, Google ID token, embedding wajah, foto wajah, maupun kredensial perangkat. **Berlaku bukan berarti teruji:** belum ada kode audit yang ditulis, sehingga hasil uji hanya boleh melaporkan apa yang benar-benar teramati, dan keandalan logging keamanan kini bersandar pada **FL-005S yang SELESAI secara implementasi pada 16 Oktober 2026** (§4), yang masih menunggu PR di-merge.
6. Jalan pintas yang membuat skenario "lulus" secara palsu harus dinyatakan **gagal**: menerima `alg` selain RS256, mempercayai klaim role dari ID token atau klien, membuat akun/role otomatis saat login Google tanpa baris `users`, dan memakai ID token Google sebagai token sesi aplikasi.
7. Skenario GA dan TA pada §8 **tidak menguji absensi/biometrik**; batas uji kunci dan template tetap mengikuti §6 (`docs/SECURITY_PRIVACY.md` §5d–§5f) dan tidak diubah oleh FL-010U.

### 8.3 Skenario tambahan FL-010V: penautan, nonce, batas sesi, dan audit (GA-13..GA-18)

**Ditambahkan pada FL-010V (16 Oktober 2026).** Skenario ini melengkapi GA-01..GA-12 (§8.1) dan **tidak menggantikannya**; seluruhnya berstatus **BELUM TERVERIFIKASI**.

| ID | Skenario | Harapan |
|---|---|---|
| GA-13 | Dua permintaan login identik untuk akun `invited` yang sama dikirim **bersamaan** (penautan bersamaan) | Hanya **satu** penautan berhasil; **tidak ada baris ganda** pada `user_identities`, `session_tokens`, maupun `users`; permintaan yang kalah ditolak secara aman tanpa membuat identitas/akun kedua. Penjaganya adalah transaksi atomik **dan** batasan unik `(provider, subject)` serta `(user_id, provider)`, bukan sekadar pemeriksaan aplikasi |
| GA-14 | Email Google diubah **setelah** identitas tertaut, lalu pengguna login lagi | Akun **tidak dialihkan**: pengenal utama setelah penautan adalah `subject` (`sub`), bukan email; pencocokan email **tidak pernah** dipakai lagi setelah penautan. Tidak ada account takeover, tidak ada perubahan `user_id`, dan tidak ada identitas baru yang ditautkan ke akun lain. Kegagalan bila implementasi masih mencocokkan email pada login berikutnya |
| GA-15 | Satu identitas Google (`provider='google'`, `subject` sama) mencoba menautkan **dua akun** `users` berbeda | Ditolak; batasan unik `(provider, subject)` dan `(user_id, provider)` mencegahnya. Tidak ada akun kedua yang tertaut, tidak ada sesi kedua yang diterbitkan, dan tidak ada mekanisme resolusi otomatis yang menebak akun tujuan |
| GA-16 | Nonce dipakai ulang (nonce yang sama dikirim pada percobaan login kedua), atau nonce kedaluwarsa melewati TTL 5 menit | Ditolak `401 IDENTITY_TOKEN_INVALID`; tidak ada sesi diterbitkan; pemeriksaan juga memastikan nonce disimpan **hanya sebagai hash** pada `auth_nonces` dan tidak muncul di log atau `audit_events` |
| GA-17 | `POST /auth/refresh` dipakai berulang sepanjang masa sesi | Penyegaran **tidak** memperpanjang **batas absolut 30 hari** sejak sesi awal dibuat; setelah batas tercapai penyegaran ditolak (`401`), seluruh jalur sesi lama berhenti, dan pengguna wajib **Google Sign-In ulang**. Kegagalan bila rotasi memperpanjang batas absolut tanpa autentikasi baru yang sah |
| GA-18 | Skenario audit keamanan dijalankan (login berhasil/gagal, penautan, rotasi, pencabutan, penolakan otorisasi, ketidakcocokan perangkat, perubahan role/status) | Event D-036 yang relevan benar-benar tercatat, dan isinya **tidak** memuat token, ID token, atau embedding. Pemeriksaan mencakup: tidak ada access token, refresh token, Google ID token, embedding, foto, password, maupun kredensial perangkat pada `audit_events` dan `safe_metadata`; `reason` ber-kode dipakai untuk penolakan akun, bukan email. Hasil hanya sah bila **FL-005S** sudah selesai — **FL-005S SELESAI secara implementasi pada 16 Oktober 2026** (§4) — karena log respons yang keliru membuat keandalan pencatatan tidak dapat disimpulkan |

**Ketentuan tambahan untuk GA-13..GA-18:** statusnya **BELUM TERVERIFIKASI**; uji negatif adalah inti (penolakan penautan ganda, penolakan nonce bekas, penolakan penyegaran melewati batas absolut); **dilarang** membuat skenario "lulus" dengan menerima penautan berdasarkan email setelah penautan pertama, menerima nonce bekas, atau memperpanjang batas absolut tanpa autentikasi baru.

### 8.4 Skenario tambahan FL-010V: audit minimal, TLS LAN, dan larangan bypass (TA-23..TA-26)

**Ditambahkan pada FL-010V (16 Oktober 2026).** Skenario ini melengkapi TA-01..TA-22 (§7 dan §8.2) dan **tidak menggantikannya**; seluruhnya berstatus **BELUM TERVERIFIKASI**.

| ID | Skenario | Harapan |
|---|---|---|
| TA-23 | `dosen`/`mahasiswa` memanggil endpoint yang bukan haknya (mis. `GET /audit-events`, `GET`/`POST /users`), lalu `GET /audit-events` diperiksa | Tercatat `authz.denied` dengan `safe_metadata` **minimal** — hanya identifier internal yang diperlukan (mis. role, jalur/endpoint, alasan penolakan) — dan **tidak** memuat token, ID token, email, `sub`, JPEG, embedding, maupun kredensial perangkat. Yang diperiksa adalah **isi** `safe_metadata`, bukan hanya keberadaan barisnya |
| TA-24 | Token perangkat memakai `device_id` perangkat lain (melengkapi TA-05), lalu `GET /audit-events` diperiksa | Tercatat `auth.device.mismatch` dengan `safe_metadata` **minimal** — tanpa token perangkat, tanpa kredensial perangkat, tanpa ID token, dan tanpa embedding; tidak ada data perangkat lain yang tersaji. Sebagai pemeriksaan tambahan: tidak ada satu pun baris audit keamanan pada skenario ini yang memuat access token, refresh token, Google ID token, foto wajah, password, atau kredensial perangkat; penolakan akun memakai `reason` ber-kode (`no_account`, `not_linked`, `suspended`, `email_mismatch`), bukan email |
| TA-25 | Aplikasi mengakses backend lewat LAN **tanpa** TLS tepercaya (sertifikat tidak tepercaya, sertifikat kedaluwarsa, atau HTTP terbuka), lalu dicoba diakali dengan menonaktifkan validasi | Akses **ditolak** atau diarsitekturkan aman: klien tidak memercayai sertifikat yang tidak sah, tidak ada `InsecureSkipVerify`/penonaktifan validasi pada konfigurasi mana pun, dan tidak ada token yang dikirim di atas koneksi yang gagal diverifikasi. Pengikatan LAN hanya aktif lewat konfigurasi eksplisit dengan default loopback. Hasil uji **tidak boleh** menyimpulkan TLS LAN sudah berfungsi (D-035, `docs/SECURITY_PRIVACY.md` §6) |
| TA-26 | Internet dimatikan **dan** sesi aplikasi tidak sah (kedaluwarsa, dicabut, atau batas absolut 30 hari tercapai), lalu dicoba seluruh jalur yang mungkin | **Tidak ada jalur bypass autentikasi**: endpoint terproteksi tetap menolak (`401`), aplikasi tidak membuka layar operasi atau data lama, dan tidak ada kredensial lokal/kunci darurat. Absensi manual mengikuti prosedur operasional aman di luar aplikasi, lalu dimasukkan operator **setelah terautentikasi kembali** melalui absensi manual terautentikasi dengan `reason`; kehadiran yang tidak dapat dipastikan dibiarkan tidak tercatat, bukan diisi tebakan. Rincian prosedur tetap **Q-24** dan **BELUM TERVERIFIKASI** (`docs/UX_FLOWS.md` §3a) |

**Ketentuan tambahan untuk TA-23..TA-26:** statusnya **BELUM TERVERIFIKASI**; hasil uji tidak boleh melaporkan audit sebagai "andalkan" sebelum perbaikan **FL-005S (SELESAI secara implementasi pada 16 Oktober 2026, §4)** di-merge dan diverifikasi; dan **dilarang** membuat skenario "lulus" dengan melonggarkan validasi TLS, memakai HTTP terbuka sebagai anjuran, atau menyediakan jalur operasi tanpa sesi yang sah.

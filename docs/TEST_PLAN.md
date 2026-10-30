# Rencana Pengujian dan Gerbang Kualitas FaceLink AI

**Status:** Rencana pengujian, bukan laporan hasil. Angka akurasi/latensi hanya boleh dimasukkan setelah uji yang terukur.
**Revisi FL-010U (16 Oktober 2026):** T-01 disesuaikan ke Google Sign-In (bukan kata sandi) dan ditambahkan §8 untuk tiga role manusia (`admin`, `dosen`, `mahasiswa`) beserta autentikasi mesin `device`. Cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`). Seluruh skenario pada §8 berstatus **BELUM TERVERIFIKASI**.
**Revisi FL-010V (16 Oktober 2026):** §4 ditambahi persyaratan **log akses harus mencatat status respons sebenarnya** (`503` batas waktu dan `500` panic) sebagai syarat tugas penguatan **FL-005S** (kini **SELESAI secara implementasi pada 16 Oktober 2026** — lihat revisi berikut); §8 ditambahi **GA-13..GA-18** (§8.3) dan **TA-23..TA-26** (§8.4); status audit pada §8.2 butir 5 dinaikkan menjadi **BERLAKU menurut D-036**. Seluruh skenario baru berstatus **BELUM TERVERIFIKASI**; matriks 53 endpoint, nama peran, nama tabel, dan kode kesalahan **tidak berubah**.
**Revisi FL-005S (16 Oktober 2026):** §4 diperbarui — syarat **log akses mencatat status respons sebenarnya** kini **SUDAH DIPENUHI** oleh **FL-005S** (implementasi pada cabang `fix/fl-005s-http-access-logging`), dengan 12 skenario pada `backend/internal/httpapi/accesslog_test.go`. Status **BELUM TERVERIFIKASI** skenario pada §2, §7, dan §8.1–§8.4 (termasuk GA/TA) **tidak berubah**, dan matriks 53 endpoint, nama peran, nama tabel, serta kode kesalahan **tidak berubah**.

**FL-STAB02 — diagnosis/lifecycle terbatas selesai (29 Oktober 2026):**
controller auth/Admin/Student85/85PASS; middleware inspector aktual mengirim
getPages tanpa debugger frontend. Release-like ARM64 build exit0, lalu diuji
di TECNO setelah izin baru bersama kedua debug staging: OTP/session restore/
logout, preview close dan offline ACTIVE-unverified. Fixture minimal real
worker→AADv2 ACTIVE→withdraw→REVOKED/cleanup0; native identical replay202→200
tanpa audit ganda. **Uncertainty/manual replay UI belum terdemonstrasi.**
Satu fatal historis tetap,0 fatal baru teramati; root cause **UNRESOLVED**,
bukan fixed atau PASS semua lifecycle. Attach WS berhasil, CDP round-trip belum
terbukti. Enam package diinventaris, tidak uninstall. Staging/host/reverse ditutup.
Bukti dan batas: `FL_STAB02_TECHNICAL_REPORT.md`; Fast-Track **PARTIAL**.

**Bukti FL-ENR06-R1 (29 Oktober 2026):** positive Android chain satu submission
PASS melalui dua package staging terisolasi di TECNO: consent→pending→Admin
review/approval202→real YuNet/SFace→AADv2 ACTIVE→Student withdrawal→REVOKED/
cleanup0. Form history-shift direproduksi dan fix minimal diverifikasi actual
UI. Offline trust, preview close/logout, ETag412 dan response-loss duplicate200
teramati; layar uncertain/manual replay Android tidak teramati. Satu crash
native dev inspector masih terbuka: **verdict keseluruhan PARTIAL**, bukan PASS
semua lifecycle/accessibility. Go18paket+vet exit0, Python exit0/satu skip,
mobile229PASS/9SKIP+typecheck/lint. Runtime task dihentikan aman, hash artefak
utama/model/video tetap. Laporan/batas A–L: `FL_ENR06_R1_TECHNICAL_REPORT.md`.
**STOP tanpa task berikut/human/production/FL-UI01 otomatis.**
Suite client opt-in terakhir236PASS/2SKIP; combined HTTPS dipanggil terpisah
dengan environment eksplisit dan lulus2/2. Bukan klaim satu invocation238PASS.

**Bukti FL-STAB01 (27 Oktober 2026):** F1 expired-access logout FAIL sebelum fix,
PASS setelah refresh-proof revocation; persisted revoke/old-refresh401,
idempotensi, isolation, concurrency, restart dan atomic audit rollback tercakup.
F2 actual-source animation race FAIL sebelum fix dan PASS sesudah; native TECNO
berizin menjalankan login/logout tiga role, restore, navigasi dan background.
Final Go17+vet; main/stage219 PASS/6 skipped; HTTPS E2E Admin5/5 dan Mahasiswa1/1;
stage ARM64 build sukses. Native proof bounded, F3 tetap terbuka.
Laporan A–K dan batas: [`FL_STAB01_TECHNICAL_REPORT.md`](FL_STAB01_TECHNICAL_REPORT.md).


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

**Revisi FL-011 (16 Oktober 2026):** status skenario §8 **diperbarui dari rencana menjadi hasil nyata** untuk bagian yang kodenya sudah ada. Pemetaan lengkap ada di **§8.5**; ringkasnya, dari **29 skenario** §8 (GA-01..GA-18, TA-16..TA-26): **20 TERVERIFIKASI**, **7 TERVERIFIKASI sebagian**, dan **2 BELUM TERVERIFIKASI** — dua sisanya (TA-24, TA-26) bergantung pada perangkat IoT dan aplikasi klien yang memang di luar lingkup FL-011. Matriks endpoint bertambah satu baris (`POST /auth/nonce`) menjadi **54 endpoint**; nama peran, nama tabel lain, dan kode kesalahan **tidak berubah**. Angka-angka pada §8.5 diambil dari nama uji yang benar-benar ada di repositori, bukan dari klaim.

**Status: skenario §8 sebagian sudah TERVERIFIKASI (FL-011).** Pernyataan "seluruhnya BELUM TERVERIFIKASI" pada revisi FL-010U **tidak lagi berlaku** untuk butir yang tercantum di §8.5; butir yang belum diuji tetap dinyatakan **BELUM TERVERIFIKASI** dan tidak boleh dilaporkan sebagai lulus. **FL-010V (16 Oktober 2026)** menambahkan GA-13..GA-18 (§8.3) dan TA-23..TA-26 (§8.4); perluasan skenario **tidak** mengubah matriks endpoint selain satu baris baru pada FL-011 (`POST /auth/nonce`, total menjadi 54), dan **tidak** mengubah nama peran, nama tabel, maupun kode kesalahan.

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

### 8.5 Pemetaan skenario GA/TA ke hasil pengujian nyata (FL-011, 16 Oktober 2026)

Bagian ini menggantikan status "rencana" untuk butir yang **sudah dijalankan**. Isinya dipetakan dari nama uji yang benar-benar ada di repositori pada cabang `feat/fl-011-google-auth-backend`. Perintah yang dijalankan dari `backend/`: `go test -count=1 ./...` dan `go test -race -count=1 ./...` (keduanya lulus; hasilnya dicatat pada `PROJECT_STATE.md`). Tidak ada skenario yang dinyatakan lulus tanpa nama uji di kolom bukti.

| ID | Status | Bukti (uji otomatis) |
|---|---|---|
| GA-01 | **TERVERIFIKASI** | `TestLoginGoogleMenolakBuktiIdentitasTidakSah/audience_tidak_dikenal`, `TestVerifyPenolakan` (audience di luar daftar ditolak) |
| GA-02 | **TERVERIFIKASI** | `TestLoginGoogleMenolakBuktiIdentitasTidakSah/issuer_tidak_dikenal`, `TestVerifyIssuerKhusus` |
| GA-03 | **TERVERIFIKASI** | `TestLoginGoogleMenolakBuktiIdentitasTidakSah/tanda_tangan_dari_kunci_asing`, `TestVerifyPenolakan` (alg `none`/HS256/ES256), `TestJWKSCacheKidBaru` |
| GA-04 | **TERVERIFIKASI** | `TestLoginGoogleMenolakBuktiIdentitasTidakSah/token_kedaluwarsa_setelah_jam_dimajukan`, `TestVerifyLeeway` |
| GA-05 | **TERVERIFIKASI** | `TestLoginGoogleNonceTidakCocokTidakMenautkanAkun`, `TestLoginGoogleMenolakBuktiIdentitasTidakSah/email_belum_diverifikasi`, `TestVerifyAzpDanNbf` (kasus `azp`/`nbf`) |
| GA-06 | **TERVERIFIKASI** | `TestLoginGoogleAkunUndanganMenjadiAktif` (status menjadi `active`, identitas tertaut, role dari database) |
| GA-07 | **TERVERIFIKASI** | `TestLoginGoogleAkunSuspendedDitolak` (403 generik, tanpa sesi, audit `auth.login.denied` `reason='suspended'`) |
| GA-08 | **TERVERIFIKASI** | `TestLoginGoogleEmailTidakDiundangTidakMembuatAkun`, `TestBootstrapAdminHanyaSekali` |
| GA-09 | **TERVERIFIKASI** | `TestPemakaianUlangPenyegarMencabutSeluruhSesi` (seluruh sesi, bukan satu), `TestPemakaianUlangPenyegarMencabutSesi` (audit `auth.session.reuse_detected`) |
| GA-10 | **TERVERIFIKASI** | `TestLogoutIdempoten`, `TestAdminMencabutSesiPenggunaLain` |
| GA-11 | **TERVERIFIKASI** | `TestPenonaktifanAkunMencabutSesiAktif`, `TestPerubahanRoleBerlakuPadaTokenLama` |
| GA-12 | **TERVERIFIKASI** | `TestPerubahanAkunSendiriDilindungi`, `TestSetRoleMenolakAkunSendiriDanAdminTerakhir`, `TestSetStatusMenolakAkunSendiriDanAdminTerakhir` |
| GA-13 | **TERVERIFIKASI** | `TestLinkInvitedAccountBersamaan` (penautan paralel), `TestNonceParalelHanyaSatuYangLolos` (satu nonce → satu sesi) |
| GA-14 | **TERVERIFIKASI** | `TestPerubahanEmailGoogleTidakMengalihkanAkun`, `TestPerubahanEmailGoogleTidakMengalihkanAkunSaatSudahTertaut` |
| GA-15 | **TERVERIFIKASI** | `TestLinkInvitedAccountMenolakSubjectMilikAkunLain`, `TestLinkInvitedAccountMenolakPenautanKedua` |
| GA-16 | **TERVERIFIKASI** | `TestNonceKedaluwarsaDitolak`, `TestNonceParalelHanyaSatuYangLolos`, `TestNonceTersimpanSebagaiHash`, `TestLoginGoogleNonceSekaliPakaiMenolakPengulangan` |
| GA-17 | **TERVERIFIKASI** | `TestBatasAbsolutTigaPuluhHariMenghentikanPenyegaran`, `TestBatasAbsolutTidakDiperpanjang` |
| GA-18 | **TERVERIFIKASI sebagian** | `TestLogDanAuditTidakMembocorkanRahasia` (audit bebas token/email/nonce, event D-036 yang relevan tercatat), `TestRecordMenolakMetadataTerlarang`, `TestRecovererMenyensorIsiPanic`. **Belum diuji:** `auth.device.mismatch` karena jalur perangkat belum ada (TA-24) |
| TA-16 | **TERVERIFIKASI sebagian** | `TestDosenHanyaMelihatKelasYangDitugaskan` (cakupan dosen diuji pada jalur yang sudah ada). **Belum diuji:** `GET /classes` dan sesi absensi — endpointnya belum ada |
| TA-17 | **TERVERIFIKASI sebagian** | `TestMahasiswaDitolakMembacaDaftarAkun`, `TestMahasiswaDitolakMenautkanAkunPeserta`. **Belum diuji:** pembacaan data peserta lain lewat `/people` — endpointnya belum ada |
| TA-18 | **TERVERIFIKASI sebagian** | `TestMeClassesTanpaPengaitanPeserta` (`GET /me/classes` → `409`). **Belum diuji:** `/me/schedule`, `/me/attendance`, `/me/consent` — ketiganya belum ada |
| TA-19 | **TERVERIFIKASI sebagian** | `TestDosenDitolakPadaEndpointAdministratif` (`GET`/`POST /users` → 403). **Belum diuji:** `/audit-events`, `/export.csv`, `/devices*` — endpointnya belum ada |
| TA-20 | **TERVERIFIKASI** | `TestPerubahanRoleBerlakuPadaTokenLama`, `TestLoginGoogleAkunUndanganMenjadiAktif` (role dibaca dari database, bukan dari klaim) |
| TA-21 | **TERVERIFIKASI** | `TestPerubahanAkunSendiriDilindungi` (admin terakhir → `LAST_ADMIN_PROTECTED`; dengan admin kedua → `SELF_ACCOUNT_PROTECTED`), `TestSetRoleMenolakAkunSendiriDanAdminTerakhir` |
| TA-22 | **TERVERIFIKASI** | `TestPemakaianUlangPenyegarGenerasiLamaTerdeteksi`, `TestPemakaianUlangPenyegarMencabutSeluruhSesi` |
| TA-23 | **TERVERIFIKASI sebagian** | `TestLogDanAuditTidakMembocorkanRahasia` memeriksa isi `metadata` audit (tanpa token/email/subjek), `TestDosenDitolakPadaEndpointAdministratif` memicu `authz.denied`. **Belum diuji:** `GET /audit-events` sebagai jalur pembacaan — endpointnya belum ada |
| TA-24 | **BELUM TERVERIFIKASI** | Bergantung pada token perangkat dan `auth.device.mismatch`; jalur perangkat belum diimplementasikan (di luar lingkup FL-011). Yang **sudah** dapat dipastikan dari kode: tidak ada tabel atau jalur perangkat yang menyentuh token sesi pengguna |
| TA-25 | **TERVERIFIKASI sebagian** | Dari sisi kode: `grep InsecureSkipVerify` pada seluruh backend **tanpa hasil**, dan alamat bawaan `127.0.0.1:8080` (`internal/config/config.go`). Smoke test FL-011 membuktikan klien **menolak** endpoint JWKS bersertifikat tidak tepercaya dan hanya menerima setelah CA lokal dipercaya. **Belum diuji:** TLS LAN nyata dan trust store Android (D-035, Q-25) — di luar lingkup FL-011 |
| TA-26 | **BELUM TERVERIFIKASI** | Bergantung pada aplikasi klien dan prosedur operasional (Q-24). Yang **sudah** terbukti dari sisi backend: tidak ada jalur operasi tanpa sesi sah, dan sesi yang dicabut/kedaluwarsa ditolak pada setiap permintaan terproteksi (`TestMeTanpaTokenDanTokenAsingDitolak`, `TestTokenAksesKedaluwarsaDitolak`, `TestPenonaktifanAkunMencabutSesiAktif`) |

**Rekapitulasi:** dari **29 skenario** pada §8 (GA-01..GA-18 dan TA-16..TA-26), **20 TERVERIFIKASI**, **7 TERVERIFIKASI sebagian** (GA-18, TA-16, TA-17, TA-18, TA-19, TA-23, TA-25), dan **2 BELUM TERVERIFIKASI** (TA-24, TA-26). Tidak ada skenario yang dinyatakan lulus hanya karena kodenya "terlihat benar": setiap baris yang berstatus TERVERIFIKASI menyebut nama uji yang benar-benar ada dan dijalankan.

**Batas yang harus dibaca bersama tabel di atas:**

1. **Endpoint yang belum ada tidak diuji.** `/people`, `/classes`, `/sessions`, `/devices`, `/audit-events`, `/me/schedule`, `/me/attendance`, dan `/me/consent` masih kontrak saja (lihat `docs/API_CONTRACT.md` §4 status implementasi). Skenario yang bergantung padanya hanya dapat diuji sebagian.
2. **Tidak ada akun Google nyata yang dipakai.** Seluruh uji OIDC memakai issuer, JWKS, dan token simulasi terkontrol (`internal/oidc/verify_test.go`, `internal/oidc/jwks_test.go`, `internal/httpapi/authflow_test.go`). Perilaku nyata endpoint Google (mis. kapan `azp` dikirim) **tidak** dapat disimpulkan dari uji ini.
3. **Pembatasan laju belum ada.** Kode `429 RATE_LIMITED` tidak pernah dihasilkan; ini dicatat sebagai kesenjangan pada `docs/API_CONTRACT.md` §10 dan bukan bagian dari skenario §8.
4. **Smoke test bukan pengganti uji unit.** Smoke FL-011 menjalankan biner sungguhan dengan penyedia identitas tiruan di atas HTTPS ber-CA lokal, dan membuktikan jalur HTTP end-to-end; hasilnya dicatat pada `PROJECT_STATE.md` beserta tanggalnya.

## 9. Bukti FL-CORE02 — sesi dan fallback manual (25 Oktober 2026)

Seluruh baris berikut dijalankan pada database disposable dan autentikasi sintetis,
bukan akun testing Product Owner. Face Recognition tetap mekanisme utama produk;
tidak ada engine/camera/enrollment/template yang diaktifkan pada validasi ini.

| Acceptance FL-CORE02 | Bukti konkret yang lulus |
|---|---|
| 1. Lifecycle valid | `TestCore02LifecycleThresholdsAndManualAlternative`: draft/open/closed dan waktu server; HTTP `TestCORE02HTTPManualLifecycleReplayAndPrivacy` |
| 2. Concurrent open | `TestCore02IndependentHandlesProtectOpenClassAndDevice`: dua handle, tepat satu sesi open dan satu audit; kelas dan perangkat diuji |
| 3. Close dan retry | `TestCore02ConcurrentCloseCommitsOneTransitionAndAudit`: satu sukses/satu ErrSessionClosed, satu audit sesuai aktor/alasan pemenang; retry HTTP `409 SESSION_CLOSED` |
| 4. Roster member sah | `TestCore02LifecycleThresholdsAndManualAlternative`: insert manual sah dan batas tepat late_after |
| 5. Nonmember | `TestCore02ManualSubjectEligibilityAndValidation` dan `TestCORE02HTTPTimeThresholdsAndStrictParsing`: `422 CLASS_MISMATCH` |
| 6. Duplikasi | `TestCore02IndependentHandlesManualAndIdempotencyRaces`: satu record/audit; replay respons identik; `TestCore02DuplicateAcrossLateThresholdPreservesFirstTimestamp`: retry tetap `409`, bukan mengubah waktu/status |
| 7. Closed check-in | `TestCore02ClosedExplicitCorrectionAndNoOp` dan `TestCore02ElapsedSessionBlocksImplicitWithoutAutoClosing`: check-in biasa ditolak setelah close/deadline |
| 8. Koreksi beralasan | `TestCore02ClosedExplicitCorrectionAndNoOp`: explicit historical correction sah, no-op identik tanpa audit ganda, alasan lama tetap di audit |
| 9. Unauthorized correction | `TestCORE02HTTPRolesScopeAndCacheAuthorization`: mahasiswa `403`, dosen kelas lain `404` |
| 10. Audit rollback | `TestCore02AuditFailureRollsBackEveryMutation`: trigger audit gagal, create/open/close/insert/correction dan snapshot ikut rollback |
| 11. Dosen out-of-scope | `TestCore02ActorScopeAndReplayReauthorization` serta HTTP replay reauthorization: penugasan dicabut menutup replay dan pembacaan |
| 12. Mahasiswa bukan penulis | HTTP roles test dan smoke HTTPS: POST manual-checkins mahasiswa `403`, tidak menambah attendance |
| 13. Tanpa biometric consent | Lifecycle dan availability test: manual berhasil dengan nol consent; peserta tanpa account binding juga teruji |
| 14. Biometrik unavailable | `TestCORE02ManualRemainsAvailableWithoutBiometrics`: readiness manual ready/biometrics unavailable, manual berhasil, nol nonmanual write, recognition route tidak aktif |
| 15. Upgrade disposable | `TestCORE02UpgradePreservesLedgerLegacyAttendanceAndForeignKeys` dan `TestCORE02FailedUpgradeRollsBackSchemaDataAndLedger`: schema 5→6 dengan data lama, uniqueness/FK/ledger utuh atau rollback |
| 16. Go tests | `go test -count=1 ./...`: 14 paket uji lulus pada run final |
| 17. Go vet | `go vet ./...`: exit 0 pada run final |
| 18. HTTP integration | `internal/httpapi/core02_test.go` dan `core02_availability_test.go`: pagination/filter/ownership, strict input, ETag/304 setelah scope, status pending→absent |

Smoke program sementara `go run ./cmd/core02-smoke` berjalan melalui socket HTTPS
dengan sertifikat uji tepercaya: readiness `200`, create session dan replay
`201/201` dengan UUID sama, open `200`, manual dan replay `201/201` dengan waktu
sama, duplicate tanpa key `409`, self history `200`, mahasiswa POST `403`, close
`200` lalu `409`, check-in biasa closed `409`, koreksi eksplisit closed `200`.
Database schema 6 berakhir dengan satu attendance dan dua audit mutasi manual,
nol consent/biometric writes. Program dan database sementara dibersihkan.

Run awal menemukan dua kegagalan regresi: fixture CORE01 mengandalkan default
session `open` lama, dan pola kebocoran menganggap kata `biometric` sendiri
sebagai embedding. Fixture kini menyatakan sesi open secara eksplisit;
assertion wording itu dihapus, pemeriksaan rahasia/path/database/stack tetap.
Run final di atas lulus seluruh paket. Review independen juga menemukan TTL
cleanup hanya same-key dan duplicate melewati late_after menjadi `422`;
keduanya diperbaiki, regresi tersedia dan lulus, serta actual-code direview ulang.

Race detector: **NOT_RUN**, Go 1.27.0 windows/amd64, `CGO_ENABLED=0`.
Tes goroutine dengan dua handle SQLite bukan pengganti race detector.
Tidak ada uji OTP/Android/KeyProvider/recognition production pada tugas ini.

## 10. Bukti FL-ENR01B — backend enrollment sintetis (26 Oktober 2026)

Seluruh bukti memakai database SQLite baru, direktori sementara, kunci DPAPI
CurrentUser yang baru dibuat, dan JPEG pola sintetis. Tidak ada ADB, foto manusia,
database Product Owner, akun testing, SMTP/DNS, OTP/session production, atau
model YuNet/SFace nyata yang dipakai.

| Acceptance FL-ENR01B | Bukti konkret |
|---|---|
| Consent self-service + notice | `TestSyntheticEnrollmentLifecycleAndEncryptedPayloads`, `TestConsentCASIdempotencyAndFreshGrant`; HTTP tampered/unknown field 400 |
| Withdrawal + purge | Lifecycle dan `TestWithdrawalAuditFailureRollsBackAllPayloadDeletion`; HTTP withdraw 200 lalu status REVOKED, template 0 |
| RBAC/ownership | `TestEnrollmentHTTPGuardsPreviewApprovalAndWithdrawal`: Dosen/Mahasiswa preview+approve 403; Admin review; role fence `TestRoleFencePreservesActiveTemplateButReadinessFalse` |
| Idempotency/CAS | Submit replay 200 ID sama; changed JPEG `IDEMPOTENCY_CONFLICT`; stale/missing If-Match 412/428; weak/canonical guard test |
| Double live attempt/race | `TestConcurrentSubmitOnlyOneLiveAttempt`, `TestDecisionRaceIsSingleImmutableCAS` (satu pemenang, nol duplikat) |
| TTL 72 jam | `TestExpiryIs72HoursAndNeverExtendedByReplay`; replay 23 jam tidak memperpanjang; HTTP expiry tanpa scheduler |
| Key/integrity failure | `TestKeyFailurePreservesCiphertextAndWithdrawalNeedsNoKey`, `TestRecoveredRetainedPhotoRetryIsFencedAndBounded`; key-loss tidak menghapus, retry terbatas 3 |
| Extraction failures | `TestExtractionErrorsNeverActivateAndConservativelyPurge`: no-face/multiple/NaN/zero/norm/manifest tidak ACTIVE, foto biasa dipurge |
| Reviewer/subject fences | `TestReviewerDemotionRestorationPermanentlyFencesApproval`, `TestAccountRosterAndLinkRestorationDoNotResurrectWork`, `TestPhotoExpiryDuringProcessorFencesLateActivation` |
| Restart/reconciliation | `TestEncryptedFileCrashRecoveryAndOrphanCleanup`, `TestAbandonedRunningJobFailsOnRestart`, `TestRestartImmediatelyAfterBothPublicationTransitions` |
| Audit atomicity | `TestAuditFailuresRollBackGrantPreviewDecisionAndActivation`; HTTP preview audit failure tidak mengirim JPEG |
| JPEG safety | `TestNormalizeJPEGStripsMetadata`, `TestJPEGRejectsMalformedUnsafeAndOversize`, EXIF pointer/orientation tests, batas 2073600 pixel |
| Crypto/storage | `TestEnvelopeAuthenticatesEveryTrustedField`, `TestTemplateAADPreservesD022Tuple`, DPAPI provisioning no-replacement, DACL/hardlink/root-replacement regressions |
| Migration isolation | Default `migrations/` tetap schema 6 dan tanpa tabel biometrik; `TestENR01BUpgradeIsAdditiveAndPreservesPriorChecksums` dan rollback memakai salinan disposable schema 7 |
| Reactivation after restart | `Reconcile` memulihkan staging/orphan dan menandai worker yatim; bukan bukti power-loss durability |
| Manual fallback tanpa kunci | Smoke HTTPS: setelah kunci template diganti nama, status sintetis `template_ready=false`, satu template tetap ada, create/open/manual session 201/200/201 |
| HTTP no-store/no-sniff | Preview test: `no-store`, `nosniff`, exact length, Range 400; log tidak memuat token/JPEG/key/embedding |

Run final: `gofmt` seluruh paket yang disentuh, `go test -count=1 ./...` **16
paket uji lulus**, `go vet ./...` exit 0. `go run ./cmd/enrollment-synthetic`
mencetak schema 7, notice/grant/submit/replay/approve/active/withdraw sesuai,
preview `403` untuk Dosen dan Mahasiswa, key-loss fail-closed, manual tanpa kunci
`201`, lalu nol template dan nol ciphertext setelah withdrawal+reconcile.

Kegagalan nyata yang ditemukan dan diperbaiki: Windows root replacement sempat
diterima; crash antara create/protect `.tmp` dan link/unlink publication dapat
meninggalkan state yang ditolak recovery; migrasi schema7 sempat berada di
discovery startup normal. Ketiganya kini memiliki regresi. Migrasi dipindah ke
`synthetic-migrations/` dan hanya digabung oleh fixture disposable.

Batas: race detector **NOT_RUN** (CGO_ENABLED=0), pengujian Linux untuk
`renameat2` **NOT_RUN** (tidak ada runtime Linux; hanya build/vet lintas platform
bila dijalankan), tidak ada uji power-loss, preview yang sudah diotorisasi dan
sedang ditulis tidak dapat ditarik kembali, dan penghapusan tidak menjangkau
backup eksternal/SSD/WAL secara forensik. Timer recovery D-030 tetap BLOCKED.

## 11. Bukti FL-ENR02A — UI mahasiswa dan integrasi sintetis (26 Oktober 2026)

Scope: aplikasi staging turunan FaceLink sebenarnya, package `ai.facelink.enr02host`,
database/akun/roster/kunci disposable; tidak memakai database Product Owner,
ADB, manusia atau kamera. Laporan A–L: `FL_ENR02A_TECHNICAL_REPORT.md`.

| Acceptance | Bukti dan batas |
|---|---|
| Consent terpisah dari enrollment | `enrollmentFlow.test.ts`: explicit grant, template/consent/status guard, renewal menyimpan terminal submission lalu dapat menuju scan |
| Guidance dan single submission | Harness clock alignment/hold/capture; cancel terminal; fixture dikirim sekali dan referensi buffer dikosongkan |
| Status pending/rejected/expired/failed | State mapping tanpa trust ACTIVE; TTL tidak diciptakan client |
| Snapshot ACTIVE stale | Background, umur30detik, failed full reload dan network failure menghilangkan trust |
| Withdrawal | State withdrawn; regresi respons hilang setelah commit merekonsiliasi REVOKED tanpa mempertahankan ACTIVE |
| Lifecycle async | Cancel saat menunggu token tidak mengirim JPEG; token null keluar dari uploading; resume initial load membaca consent lagi |
| API | `enrollmentApi.test.ts`: ETag/If-Match, replacement ID, UUID, body JPEG, error parsing, menolak marker nonsintetis |
| Navigation/roles | Existing role-route tests ikut lulus; main flag default-off dan transform staging diuji; UI perangkat NOT_RUN |
| Backend actual | E2E opt-in server Go nyata + client TS nyata: grant → JPEG → PENDING → Admin approval202 → ACTIVE/template_ready → withdrawal/REVOKED |
| Native packaging | Gradle ARM64 debug lulus; aapt package terpisah dan tanpa CAMERA/media/storage/overlay |

Run akhir main dan staging: `npm run typecheck && npm run lint && npm test`,
masing-masing **139 passed,1 skipped**. E2E dipisah:
`FACELINK_ENR_E2E=1 npx vitest run tests/enrollmentE2E.test.ts`: **1/1 passed**.
Export `npx expo export --platform android --output-dir artifacts/export-final`
lulus1491 modules. Gradle `app:assembleDebug -PreactNativeArchitectures=arm64-v8a
--offline --console=plain` **BUILD SUCCESSFUL4m8s**.
Backend `go test -count=1 ./...`16 paket uji lulus/5 tanpa tes,
`go vet ./...` lulus, `go run ./cmd/enrollment-synthetic` smoke TLS nyata lulus,
key-loss fail-closed, manual tanpa kunci201, akhir0 template/0 ciphertext.

Review independen source dan generated config tidak menyisakan temuan actionable
yang dilaporkan setelah perbaikan. Tes UI di atas adalah state/kontrak, bukan
mounted visual interaction. **NOT_RUN:** layar Android, accessibility aktual,
OTP staging di HP, UI→backend device E2E, runtime TLS dan isolasi SecureStore
antar-app. Izin perangkat baru diperlukan sebelum ADB/install.
Gate **PARTIAL** untuk acceptance UI; **NO-GO** FL-ENR02B otomatis,
capture manusia, production atau migrasi database Product Owner.

Cleanup harness: EOF stdin menghentikan server test-token nyata; readiness
failure menutup child sebelum rethrow. Smoke sertifikat hilang teramati gagal
sesuai harapan dengan0 root disposable tertinggal; sesudah E2E0 server fixture.
Review independen menutup temuan orphan `go run` Windows dan bypass defer
pada fatal startup. Ini bukan pengujian crash/power-loss atau forensic cleanup.

## 12. Bukti FL-ENR02A-R1 — Android fisik sintetis (26 Oktober 2026)

**PARTIAL keseluruhan; alur inti perangkat PASSED.** TECNO LJ7 fisik, izin baru
sebelum ADB/install, package staging `ai.facelink.enr02host`. Laporan A–M:
`FL_ENR02A_R1_ANDROID_ACCEPTANCE_REPORT.md`. Bukti §11 adalah hasil offline
historis; bagian ini menambahkan observasi perangkat, tidak mengganti asal bukti.

| Jenis bukti | Skenario dan hasil nyata |
|---|---|
| Android UI | Login OTP mahasiswa sintetis dibantu pemilik; consent/notice dan checkbox; tombol disabled sebelum acceptance |
| Android UI→HTTPS | Scan pola sintetis, satu pending; Admin reject200; fresh consent/retry menghasilkan submission baru; approval202→worker ACTIVE→Wajah terdaftar |
| Cancel/lifecycle | Back dan HOME saat scan tidak mengirim submission (queue0); resume preparation; force-stop/cold relaunch memulihkan sesi valid |
| Offline/error | TLS byte-relay staging dihentikan; UI Aktif (belum terverifikasi), stale warning dan retry; sesudah koneksi pulih fresh ACTIVE kembali |
| Withdrawal | Konfirmasi perangkat→Persetujuan ditarik; sesudah respons HTTPS REVOKED/template_ready=false/deletion COMPLETE |
| Expiry perangkat | Pending ketiga; clock fixture+73jam→backend EXPIRED; login ulang sah setelah sesi expired; HP menampilkan PHOTO_RETENTION_EXPIRED pada offset75jam |
| Expiry otomatis | Clock72jam+1detik; replay23jam tidak mengubah deadline; expired preview410/approve409 |
| RBAC/audit otomatis | Student/Dosen preview/approve/queue/detail403; self selector400; replay ID sama/count1; audit actor/entity/count dan protected-log assertions lulus |
| Preservation | Main APK SHA256 sama, lastUpdateTime tidak berubah,6 setting identik; staging installed tanpa CAMERA; tidak mengubah global permission |

Perintah acceptance otomatis yang dijalankan:
`FACELINK_R1_BACKEND=1 go test ./internal/httpapi -run '^TestR1HTTPSAcceptance$' -count=1 -v`.
Tiga subtest **PASSED**: GrantReplayRejectRenew, ApproveActiveWithdraw,
PendingExpiry72Hours. Test memakai real HTTPS dengan trusted CA dan verified
identity, bukan InsecureSkipVerify. Setup auth fixture otomatis bukan bukti OTP;
OTP dibuktikan terpisah oleh login perangkat dan Admin melalui endpoint normal.

Clock kontrol hanya pada harness disposable, tanpa endpoint baru atau perubahan
retensi/session timestamp SQL. Offset maju74/75jam untuk cooldown OTP yang
terbekukan; jam HP tidak diubah. Injeksi clock kemudian dikembalikan ke clock.Real.
Dua harness sementara diarsipkan di evidence ignored, bukan source aktif.

Regresi akhir setelah restorasi: main `npm run typecheck && npm run lint &&
npm test` **139 passed,1 skipped**; backend `go test -count=1 ./...`16 paket uji
lulus/5 tanpa tes, `go vet ./...` lulus; smoke `go run ./cmd/enrollment-synthetic`
HTTPS lulus dan akhir0 template/0 ciphertext.

Runtime perangkat sebelum cleanup: submission akhir REJECTED1/REVOKED1/EXPIRED1,
template0/photo0; audit submitted3/approved1/activated1/rejected1/revoked1/expired1.
Screenshot dan metadata lokal: `research/fl-enr02a-host/evidence-local/r1/`.
Log sample tidak memuat marker JPEG/base64 payload atau token; bukan audit
forensik/zero-telemetry semua SDK.

**Residual:** LogBox expo-video TextureVideoView/player already released muncul
non-fatal pada lifecycle, telah tercatat sebelum R1 di ANDROID_SETUP §17d.
Tidak ada fix dugaan/suppression atau perubahan video/main. Accessibility
semantik dasar diamati; TalkBack/varian font/endurance/release-build R1 NOT_RUN.

**Cleanup:** staging force-stopped dan tetap installed, HTTPS/Metro/relay
berhenti, reverse kosong, XML UI sementara di HP dan root disposable task dihapus;
main/PO DB/APK historis/video/evidence dipertahankan.
NO-GO capture manusia/production/FL-ENR02B otomatis; STOP setelah laporan.

## 13. Bukti FL-ENR02A-R2 — video lifecycle dan accessibility (26 Oktober 2026)

**PARTIAL; GO terbatas synthetic-only.** Laporan A–H:
`FL_ENR02A_R2_VIDEO_ACCESSIBILITY_REPORT.md`.

| Bukti | Hasil |
|---|---|
| Pra-fix device | Restart dengan sesi tersimpan + TalkBack menampilkan `TextureVideoView`/`shared object already released`; non-fatal |
| Post-fix device | Urutan restart sama mendarat di dashboard tanpa warning; tidak ada FATAL |
| Fix | Video login hanya dipasang saat status bukan `memulihkan`/`signedIn`/`unregistered` |
| Accessibility | Target Back/retry48dp, heading, label status, checkbox hint/state, label progress, announcement phase, copy offline tegas |
| Font scale |1.0/2.0 pada intro/consent; teks membungkus, konten dapat discroll, tombol tidak terpotong |
| Kontras | Utama gelap ≥4.8:1; residual putih pada tombol merah≈3.90:1 dan TalkBack penuh belum PASS |
| Regresi | Main/staging143 lulus/1 skipped, typecheck/lint lulus; export1491 modules; Gradle debug ARM64 BUILD SUCCESSFUL; backend16 paket/vet lulus |
| Cleanup | Font1.0, TalkBack off/`null`/touch0, rotation tetap, reverse kosong, staging force-stopped/installed, main hash sama |

TalkBack diaktifkan dan fokus visual/TTS teramati, tetapi onboarding sistem dan
panjang notice membuat traversal ucapan menyeluruh **NOT_RUN**. Ini bukan klaim
accessibility PASS. Tidak ada kamera, human capture, production, atau migrasi
database Product Owner.

## 14. Bukti FL-ENR02B — native camera offline (26 Oktober 2026)

**PARTIAL; live camera/mannequin NOT_RUN.** Laporan A–L:
`FL_ENR02B_TECHNICAL_REPORT.md`.

| Bukti | Hasil |
|---|---|
| Native bridge | `FaceLinkCaptureLive` CAM00 + `Cam02FrameStore`/adapter/encoder; metadata-only JS |
| Detector | patch 2.1.0 CAM02 idempotent; host live compile bergantung marker |
| Modes | synthetic default tanpa CAMERA; mannequin staging dengan CAMERA/FLAG_SECURE, explicit start |
| Privacy | no JPEG/base64/pixel/path ke JS; opaque consume token private; no live upload |
| Tests | main/staging147 lulus/1 skipped, typecheck/lint; CAM00 JVM35+224, JS21+10 |
| Android | export1622 modules; mannequin debug BUILD SUCCESSFUL5m3s; synthetic debug BUILD SUCCESSFUL3m34s; mannequin CAMERA, synthetic no-CAMERA; both no storage/media/overlay |
| Security | review menemukan lalu menutup FLAG_SECURE, install/patch, mode isolation, availability, status, ownership |
| Device | ADB/kamera/mannequin/permission/background/TalkBack **NOT_RUN** |

Tidak ada klaim runtime live, recognition, PAD, production, human capture, atau
migrasi database Product Owner.

## 15. Bukti FL-ENR02B-R1 — Android live mannequin acceptance (26 Oktober 2026)

**PARTIAL; NO-GO menuju Admin Enrollment UI integration.**
Laporan A–M: `FL_ENR02B_R1_ANDROID_ACCEPTANCE_REPORT.md`.

| Kategori | Hasil dan batas |
|---|---|
| Gate | Offline package/hash/manifest sebelum ADB; izin eksplisit baru; fixture mannequin dikonfirmasi pemilik |
| Startup | Installed staging terpisah; OTP normal; sink attach error saat reload diperbaiki dengan identity-checked detach pada host destruction; cold/hot restart teramati |
| Preview LIVE | Explicit start setelah consent dan permission; preview normal menurut pemilik; FLAG_SECURE tanpa screenshot preview |
| Guidance LIVE | NO_FACE/0 dan TOO_FAR/1 teramati; frame callback berjalan; tidak capture sebelum syarat terpenuhi |
| Capture/consume LIVE | **NOT_PROVEN**; trace CAPTURE/READY/CONSUME kosong; positive dwell, same-frame, duplicate/stale consume dan cleanup setelah codec **NOT_RUN** |
| Lifecycle LIVE | Permission denied/one-time grant; cancel/Back; background release setelah1500ms; resume tidak auto-start |
| Privacy | Inventory nama/ukuran staging tanpa image-named paths; backend disposable0 submission/foto/template; bukan proof no-temporary-write/zero-telemetry |
| Unit | Kotlin35+224 assertions, JS21+10 PASS; bukan live codec proof |
| Regresi | Main/staging TypeScript/lint dan147 tes/1 skipped; native rebuild; E2E sempat timeout saat build berjalan, satu run tanpa build PASS; rincian hasil akhir di laporan |
| Cleanup | Staging force-stopped/installed, reverse kosong, layanan berhenti/root fixture dihapus; main APK hash identik; diagnostik sementara dibuang |
| Settings | Tidak ada settings put; setelah jeda brightness4→1 dan timeout15000→1800000 teramati dan dipertahankan; tidak mengklaim seluruh baseline setting identik |

TalkBack/font scale, Settings revocation, dua fixture nonmanusia, positive oval/
mirror/crop dan background codec belum dibuktikan. Tidak ada manusia, live
upload, production, migrasi Product Owner, atau commit/push/merge.

## 16. Bukti FL-ENR02B-R2 — readiness dan same-frame capture (26 Oktober 2026)

**Happy path LIVE DEVICE PASS; keseluruhan PARTIAL.**
Laporan A–M: `FL_ENR02B_R2_READINESS_CAPTURE_REPORT.md`.

| Kategori | Hasil dan batas |
|---|---|
| Gate | Izin R2 sebelum semua ADB; APK staging sudah identik, tidak install ulang; fixture gambar mannequin nonmanusia di browser dikonfirmasi sebelum start |
| Diagnosis | Gate tetap dp; sensor bounds bukan ML Kit dimensions; unit trace dikoreksi sebelum device run; tidak ada perubahan threshold/mapping |
| Readiness LIVE | Generation1793029361952/frame1512, dwell1504.65792ms/32 sampel; ukuran/containment/pose gate tetap; prefix event awal tidak seluruhnya tersimpan |
| Same-frame LIVE | Frame1512 sama pada CAPTURE_READY, CAPTURE, JPEG_READY146991 byte, CONSUMED dan RELEASED |
| Native replay | Stale generation−1→E_CAPTURE_GENERATION; duplicate sesudah consume→E_CAPTURE_HANDLE; bukan live concurrent-race proof |
| Native memory | READY owned4194304/held0/busytrue; sesudah consume owned0/held0/busyfalse, captures1/consumes1/releases1; bukan total memory-zeroization claim |
| Geometry | Preview323.70367×473.70367dp; round-trip0.00003dp; native rotation270/mirroredtrue/buffer1280×960; inspeksi visual JPEG crop NOT_RUN |
| Lifecycle | Close sesudah capture; Back saat start; HOME sesudah FRAME1513; DRAINED nol; camera client kosong setelah settle1500ms; resume/cold restart tidak auto-start |
| Residual lifecycle | Background/cancel tepat ketika ENCODING, revocation dan TalkBack scanner NOT_RUN pada R2 |
| Privacy | Sandbox32 berkas awal/akhir tanpa image paths; disposable0 enrollment/submission/foto/template; UI Belum terdaftar, bukan ACTIVE; tanpa JPEG di JS/log/evidence |
| Offline | Main/staging147 PASS/1 skipped;5 trace tests; Kotlin35+224 assertions; JS31; final mannequin dan synthetic TypeScript/lint PASS |
| Build | Android ARM64 offline max-workers1 BUILD SUCCESSFUL7m7s; build pertama gagal baca PromiseType.hpp, penyebab sementara belum diisolasi |
| Preservation | Main APK SHA256 identik;10 settings identik; tidak ada settings put/restore; tidak ada native/auth/schema/main-source change |
| Cleanup | Staging force-stopped/installed; reverse kosong; port8089/8090/8091/8769 kosong;0 fixture backend; root disposable hilang; stage synthetic_flow/TRACE0/PROBES0 dan regresi ulang lulus |

Tidak ada human capture, live upload, production enrollment/template, database
Product Owner, ESP-CAM, SMTP/DNS atau commit/push/merge. Tidak memulai
FL-ENR03/Admin UI implementation otomatis. **STOP setelah laporan R2.**

## 17. Bukti FL-ENR03 — Admin enrollment staging sintetis (27 Oktober 2026)

| Area | Hasil nyata |
|---|---|
| Backend | `go test -count=1 ./...` 17 paket `ok`, 4 tanpa tes; `go vet ./...` exit0; replay receipt setelah subject menjadi ineligible, filter status kosong, dan segmen statik Windows diuji |
| Mobile unit | `npm run typecheck`, `npm run lint`, `npm test` lulus; 14 file lulus/2 skipped, **206 tes lulus/6 skipped** |
| E2E HTTPS | `FACELINK_ADMIN_ENR_E2E=1`, `NODE_EXTRA_CA_CERTS=backend/dev/certs/dev-ca.pem`, proses Go nyata + SQLite disposable: **5/5 lulus** |
| E2E scenarios | Queue/detail/preview; approve202→worker ACTIVE→withdraw; dua reviewer/CAS replay; retry setelah failure; Dosen/Mahasiswa 403; role/status Admin dicabut; expiry72jam |
| Frontend contracts | DTO malformed/wrong-target ditolak; 202 ACTIVE palsu ditolak; token tidak pernah di URL; preview ≤1MiB, `image/jpeg`, `no-store`, abort setelah chunk; replay memakai key/ETag/payload sama |
| Lifecycle | Rotasi bearer normal, polling tidak membatalkan read, blur/background menyimpan operasi ambigu, preview tidak membuka ulang otomatis; unit/controller membuktikan; runtime perangkat NOT_RUN |
| Staging | `prepare-enr02a.cjs` default-OFF menghapus route dan menulis flag0; flag eksplisit mengembalikan route/flag1; stage typecheck/lint lulus; web export1093 modul |
| Build | Gradle ARM64 `assembleDebug --offline --max-workers=1` BUILD SUCCESSFUL11m48s; APK102262250B SHA256`07bd378d…afe32`, package`ai.facelink.enr02host`, tanpa CAMERA/media/storage/overlay |
| Browser smoke | Login staging dan `/admin-enrollment` tanpa sesi termuat; route menolak menampilkan review. Login OTP web berhenti pada SecureStore yang tidak tersedia (perilaku existing) |
| NOT_RUN | UI terautentikasi di web; ADB/install/launch Android; TalkBack/font scaling; dua perangkat; cold restart; cache/zeroization renderer |

Tidak ada endpoint/migrasi baru, foto manusia, kamera, database Product Owner,
perubahan OTP/session/role, atau commit/push/merge. **STOP; NO-GO production.**

## 18. Bukti FL-ENR04 — integrasi CV lokal sintetis/disposable

- Python: `services/face-engine/.venv/Scripts/python.exe -m pytest
  services/face-engine/tests` dari root: **208 passed, 1 skipped**; skip existing
  memerlukan deteksi positif yang tidak ada pada gambar acak.
- Backend: dari `backend/`, `go test -count=1 ./... && go vet ./...`:
  **18 paket ok, 4 tanpa tes, exit0**. Tes IPC memakai child helper terisolasi
  untuk auth/schema/binding/bounds/crash/cancellation/deadline/singleflight.
- Mobile dan stage masing-masing: typecheck/lint lulus, **219 passed/6 skipped**.
  Opt-in HTTPS Admin: `FACELINK_ADMIN_ENR_E2E=1` dan
  `NODE_EXTRA_CA_CERTS=backend/dev/certs/dev-ca.pem`: **5/5**.
  `FACELINK_ENR_E2E=1`: mahasiswa HTTP disposable **1/1**.
- Smoke executable Go + Python + model pinned: approval202 →
  FAILED_PROCESSING/NO_FACE; template0, photo READY0, processing.failed1;
  shutdown menghapus root disposable. Tes langsung IPC juga membuktikan request
  tanpa HMAC benar/oversize ditolak dan missing/corrupt model gagal tertutup.
- Angka resource adalah satu smoke, bukan SLA: model init122,21ms,
  JPEG+YuNet negative140,99ms, peak working set126,59MiB pada proses Python
  aktual, OpenCV threads1. Pengukuran launcher venv tidak dipakai.
- Run awal Go menemukan empat tes baru mencoba mengubah binding yang memang
  immutable; tes diperbaiki untuk membuktikan penolakan DB dan legitimate
  activation tetap berjalan, tanpa mengubah/mematikan trigger produk.
- **REAL_MODEL_POSITIVE_NOT_PROVEN**. Tes SFace real dengan landmark buatan
  adalah bukti komponen saja. Tidak ada manusia/kamera/ADB/Android build,
  FAR/FRR, kalibrasi, PAD/liveness, atau production acceptance.
- Review keamanan dan lifecycle independen tidak menemukan defect actionable
  pada trust boundary yang ditinjau; bukan jaminan semua race native hilang.

Laporan lengkap A–O: `FL_ENR04_TECHNICAL_REPORT.md`. **STOP sebelum FL-ENR05.**

## 19. Bukti FL-ENR04-R1 — bounded synthetic positive screening

| Area | Hasil yang dijalankan |
|---|---|
|Fixture|6mannequin height-field2,5D lokal, CC0 fixture-only, hash reproducible6/6; ignored evidence-local, tanpa asset manusia|
|YuNet asli|Semua6 NO_FACE, threshold0,75 unchanged; direct pipeline dan HMAC IPC|
|SFace/alignment real positive|NOT_RUN; tidak ada deteksi/landmark sah|
|Go real-model strict positive|Configured candidate01 **FAIL/NO_FACE**; pending-photo encryption/HTTP202/actual IPC/reconcile berjalan, template0/active0/photo0/files0|
|Template real positive dan withdrawal sesudahnya|NOT_PROVEN/NOT_RUN|
|Python|213passed,1skipped; invalid query/template/dimension/NaN/Inf/model version/checksum matching guards|
|Go+vet|18paket ok,4tanpa tes, exit0; opt-inpositive test absent config SKIP; partial config FAIL sebelum pipeline|
|Failure/concurrency|Withdrawal/timeout/crash/model unavailable/invalid vector/AAD/expiry/lease/reviewer/restart, plus failed template encryption retained-safe dan duplicate worker single activation|
|E2E|HTTPS Admin5 + HTTP mahasiswa1,6/6passed; no mobile shared/runtime contract change|
|Evidence hardening|Screening HMAC/version/binding checks tidak memakai removable assert; replay `-O` dan tampered HMAC rejection terbukti|
|Review|Dua independent read-only review tanpa actionable security findings; keterbatasan evidence optimasi ditutup|

Artefak final: `research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified/screening.json`.
Run awal dan evidence negatif Go dipertahankan. Hasil default-suite PASS tidak
menggantikan configured-positive FAIL. Tidak ada Android/ADB/kamera, manusia,
production, threshold manipulation atau D-030 policy change.
Laporan A–N dan perintah reproduksi: `FL_ENR04_R1_TECHNICAL_REPORT.md`.
**PARTIAL — REAL_MODEL_POSITIVE_NOT_PROVEN; STOP sebelum FL-ENR05.**

## 20. Bukti FL-ENR04-R2 — native positive dan batas AAD v1

Hasil R1 di atas adalah snapshot sebelum perbaikan R2. Tidak ada kandidat unik
baru; fixture01 bytes/hash existing dipakai ulang. Threshold0,75, quality gate,
dua model dan dependency tetap. Root cause: upscale512→960 menghilangkan deteksi;
native512 menghasilkan satu wajah. Frame≤960 kini tetap native, fingerprint
Go/Python `enr04-synthetic-native-v2`.

| Area | Hasil yang dijalankan |
|---|---|
| Regresi ukuran square/rectangular/non-multiple32 dan real extraction |4FAIL sebelum→4PASS setelah|
| Python lengkap |217PASS,1SKIP|
| Go lengkap dan vet |18paket ok,4tanpa tes; vet exit0|
| Real-model lifecycle HTTPS disposable |18request CA/hostname verified, actual YuNet→SFace→IPC→AES-256-GCM→ACTIVE→withdraw→REVOKED; payload/files0|
| Template |128float32 finite/unit, ciphertext528byte; metadata/fingerprint/ownership sesuai; purpose-key/nonce separation dan tamper rejection|
| AAD same-person/model/version snapshot substitution |2/2diterima dengan payload record lain; ordinary UPDATE ditolak;4kontrol lain ditolak|
| Mobile typecheck/lint/tests |Lulus;219PASS,6SKIP|
| Existing client E2E |Admin5 + mahasiswa1:6/6PASS|
| Independent review |Tidak ada defect preprocessing tambahan; satu P2 AAD record-binding terkonfirmasi|

AAD diagnostic memakai dua SQLite snapshot offline, FK/lifecycle INSERT guard
tidak diuji; bukan remote endpoint exploit atau mutation path legal. Temporary
test dihapus setelah hasil aman disimpan. Kontrak AAD/ciphertext tidak diubah.
**Positive functional PASS; keseluruhan NO-GO FL-ENR05** sampai keputusan kontrak
AADv2/remediasi terpisah. Tidak ada Android/ADB, kamera, manusia, migrasi Product
Owner, threshold manipulation atau D-030 policy change.
Laporan A–K, perintah dan evidence: `FL_ENR04_R2_TECHNICAL_REPORT.md`.
**STOP; R3/R4/FL-ENR05 tidak dimulai otomatis.**


## 21. Bukti FL-SEC01 — template AAD v2 record binding

| Pemeriksaan | Hasil yang dijalankan |
|---|---|
| Sameperson/model, target templateID berbeda |FAIL-before plaintext=true→PASS-after tanpa plaintext|
| AAD lengkap |Template/submission/person/immutable grant/model/version/fingerprint/key-version; forged storedAAD tetap ditolak GCM|
| Encoding |Delimiter dan no-separator tuple collision ditolak; identical binding menghasilkan deterministicAAD|
| Legacy/unknown |Authenticv1 unsupported tanpa fallback; relabel/rewriteAAD ditolak; purpose-specific version check sebelum key lookup|
| SQLite coherent transplant |Full envelope dengan identitas template/submission/grant diubah independen, FK/join tetap valid: tidak ready, ciphertext retained|
| Migration7→8 |Legacy bytes/metadata/ledger≤7/FK preserved; newv1INSERT ditolak; default migrations tidak berubah|
| Withdrawal |Grant historis tetap; ACTIVEv2→REVOKED/template_readyfalse/deletionCOMPLETE; payload/files0|
| Failure/concurrency |Encryptionfailure noACTIVE; auditrollback, stale lease/attempt/epoch/reviewer, keyloss/corruption/restart/reconcile retained-safe; dua record/vector tidak tertukar|
| Key ownership |Owned copies wiped pada Seal/Open success, authenticationfailure, shortkey dan providererror|
| Go lengkap+vet |18paket ok,4tanpa tes; vet exit0|
| Python |217PASS,1SKIP|
| Real-model HTTPS |18CA/hostname verified requests, actual YuNet/SFace/IPC, envelopeversion2,128float32finite/unit,ciphertext528byte|
| Mobile |Typecheck/lint lulus;219PASS,6SKIP; explicit existing E2E AdminHTTPS5+mahasiswaHTTP1:6/6PASS|
| Independent review |Crypto/domain/migration/caller/security; tidak ada evidence-backed defect tersisa|

Run Go awal gagal pada fixture baru (multi-statement parameter binding dan
legacy activation); setup diperbaiki dengan Exec terpisah/RowsAffected tanpa
melemahkan assertion, kemudian fullsuite/vet lulus. Catatan skip/host dan
perintah reproduksi ada pada `FL_SEC01_TECHNICAL_REPORT.md` §J.
AEAD tidak mencegah rollback seluruh DB/state yang konsisten; replay dengan
binding sama tetap authentic dan memerlukan kontrol freshness di luar AEAD.
Tidak ada human/camera/ADB, model/threshold/auth/session/role/KeyProvider change,
atau migration database Product Owner. D-030 tetap BLOCKED.
**PASS sintetis/disposable; STOP tanpa memulai FL-ENR05 otomatis.**

## 22. Bukti FL-ENR05 — synthetic integrated E2E

- Continuous actual Student/Admin TS APIs+controllers, normal OTP/verifiedHTTPS,
  Go staging, satu WAL schema8 disposable, real pinned YuNet/SFace IPC,
  encrypted template AADv2, server ACTIVE, withdrawal/REVOKED/cleanup0: PASS.
- Protected preview SHA = canonical submission SHA = actual Process input SHA;
  approval202 belum ACTIVE, embedding128finite/unit dan manifest tervalidasi.
- Failure/concurrency23 kasus dipetakan per lapisan pada laporan §H; actual TLS
  consent/72h/retryTTL/reviewer/lease races, SQLite reopen/reconstruction,
  filesystem cleanup outage, adapter child timeout/crash, crypto/legacy/keyloss.
- Response-loss diinjeksikan setelah realHTTPS approval commit: uncertain,
  noACTIVE, identical body/CAS/key replay200, approval audit+1/replay+0.
  Bukan physical network disconnect atau Android device evidence.
- Student token-null ACTIVE distrust:3FAIL-before; load/manual/background
  stale+recovery PASS-after, termasuk smoke dengan ACTIVE backend aktual.
- OTP fixture barrier menunggu exact committed delivery; auth policy unchanged.
  Activation/withdrawal cleanup diperiksa setelah existing reconciliation.
- Final Go18paket+4tanpa tests dan vet exit0; Python217PASS/1SKIP;
  mobile typecheck/lint,17files/230PASS (8HTTPS E2E opt-in aktif);
  offline JS/native contract36PASS. Android UI/native build/ADB NOT_RUN.
- Run awal gagal karena timing cleanup, receipt24h disalahartikan sebagai72h,
  dan helper polling terhapus; diperbaiki tanpa melonggarkan runtime kontrak.
  Dua trace FAIL dipertahankan; tidak dihitung sebagai PASS.
- Laporan A–N, commands, measurement dan satu trace:
  `FL_ENR05_TECHNICAL_REPORT.md`; trace `<redacted-id>`.
  Tidak ada human/productionDB/modelthreshold/native/SMTP/authrole redesign.
  **GO Synthetic Integrated E2E; STOP tanpa human/production/tahap baru.**

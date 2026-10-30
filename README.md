# FaceLink AI

> Sistem absensi otomatis berbasis pengenalan wajah, kamera IoT, dan aplikasi React Native. Dibangun secara lokal terlebih dahulu, dengan perhatian khusus pada privasi dan kemudahan pengembangan.

**FL-GIT02 — persiapan integrasi terisolasi; publikasi belum diizinkan.**
Source yang ditinjau diekspor ke `integration/fl-git01-source-preservation`,
disanitasi dan diuji tanpa mengubah workspace asal atau backup FL-GIT01.
Laporan: [`docs/FL_GIT02_TECHNICAL_REPORT.md`](docs/FL_GIT02_TECHNICAL_REPORT.md).
Daftar exact-file, status 23 review manual dan sembilan source ignore:
[`docs/FL_GIT02_PUBLICATION_MANIFEST.json`](docs/FL_GIT02_PUBLICATION_MANIFEST.json).
Tidak ada staging/commit/push/PR/merge, ADB/uninstall, atau tahap fitur baru.
Status FL-GIT01 dan sebelumnya berikut dipertahankan sebagai snapshot historis.


**FL-GIT01: source diamankan lokal; publikasi HOLD.**
Snapshot source/bundle Git terverifikasi dengan akses terbatas; sembilan
source tersembunyi ignore rules ikut diselamatkan. PR #19 terkonfirmasi merged.
Audit menemukan identitas operasional pada source/evidence/history yang
memerlukan redaksi/review; tiga whitespace findings belum diubah.
Smoke synthetic terpadu baru2/2PASS. Tidak ada commit/push/PR/merge, ADB atau uninstall.
Rencana branch/worktree terpisah dan approval gate:
[`docs/FL_GIT01_TECHNICAL_REPORT.md`](docs/FL_GIT01_TECHNICAL_REPORT.md).
Status FL-STAB02 dan sebelumnya berikut dipertahankan sebagai bukti historis.

**FL-STAB02: crash UNRESOLVED / Fast-Track PARTIAL; laporan selesai.**
Setelah izin baru, debug dan release-like diuji di TECNO: sesi, preview,
offline trust dan withdrawal disposable terbukti pada kasus terbatas.
Tidak ada fatal baru yang teramati; crash asli belum direproduksi/fixed.
Controller85/85PASS; release-like build exit0. Enam package terinventaris,
tidak uninstall; main/APK/video/model dipertahankan. Runtime task ditutup aman.
Uncertainty/manual replay UI tetap belum terdemonstrasi.
Laporan dan kandidat cleanup: [`docs/FL_STAB02_TECHNICAL_REPORT.md`](docs/FL_STAB02_TECHNICAL_REPORT.md).
Status FL-ENR06-R1 di bawah dipertahankan sebagai bukti historis.

**FL-ENR06-R1: positive Android E2E PASS; keseluruhan PARTIAL (29 Oktober 2026).**
Satu submission baru melalui UI Student dan Admin pada dua package staging
terisolasi → real YuNet/SFace → template AADv2/ACTIVE → withdrawal → REVOKED
dan cleanup0 terbukti pada TECNO. Form Admin yang bergeser karena history
polling diperbaiki dan diuji pada perangkat. Satu crash native React Native
dev inspector pada lifecycle tambahan masih terbuka; bukan PASS lifecycle penuh.
Kedua staging di-force-stop, host berhenti, database disposable dihapus;
APK utama/video/model utuh. Tidak ada kamera, manusia, production atau tahap baru.
Laporan A–L: [`docs/FL_ENR06_R1_TECHNICAL_REPORT.md`](docs/FL_ENR06_R1_TECHNICAL_REPORT.md).
**STOP. Status FL-ENR06 dan sebelumnya berikut adalah snapshot historis.**

**FL-ENR06: PARTIAL — kelanjutan tanpa HP.** Student Android sebelumnya
mencapai `PENDING_APPROVAL`; full Admin→ACTIVE→withdrawal Android belum terbukti.
Pemilik meminta lanjut tanpa TECNO yang sedang dipakai; tidak ada ADB pada
kelanjutan ini. Regresi client229PASS/9SKIP + typecheck/lint dan smoke terpadu
actual TS→Go HTTPS→real Python2/2 lulus pada database disposable baru, termasuk
ACTIVE/AADv2→withdrawal/cleanup0. Smoke bukan PASS UI Android.
Rincian: [`docs/FL_ENR06_TECHNICAL_REPORT.md`](docs/FL_ENR06_TECHNICAL_REPORT.md).

Status FL-ENR05 di bawah merupakan snapshot sebelumnya.

**FL-ENR05: GO — Synthetic Integrated E2E.**
Satu continuous client TypeScript → Go HTTPS/SQLite WAL schema8 → real
YuNet/SFace IPC → encrypted template AADv2 → server ACTIVE → withdrawal →
REVOKED/cleanup0 terbukti. Login OTP normal, protected-preview hash sama
dengan artefak yang diproses; approval202 bukan ACTIVE. Stale ACTIVE saat
token tidak tersedia diperbaiki; response-loss/exact replay tanpa audit ganda.
Go18paket+vet, Python217PASS/1SKIP, mobile typecheck/lint+230PASS termasuk
8 client E2E HTTPS, native/JS offline36PASS. Android UI NOT_RUN; APK/video/model
utuh. Tidak ada ADB/kamera/manusia, migrasi Product Owner atau commit/push/merge.
Laporan A–N dan correlation trace:
[`docs/FL_ENR05_TECHNICAL_REPORT.md`](docs/FL_ENR05_TECHNICAL_REPORT.md).
**STOP; bukan izin human enrollment, production, face attendance atau tahap baru.**

Status FL-SEC01 dan sebelumnya berikut adalah snapshot historis.

**FL-SEC01: PASS — template AES-256-GCM AAD v2 record-bound.**
Template/submission/person/immutable consent grant/model/pipeline/key version
terikat; swap yang sebelumnya membuka plaintext kini ditolak. Template v1
unsupported/quarantined tanpa fallback atau write baru. Migration00008 hanya
disposable, mempertahankan bytes legacy dan ledger; database Product Owner tidak disentuh.
Real YuNet/SFace→IPC→templatev2→ACTIVE→withdraw→REVOKED lulus melalui HTTPS.
Go18paket+vet, Python217PASS/1skip, mobile219PASS/6skip dan client E2E6/6 lulus.
Laporan A–M: [`docs/FL_SEC01_TECHNICAL_REPORT.md`](docs/FL_SEC01_TECHNICAL_REPORT.md).
**GO terbatas review/perencanaan FL-ENR05 sintetis; bukan izin menjalankannya.
STOP; NO-GO manusia/production/migrasi Product Owner.** Full DB rollback dan
D-030 irreversible deletion tidak diselesaikan oleh AAD.

Status R2 dan sebelumnya di bawah dipertahankan sebagai snapshot historis.

**FL-ENR04-R2: positive real-model PASS; NO-GO menuju FL-ENR05 karena binding AAD v1.**
Penyebab NO_FACE R1: pembesaran input 512→960. Resolusi native dipertahankan;
threshold0,75/quality gate/model tetap. Fixture existing melewati YuNet/SFace,
IPC dan HTTPS disposable hingga encrypted ACTIVE→withdraw→REVOKED tanpa payload tersisa.
Pertukaran envelope same-person/model/version oleh penulis DB offline diterima;
UPDATE biasa ditolak, bukan bukti exploit endpoint. Kontrak ciphertext tidak diubah.
Laporan A–K: [`docs/FL_ENR04_R2_TECHNICAL_REPORT.md`](docs/FL_ENR04_R2_TECHNICAL_REPORT.md).
**STOP; tidak ada izin manusia, production, migrasi Product Owner, atau R3/R4 otomatis.**

Hasil R1 dan FL-ENR04 di bawah adalah snapshot historis sebelum perbaikan R2.

**FL-ENR04-R1: PARTIAL — enam fixture prosedural lokal tetap NO_FACE.**
Threshold0,75 dan quality gate tidak diubah. Harness Go/Python nyata menolak
kandidat01 tanpa template/ACTIVE; positive extraction/encrypted activation
**belum terbukti**. Python213PASS/1skip, Go18paket+vet, E2E6/6; tes positif
terkonfigurasi justru FAIL/NO_FACE dan tidak dihitung sebagai PASS.
Laporan A–N: [`docs/FL_ENR04_R1_TECHNICAL_REPORT.md`](docs/FL_ENR04_R1_TECHNICAL_REPORT.md).
**STOP; belum GO FL-ENR05, tidak ada izin manusia/production.**

**FL-ENR04 (27 Oktober 2026): integrasi model lokal sintetis/disposable tersedia.**
Go menjalankan YuNet/SFace melalui subprocess ber-IPC privat terautentikasi dan
terbatas; hanya command staging dengan tiga flag `-cv-*` eksplisit. Smoke nyata
membuktikan approval202 → `FAILED_PROCESSING / NO_FACE`, tanpa template atau
sukses palsu. **REAL_MODEL_POSITIVE_NOT_PROVEN**; tidak ada izin manusia,
production, kamera, atau FL-ENR05 otomatis.
Laporan A–O: [`docs/FL_ENR04_TECHNICAL_REPORT.md`](docs/FL_ENR04_TECHNICAL_REPORT.md).

**FL-STAB01 (27 Oktober 2026): F1/F2 diperbaiki dan diverifikasi terbatas.**
Logout memakai refresh proof current/previous dan mencabut hanya sesi itu,
termasuk setelah access expiry; kegagalan remote/local cleanup dilaporkan jujur.
Lifecycle crossfade video membatalkan callback sebelum player dilepas.
Go 17 paket + vet, mobile 219 PASS/6 skipped, HTTPS E2E Admin5/5 dan Mahasiswa1/1,
serta acceptance TECNO LJ7 sintetis lulus pada skenario yang dijalankan.
PR #19 merged `95d31d8`; 21 commit lokal dan pekerjaan uncommitted belum masuk main.
Tidak ada commit/push/merge atau izin FL-ENR04/production otomatis.
Rincian: [`docs/FL_STAB01_TECHNICAL_REPORT.md`](docs/FL_STAB01_TECHNICAL_REPORT.md).
Entri tahap sebelumnya di bawah adalah snapshot historis, bukan status acceptance terbaru.


**Status (FL-ENR03, 27 Oktober 2026): PARTIAL; Admin enrollment review sintetis selesai di staging, acceptance perangkat NOT_RUN.** Login OTP tiga role, people/roster/binding, attendance, serta consent/enrollment sintetis berjalan pada database disposable. Admin staging dapat melihat antrean/detail identitas, preview JPEG sintetis, dan mengirim Approve/Reject/Retry melalui endpoint existing; route hanya ada di staging default-OFF dan `cmd/server` tetap tanpa enrollment. Bukti: Go 17 paket ok/4 tanpa tes + vet; mobile typecheck/lint, 206 tes lulus/6 skipped; E2E HTTPS Go nyata 5/5; stage typecheck/lint + export1093 modul; APK ARM64 build sukses tanpa CAMERA/media. Web authenticated, ADB/perangkat, TalkBack, enrollment production, template manusia, dan recognition otomatis **belum terbukti/belum aktif**. Google tetap disabled + “Segera hadir”; dashboard non-enrollment masih memakai fixture. Detail dan batas: [`docs/FL_ENR03_TECHNICAL_REPORT.md`](docs/FL_ENR03_TECHNICAL_REPORT.md).
**Lingkup:** Prototipe tugas kampus berskala kecil: satu kelas, satu terminal kamera, dan sekitar 5–10 peserta sukarela yang memberikan persetujuan.  
**Biaya:** Seluruh perangkat lunak dan pemrosesan AI lokal harus dapat berjalan dengan biaya lisensi/layanan **Rp0**. Anggaran hardware maksimal **Rp1.000.000**.  
**Cloud:** Google Cloud dan layanan cloud lain ditunda sampai sistem lokal selesai diuji serta mendapat persetujuan pengembang.  
**Pengembangan dengan AI:** OmO direncanakan sebagai pengatur agent dengan model awal `deepseek-v4.1-flash`. Semua spesifikasi harus dapat dibaca agent dan model lain.

**FL-CORE01 (25 Oktober 2026): fondasi nonbiometrik selesai dan teruji terisolasi.**
People CRUD, NIM immutable/unik, attestation referensi roster oleh Admin,
binding mahasiswa satu-ke-satu, kelas/membership, RBAC dan audit transaksional
sudah tersedia. Go tests, vet, dan smoke API HTTPS pada SQLite WAL disposable
lulus. Migration `00005` **belum diterapkan ke database Product Owner**; jangan
menjalankan contoh `-migrate` di README pada database existing tanpa izin tambahan.
R5/FL-ENR00/FL-ENR00R tetap dipertahankan; tidak ada perubahan OTP/session,
akun testing, UI, video, atau keputusan P01–P12. Absensi manual/sesi dan
biometrik belum diimplementasikan. Rincian: [`backend/README.md`](backend/README.md).

**FL-CORE02 (25 Oktober 2026): sesi dan fondasi attendance selesai, teruji disposable.**
Lifecycle draft/open/closed, fallback manual beralasan, koreksi historis, rekap
ETag/pagination, dan riwayat mahasiswa sendiri tersedia di backend sesuai RBAC.
Face Recognition tetap mekanisme absensi **utama** yang direncanakan; manual
hanya fallback terotorisasi. Recognition/camera/enrollment production/template manusia belum aktif; enrollment sintetis hanya berjalan di staging disposable (FL-ENR01B–ENR03).
Go tests (14 paket), vet, upgrade data-preserving schema 5→6, dan smoke socket
HTTPS lulus. Race detector **NOT_RUN**. Migration `00006` belum diterapkan ke
database Product Owner; tidak ada perubahan auth, akun testing, UI atau P01–P12.
Bukti acceptance dan batas: [`docs/TEST_PLAN.md` §9](docs/TEST_PLAN.md#9-bukti-fl-core02--sesi-dan-fallback-manual-25-oktober-2026).

**FL-ENR02B-R2 (26 Oktober 2026): positive readiness dan same-frame capture LIVE terbukti.**
Kamera TECNO menghadap gambar mannequin nonmanusia di browser lokal: dwell1504.66ms/32 sampel,
frame1512 sama sampai JPEG_READY → CONSUMED → RELEASED; stale/duplicate native ditolak,
buffer native kembali nol. Tidak ada upload atau enrollment ACTIVE akibat capture.
Keseluruhan **PARTIAL**: batas mirror/crop absolut dan lifecycle saat encoding masih ada.
Main/settings utuh; runtime dihentikan dan stage kembali synthetic tanpa trace/probe aktif.
[Laporan A–M R2](docs/FL_ENR02B_R2_READINESS_CAPTURE_REPORT.md).
**STOP; bukan izin otomatis FL-ENR03, Admin UI implementation, manusia atau production.**

**Catatan historis FL-ENR02B-R1 (26 Oktober 2026): Android live mannequin acceptance PARTIAL.**
Preview/permission/negative guidance dan lifecycle diuji pada TECNO LJ7 setelah
izin baru; capture same-frame/consume LIVE belum terbukti. Sink lifecycle dan
copy intro mannequin diperbaiki; main APK tetap utuh, tidak ada live submission.
**NO-GO pada saat itu menuju Admin Enrollment UI integration. FL-ENR03 kemudian mengimplementasikan UI Admin synthetic default-OFF; acceptance perangkat tetap NOT_RUN.**
[Laporan A–M](docs/FL_ENR02B_R1_ANDROID_ACCEPTANCE_REPORT.md).

**FL-ENR02B (26 Oktober 2026): native camera source integration offline, synthetic staging.**
`FaceLinkCaptureLive` CAM00 + detector patch; default synthetic tanpa kamera,
mannequin staging terpisah dengan CAMERA/FLAG_SECURE dan tanpa upload live.
147 tes mobile/staging, mannequin debug ARM64 dan synthetic build lulus.
Live camera/mannequin NOT_RUN; tidak ada izin perangkat baru.
[Laporan A–L](docs/FL_ENR02B_TECHNICAL_REPORT.md).


**FL-ENR02A-R2 (26 Oktober 2026): warning lifecycle video yang direproduksi sudah diperbaiki.**

Video login ditunda sampai session restore selesai; A/B TECNO tanpa warning pasca-fix
dan tanpa crash. Target sentuh, label/role, announcement status, copy offline, dan
font scale diperbaiki. Residual TalkBack penuh/kontras brand. Main/staging143 tes
lulus; export Android lulus. [Laporan A–H](docs/FL_ENR02A_R2_VIDEO_ACCESSIBILITY_REPORT.md).


Menu main default-off; route hanya ada pada staging ber-package terpisah.
Typecheck/lint,139 tes mobile dan E2E client/backend nyata lulus; APK debug
ARM64 berhasil dibangun tanpa permission kamera/media/storage/overlay.
Acceptance visual/device Android masih **PARTIAL**, tanpa ADB/install pada tugas ini.
Tidak mengaktifkan enrollment production atau capture manusia.
Laporan: [`docs/FL_ENR02A_TECHNICAL_REPORT.md`](docs/FL_ENR02A_TECHNICAL_REPORT.md).
Pembaruan R1: alur inti pada TECNO LJ7 dengan HTTPS staging nyata sudah terbukti,
termasuk Reject/retry, ACTIVE, withdrawal, offline-stale dan Expired.
Status keseluruhan tetap **PARTIAL** karena warning lifecycle video host dan
accessibility menyeluruh belum lengkap. Main APK/setting HP tetap utuh;
staging sudah dihentikan. [Laporan R1 A–M](docs/FL_ENR02A_R1_ANDROID_ACCEPTANCE_REPORT.md).

## Masalah yang ingin diselesaikan

Absensi manual memerlukan waktu dan dapat menimbulkan kesalahan pencatatan. FaceLink AI menguji apakah proses kehadiran satu kelas dapat dibantu oleh terminal kamera yang mengenali peserta **yang telah mendaftar secara sukarela**, lalu mencatat kehadiran secara otomatis pada sesi yang dibuka **dosen** di kelas yang ditugaskan kepadanya (rancangan **FL-010U**).

Proyek ini adalah **prototipe akademik**, bukan alat pengawasan massal atau sistem biometrik siap produksi. Sistem harus menyediakan jalur absensi manual dan **tidak boleh menyimpulkan seseorang tidak hadir hanya karena pengenalan wajah gagal**.

## Alur singkat

Alur di bawah adalah rancangan tiga role pengguna manusia. **Login Email OTP dasar sudah terbukti melalui data/audit backend dan testing manual Android**; restore/refresh HP target dan live RBAC belum lengkap. Langkah absensi, konsen, dan enrollment biometrik **belum diimplementasikan end-to-end**; nilai fitur pada dashboard masih contoh, bukan bukti data live.

1. Pengguna masuk dengan **Email OTP** melalui “Continue with FaceLink”: aplikasi meminta `POST /api/v1/auth/email/request`, backend hanya mengirim kode 8 digit (TTL 5 menit, HMAC, rate limit) bila email sudah diundang dan berstatus sah, lalu `POST /api/v1/auth/email/verify` menukar kode menjadi sesi. Google Sign-In **tetap tersedia di backend** tetapi tombolnya disabled pada MVP ini. Role **`admin`, `dosen`, atau `mahasiswa`** selalu dibaca dari database FaceLink AI, tidak dari domain email maupun aplikasi.
2. `admin` menyiapkan akun dan role, kelas beserta penugasan dosen, peserta, serta konsen; `dosen` masuk ke **kelas yang ditugaskan kepadanya** dan membuka sesi absensi.
3. Peserta telah didaftarkan dengan persetujuan yang jelas; data wajah diproses secara lokal.
4. Peserta berdiri seorang diri di depan webcam atau kamera IoT.
5. Mesin pengenalan wajah memeriksa kualitas gambar, mencari kecocokan, dan mengembalikan hasil terbatas.
6. Backend memeriksa persetujuan, keanggotaan kelas, status sesi, dan kemungkinan absensi ganda.
7. Jika seluruh pemeriksaan lolos, satu catatan kehadiran dibuat; `dosen` melihat daftar hadir berjalan dan rekap kelasnya, sedangkan `mahasiswa` melihat status dan riwayat kehadirannya **sendiri** lewat portal self-service (`/me/*`), tanpa akses ke data biometrik atau data peserta lain.
8. Kegagalan deteksi, kecocokan meragukan, atau layanan yang tidak tersedia menghasilkan instruksi coba lagi/verifikasi manual, bukan absensi otomatis.

`device` **bukan** role pengguna manusia: perangkat IoT tetap memakai identitas dan autentikasi mesin terpisah (token perangkat) dan hanya boleh memanggil endpoint pengiriman percobaan pengenalan. Rancangan lengkap matriks peran/izin ada di [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md). **Bukti login tidak menyatakan alur absensi, konsen, enrollment, atau pengenalan wajah sudah berjalan.**

## Batasan produk

- **Untuk sekarang:** satu kelas demo, satu kamera, **tiga role pengguna (`admin`, `dosen`, `mahasiswa`) dengan Email OTP sebagai login utama; Google disabled sementara**, peserta terbatas, rekap kehadiran, ekspor laporan, dan APK Android.
- **Untuk nanti:** bisa diadaptasi ke kantor/organisasi lain melalui desain modular, tetapi belum perlu fitur multiorganisasi, penggajian, atau integrasi HRIS.
- **Perangkat IoT (`device`) bukan role manusia:** perangkat tetap memakai identitas dan autentikasi mesin terpisah (token perangkat, bukan Google), hanya untuk mengirim percobaan pengenalan.
- **Jaringan backend:** kebijakan final **FL-010V (16 Oktober 2026)** menetapkan **default loopback** (kode FL-005 masih menolak alamat non-loopback) dengan akses HP lewat LAN privat hanya via konfigurasi eksplisit dan **wajib HTTPS bersertifikat tepercaya** (D-035); HP karena itu **belum** dapat menjangkau backend lewat Wi-Fi kelas sampai pengikatan LAN beserta TLS diimplementasikan. Rincian penyediaan sertifikat lokal dan trust chain Android **belum diuji** (pertanyaan terbuka **Q-25**).
- **Pengenalan wajah dengan kacamata:** tujuan pengujian, **bukan janji akurasi sempurna**.
- **Pencegahan spoofing dasar:** sasaran eksperimen; kedipan atau gerakan kepala saja tidak menjamin ketahanan terhadap foto/video.
- **iOS:** dapat mempertimbangkan web/PWA atau pengujian Expo Go, tetapi distribusi native iOS tidak dijanjikan gratis.

## Peta dokumen

| Dibaca | Dokumen | Tujuan |
|---|---|---|
| 1 | [`AGENTS.md`](AGENTS.md) | Aturan wajib dan cara kerja AI coding agent |
| 2 | [`PROJECT_STATE.md`](PROJECT_STATE.md) | Kondisi aktual project dan pekerjaan yang boleh dilakukan |
| 3 | [`docs/PRD.md`](docs/PRD.md) | Kebutuhan produk, ruang lingkup, dan kriteria penerimaan |
| 4 | [`docs/DECISIONS.md`](docs/DECISIONS.md) | Keputusan disetujui vs usulan yang belum disahkan |
| 5 | [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | Batas modul, pola komunikasi, dan urutan pengembangan |
| 6 | [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md) dan [`docs/DATA_MODEL.md`](docs/DATA_MODEL.md) | Rancangan API dan database sebelum implementasi |
| 6b | [`docs/EMAIL_OTP.md`](docs/EMAIL_OTP.md) | Kontrak login Email OTP, SMTP, rate limit, dan transisi Admin |
| 7 | [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md) | **Sumber tunggal matriks peran dan izin** (`admin`, `dosen`, `mahasiswa`, `device`) |
| 8 | [`docs/FACE_ENGINE.md`](docs/FACE_ENGINE.md) dan [`docs/SECURITY_PRIVACY.md`](docs/SECURITY_PRIVACY.md) | Pengenalan wajah, keamanan, dan persetujuan data |
| 9 | [`docs/UX_FLOWS.md`](docs/UX_FLOWS.md) dan [`docs/TEST_PLAN.md`](docs/TEST_PLAN.md) | Alur aplikasi dan pengujian |
| 10 | [`docs/ROADMAP.md`](docs/ROADMAP.md) dan [`docs/BACKLOG.md`](docs/BACKLOG.md) | Tahapan dan unit pekerjaan AI coding |
| 11 | [`docs/DEVICE_HARDWARE.md`](docs/DEVICE_HARDWARE.md) | Rencana perangkat IoT di fase berikutnya |
| 12 | [`docs/ANDROID_SETUP.md`](docs/ANDROID_SETUP.md) | Setup JDK 17, Android SDK, ADB, dev build, dan Google OAuth Android (Windows) |
| 13 | [`docs/AGENT_BOOTSTRAP.md`](docs/AGENT_BOOTSTRAP.md) dan [`docs/COLLABORATION.md`](docs/COLLABORATION.md) | Mulai sesi agent dan kolaborasi GitHub |
| 14 | [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md) dan [`docs/REFERENCES.md`](docs/REFERENCES.md) | Keputusan terbuka dan bacaan teknis |
| — | [`research/README.md`](research/README.md) | Tempat terpisah untuk jurnal/riset akademik tim |

## Kandidat teknologi (stack final MVP setelah FL-003)

- **Aplikasi:** React Native + Expo + TypeScript, Android lebih dahulu.
- **Backend:** **Go + `chi`** sebagai backend utama dan pemilik keputusan domain (D-010 DISETUJUI). Python tetap dipakai, tetapi hanya sebagai **mesin pengenalan wajah terpisah**, bukan backend publik; satu backend Python + FastAPI ditolak (D-019).
- **AI lokal:** Python + OpenCV dengan **YuNet** (deteksi) dan **SFace** (pengenalan) sebagai model MVP (D-012); bobot berlisensi non-permisif seperti InsightFace/ArcFace tidak dipakai (D-025). **Ambang kecocokan belum ditetapkan** dan harus dikalibrasi pada peserta uji.
- **Database:** **SQLite mode WAL** untuk MVP (D-020), dengan skema dan migrasi tetap portabel ke PostgreSQL; PostgreSQL awal ditunda (D-011). Migrasi dijalankan dari sisi Go (D-026).
- **Perangkat kamera:** webcam selama development; ESP32-S3 CAM setelah software bekerja, sebagai klien pengunggah JPEG (D-023), bukan host model AI.
- **Autentikasi:** **Email OTP** sebagai login utama bagi tiga role manusia (`admin`, `dosen`, `mahasiswa`) — backend dan layar mobile sudah diimplementasikan; Google tetap tersedia di backend tetapi tombolnya disabled + “Segera hadir”. Login OTP Android pernah diverifikasi, tetapi acceptance perangkat terbaru tetap NOT_RUN. Tidak ada login email/kata sandi buatan sendiri; role **hanya** ditetapkan `admin` di backend dan tidak pernah berasal dari Google atau dari aplikasi. Sesi aplikasi memakai token opak (bukan ID token Google) yang disimpan sebagai hash di tabel `session_tokens`. `device` memakai jalur autentikasi mesin yang terpisah.

Kontrak dan skema rinci ada di [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md) dan [`docs/DATA_MODEL.md`](docs/DATA_MODEL.md); matriks peran/izin ada di [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md). **Persetujuan stack ini adalah persetujuan arsitektur, bukan izin implementasi.**

## Kode yang sudah ada (FL-005)

Bagian yang sudah benar-benar ditulis dan diuji: **fondasi backend Go** di folder [`backend/`](backend/README.md) (kerangka layanan, SQLite mode WAL, migrasi SQL berversi, log JSON, `GET /api/v1/healthz` dan `GET /api/v1/readyz`) dan **autentikasi serta otorisasi tiga role (FL-011)** — verifikasi OIDC Google, lifecycle akun undangan, sesi opak dengan rotasi penyegar dan batas absolut 30 hari, otorisasi `admin`/`dosen`/`mahasiswa`, serta audit keamanan. **Sudah ada pada pengujian disposable:** aplikasi React Native/Expo, dashboard, people/roster/binding, sesi attendance, dan consent/enrollment sintetis staging. **Belum ada/belum aktif:** mesin pengenalan wajah Python, firmware ESP32, devices/recognition/CSV/audit-read, enrollment production, atau template manusia. Perlu dicatat apa adanya: kode ini masih menolak pengikatan ke alamat non-loopback, sehingga HP belum dapat menjangkau backend lewat Wi-Fi kelas sampai FL-011 mengimplementasikan pengikatan LAN beserta TLS (kebijakan **D-035/FL-010V**; rincian sertifikat belum diuji, lihat **Q-25** pada [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md)).

```powershell
cd backend
go run ./cmd/server -migrate   # terapkan migrasi
go run ./cmd/server            # jalankan, mendengarkan di 127.0.0.1:8080
```

Seluruh variabel lingkungan didokumentasikan pada [`.env.example`](.env.example). Petunjuk lengkap, termasuk cara menjalankan pengujian, ada di [`backend/README.md`](backend/README.md).

`backend/` di akar repositori adalah **lokasi resmi** backend Go (lihat [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) §8.1). Usulan lama `apps/api/` tidak dipakai. Quality gate otomatis ada di [`.github/workflows/backend-go.yml`](.github/workflows/backend-go.yml) dan berjalan pada Windows dan Ubuntu untuk setiap Pull Request ke `main`.

> Aplikasi React Native, mesin AI Python, dan firmware ESP32 **belum dimulai**. Jangan membaca keberadaan folder `backend/` sebagai tanda fitur absensi sudah bekerja.

## Catatan revisi FL-010U (16 Oktober 2026)

Revisi README ini berasal dari **FL-010U (16 Oktober 2026)**, cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`), dan **hanya dokumentasi** — tidak ada kode maupun migrasi yang ditulis.

- **Tiga role manusia** (`admin`, `dosen`, `mahasiswa`) dengan **Google Sign-In** sebagai metode masuk; akun mahasiswa kini ada. Role `operator` lama digantikan `dosen`, dan **cakupan `operator` lama terbagi menjadi `admin` dan `dosen`** — bukan penggantian nama dengan cakupan identik.
- **`device` bukan role pengguna manusia**: perangkat IoT tetap memakai identitas dan autentikasi mesin terpisah.
- Rancangan lengkap matriks peran/izin: [`docs/AUTHORIZATION.md`](docs/AUTHORIZATION.md). Kontrak API: [`docs/API_CONTRACT.md`](docs/API_CONTRACT.md); model data: [`docs/DATA_MODEL.md`](docs/DATA_MODEL.md); alur aplikasi: [`docs/UX_FLOWS.md`](docs/UX_FLOWS.md); pertanyaan yang masih terbuka (termasuk pengikatan jaringan dan cakupan dosen): [`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md).
- **Tidak ada klaim fitur sudah ada.** Skenario runtime tiga role dan Google Sign-In **sudah dijalankan** untuk bagian backend (FL-011): hasilnya dipetakan pada [`docs/TEST_PLAN.md`](docs/TEST_PLAN.md) §8.5 — 20 dari 29 skenario TERVERIFIKASI, 7 sebagian, 2 belum (perangkat IoT dan aplikasi klien). Skenario yang bergantung pada aplikasi klien, TLS LAN Android, absensi, dan biometrik tetap **BELUM TERVERIFIKASI**.

## Catatan revisi FL-010V (16 Oktober 2026)

Revisi README ini berasal dari **FL-010V (16 Oktober 2026)**, cabang `docs/fl-010v-final-auth-decisions`, dan **hanya dokumentasi** — tidak ada kode maupun migrasi yang ditulis, dan matriks 53 endpoint tidak diubah.

- **Keputusan arsitektur difinalkan:** **D-032–D-037 disetujui** (D-035 pada tingkat kebijakan) — lifecycle akun, session management, hak akses `dosen`, akses jaringan lokal, kebijakan audit keamanan, dan **satu role utama per akun**. Status resmi keputusan ada di [`docs/DECISIONS.md`](docs/DECISIONS.md).
- **Pertanyaan terbuka bergerak sebagian saja:** **Q-19** menjadi kebijakan yang diputuskan (D-035), **Q-20 → D-034**, **Q-21 → D-037**, dan **Q-25** (metode penyediaan sertifikat lokal & trust chain Android) baru dicatat; **Q-22, Q-23, dan Q-24 tetap terbuka** dan tidak otomatis disetujui ([`docs/OPEN_QUESTIONS.md`](docs/OPEN_QUESTIONS.md)).
- **FL-011 sudah dikerjakan** sebagai implementasi Google Authentication & Authorization backend dengan **12 butir lingkup wajib** — hasilnya pada cabang `feat/fl-011-google-auth-backend` (18 endpoint berjalan, quality gate lokal hijau, PR belum dibuat), dan **FL-005S** didaftarkan sebagai penguatan wajib — access log Go dapat melaporkan status `200` untuk respons `503` batas waktu dan tidak mencatat respons `500` hasil pemulihan panic; **FL-005S SELESAI secara implementasi pada 16 Oktober 2026** (cabang `fix/fl-005s-http-access-logging`): access log kini mencatat status final yang benar-benar dikirim klien dan ditulis tepat satu kali per permintaan saat respons selesai, diuji oleh `backend/internal/httpapi/accesslog_test.go` (12 skenario), tetapi **belum di-merge** sampai PR ditinjau ([`docs/BACKLOG.md`](docs/BACKLOG.md)). Urutan fase pada [`docs/ROADMAP.md`](docs/ROADMAP.md) **tidak berubah**.
- **Keamanan jaringan** ditambahkan ke bagian **Keamanan repository** di bawah: default loopback, akses LAN wajib HTTPS bersertifikat tepercaya, dan backend **tidak pernah** dibuka ke internet publik. Rincian teknis TLS LAN **belum diuji**.
- **Tidak ada klaim fitur sudah ada.** Skenario TLS LAN dan trust Android (Q-25) tetap **BELUM TERVERIFIKASI**, begitu pula seluruh jalur absensi dan biometrik; hasil yang sudah nyata hanya yang tercantum pada `docs/TEST_PLAN.md` §8.5.

## Catatan asal README

Teks berikut adalah isi `README.md` asli yang sudah ada di branch `main` sebelum FL-001, dipertahankan apa adanya sebagai rujukan:

> # FaceLink-Ai
> FaceLink AI is a local-first, AI-powered smart attendance system integrating facial recognition, IoT, and a React Native mobile application.

Isi di atas tetap berlaku dan telah diterjemahkan ke seluruh dokumen berbahasa Indonesia ini. Judul `FaceLink-Ai` digantikan `FaceLink AI` agar konsisten dengan nama yang dipakai di seluruh dokumentasi.

## Keamanan repository

Jangan commit foto wajah nyata, embedding biometrik, data peserta yang dapat diidentifikasi, kata sandi, token perangkat, `.env`, ekspor database, atau data uji rahasia. Gunakan data sintetis untuk contoh publik. Buat `.gitignore` **sebelum** menambahkan kode dan data lokal.

**Jaringan backend (FL-010V, 16 Oktober 2026):** backend berjalan dengan **default loopback**; akses dari HP lewat **LAN privat** hanya boleh diaktifkan lewat konfigurasi eksplisit dan **wajib HTTPS dengan sertifikat yang dipercaya perangkat** — validasi TLS tidak boleh dimatikan (`InsecureSkipVerify` atau ekuivalen dilarang) dan Wi-Fi privat **tidak** dianggap cukup melindungi access/refresh token. **Jangan pernah membuka backend ke internet publik.**

## Status hak penggunaan

Belum ada keputusan lisensi untuk kode FaceLink AI. Repository publik tanpa berkas `LICENSE` **bukan** berarti kode bebas dipakai ulang. Setiap dependensi dan model pihak ketiga mempunyai ketentuan lisensi masing-masing.

**Mulai membaca:** `AGENTS.md` → `PROJECT_STATE.md` → `docs/PRD.md` → `docs/DECISIONS.md`.

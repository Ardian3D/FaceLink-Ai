# FL-ENR03 — Admin Enrollment Staging UI (Synthetic, Default-OFF)

Tanggal: 27 Oktober 2026. **STATUS: PARTIAL; acceptance UI Android pada perangkat fisik PASS untuk alur sintetis, dengan TalkBack/font scaling NOT_RUN karena tidak ada izin perubahan settings.**

Addendum R1 (27 Oktober 2026): acceptance dijalankan pada TECNO LJ7 setelah izin perangkat eksplisit, memakai APK staging `ai.facelink.enr02host` (tanpa izin CAMERA), backend HTTPS + SQLite disposable, akun serta JPEG pola sintetis. Dua defect UI yang dapat direproduksi diperbaiki (polling antrean menghapus halaman tambahan; UUID huruf besar ditolak validator keputusan).

Admin dapat meninjau antrean, detail identitas/roster, preview JPEG sintetis, dan mengirim keputusan melalui endpoint enrollment yang sudah ada. Tidak ada endpoint, migrasi, model biometrik, foto manusia, kamera, database Product Owner, atau perubahan auth/session/role. Main app tetap default-OFF.

## A. Baseline/worktree

- Branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Seluruh pekerjaan R5, FL-ENR00/00R, ENR01B, ENR02A/B/R1/R2, CAM00–03, CORE01/02, dan video dipertahankan. Tidak ada reset, stash, clean, commit, push, atau merge.
- Worktree saat audit akhir: 40 modified/58 untracked, mencakup pekerjaan lama yang dipertahankan dan berkas FL-ENR03. Tidak ada reset/stash/clean atau berkas pengguna yang dihapus.
- `cmd/server` tetap tanpa service enrollment. Main app tidak memiliki route `admin-enrollment` (diverifikasi `glob`), dan `.env` main tidak memuat flag Admin staging.

## B. Berkas perubahan

Frontend baru:
- `apps/mobile/src/features/adminEnrollment/{model,api,flow,ReviewSession,ProtectedPreview,staging,AdminEnrollmentScreen}.ts(x)`
- `apps/mobile/tests/adminEnrollment{Fixtures,Api.test,Flow.test,E2E.test}.ts`

Frontend lama (hanya integrasi/gerbang):
- `apps/mobile/app/(app)/_layout.tsx` — provider review hanya untuk Admin staging.
- `apps/mobile/app/(app)/admin.tsx` — satu QuickAction staging, desain dashboard tidak diubah.

Backend:
- `backend/internal/enrollment/{model,views,photos,service,service_test}.go` — DTO aditif, replay receipt sebelum eligibility, regresi.
- `backend/internal/httpapi/{enrollment_handlers,enrollment_test}.go` — dokumentasi/ kontrak aditif tanpa route baru.
- `backend/cmd/enrollment-staging-serve/{main,admin_fixtures,main_test}.go` — `-admin-fixtures`, `-web-root`, kontrol stdin, fixture sintetis, guard path statik.

Staging/dokumentasi:
- `research/fl-enr02a-host/prepare-enr02a.cjs`, `templates/admin-enrollment.tsx`.
- `README.md`, `backend/README.md`, `docs/{API_CONTRACT,BACKLOG,SECURITY_PRIVACY,TEST_PLAN}.md`, `PROJECT_STATE.md`, laporan ini.

Tidak berubah: OTP/session, role, auth middleware, SMTP/DNS, mobile enrollment siswa, `cmd/server`, schema migrasi.

## C. Schema

Tidak ada migration baru dan tidak ada perubahan schema. Seluruh uji memakai database SQLite disposable di direktori sementara (schema `7`, termasuk migrasi sintetis yang sudah ada); root dihapus saat proses berhenti. Database Product Owner tidak disentuh. Tidak ada tabel `face_templates` untuk manusia dan tidak ada data biometrik production.

## D. API

Tidak ada endpoint baru; dua belas route enrollment existing dipakai ulang. Perubahan hanya aditif pada wire DTO:
- Semua `EnrollmentView` memuat `server_now` dari jam service.
- Detail Admin memuat `consent`, `identity_binding` (akun live, person, NIM, status/role, referensi/verifikasi identitas nonbiometrik, kelas aktif), dan `artifact_integrity: NOT_CHECKED|UNAVAILABLE`.
- Item antrean memuat kelas dan `photo_expires_at`; envelope daftar mempertahankan `data/page/summary` dan menambah `server_now`.
- Detail Admin memuat `review` dan `history` (maksimal 50 event terpilih, metadata mentah dibuang).

Keputusan memakai kontrak existing: strong `If-Match`, `Idempotency-Key` UUIDv4, `confirmed:true`, dan `campus_check_ref` UUIDv4. Approve mengembalikan **202 `APPROVED_PROCESSING`**, bukan ACTIVE. Reject mengembalikan `REJECTED`; retry hanya untuk `FAILED_PROCESSING` dengan foto retained. UI tidak pernah menyimpulkan ACTIVE dari respons 202; status ACTIVE hanya ditampilkan bila backend mengirim `template_ready` dan consent masih aktif.

## E. Matriks otorisasi dan bukti

| Aktor / permintaan | Hasil yang dibuktikan |
|---|---|
| Admin sah, queue/detail/preview/approve/reject/retry | 200/202 sesuai domain; approve→ACTIVE hanya setelah worker memproses |
| Dosen dan mahasiswa | 403 sebelum lookup resource; tidak ada data detail/foto |
| Mahasiswa pemilik | Hanya `/me/*`; tidak dapat memakai route Admin |
| Dua reviewer bersamaan | Satu menang; lawannya 412 precondition; replay operasi pemenang 200 idempoten |
| Admin kehilangan role/status | 403/401 pada permintaan berikutnya; UI menutup data review |
| Subject menjadi tidak eligible setelah commit | Replay receipt 200 tetap tersedia; bukan 409 yang menyembunyikan keputusan |
| Subject unlinked | Detail menampilkan akun kosong/`Tidak tertaut`, bukan identitas karangan; approve tidak tersedia |
| Expiry 72 jam | Status EXPIRED, foto 410, approval terlambat ditolak |
| Cross-user preview | Ditolak backend; tidak ada URL publik/token pada URL |
| Sesi/token berakhir | 401, data sensitif dibersihkan, replay tidak membuat operasi baru |

Frontend hanya penjaga navigasi. Otorisasi sebenarnya tetap di Go; tombol nonaktif bukan pengganti RBAC.

## F. Pengujian dan hasil

| Perintah | Hasil |
|---|---|
| `go test -count=1 ./...` (backend) | 17 paket `ok`, 4 tanpa tes; termasuk replay ineligibility, detail akun tidak tertaut, dan path statik |
| `go vet ./...` | Lulus (exit 0) |
| `npm run typecheck` (mobile) | Lulus |
| `npm run lint` (mobile) | Lulus |
| `npm test` (mobile) | 14 file lulus, 2 skipped; **206 tes lulus**, 6 skipped |
| E2E HTTPS `FACELINK_ADMIN_ENR_E2E=1 … adminEnrollmentE2E.test.ts` | **5/5 lulus**: queue/detail/preview, approve202→worker ACTIVE→withdraw, concurrent reviewers, retry/failure, role/status denial, expiry |
| Stage `npm run typecheck` + `npm run lint` | Lulus |
| Stage web export | Lulus; 1093 modul |
| Gradle `app:assembleDebug` ARM64 offline | **BUILD SUCCESSFUL 11m48s**, 557 task |
| APK audit | `ai.facelink.enr02host`, 102262250 byte, SHA256 `07bd378d86b2b3789584b6a049ca939b5dfdcdd0b48ece0175acae7bb45afe32`; tanpa CAMERA/media/storage/overlay |
| Browser smoke | Login staging dan route `/admin-enrollment` tanpa sesi termuat; route memaksa login dan tidak menampilkan UI review |
| Default-OFF generator | `prepare-enr02a.cjs` tanpa flag menghapus route dan menulis `ADMIN_ENR_STAGING=0`; dengan flag mengembalikan route dan `1` |

E2E memakai proses Go nyata, TLS loopback dengan CA development melalui `NODE_EXTRA_CA_CERTS`, database disposable, dan login OTP normal (bukan token UI). Tidak ada PostgreSQL/Neon.

Smoke ekspresi TSX aktual (tanpa renderer): delapan kondisi visibility preview dan transisi polling revisi sama → kegagalan → pemulihan lulus. Ini bukan bukti UI Android.

### F.1 Acceptance perangkat Android (R1, TECNO LJ7, APK synthetic)

Izin perangkat eksplisit diberikan sebelum ADB; tidak ada izin perubahan TalkBack/font scale. Staging dijalankan dengan Metro lokal dan backend HTTPS disposable; setelah selesai staging di-force-stop, reverse/port/layanan/root dihapus, APK dan evidence dipertahankan. Settings HP dibandingkan baseline sebelum/sesudah (hanya `screen_brightness` sempat bergeser 2→4 oleh sistem dan dikembalikan ke 2; TalkBack tetap `0`, font scale `1.0`). APK utama `ai.facelink.mobile` tidak berubah (SHA256 identik).

| Skenario | Hasil |
|---|---|
| Cold start, login OTP Admin normal, dashboard, navigasi, logout | PASS; logout dengan access token valid mencabut tepat satu sesi server dan kembali ke layar login |
| Antrean: jumlah server, filter status, filter kosong, pencarian NIM nama/kelas, refresh manual | PASS |
| Offline (transport diputus pada relay loopback): label data basi, preview ditutup, keputusan dinonaktifkan | PASS |
| Detail: akun/person/NIM/roster/status, consent + timestamp, deadline 72 jam, reviewer/attestation, riwayat audit | PASS |
| Preview JPEG sintetis terotorisasi, polling revisi sama tanpa fetch ulang, background/resume menutup preview | PASS |
| Approve: konfirmasi, referensi UUID huruf besar, 202 `APPROVED_PROCESSING`, bukan ACTIVE; worker step → ACTIVE | PASS |
| Reject dengan alasan terstruktur; Retry `FAILED_PROCESSING` → processing → ACTIVE | PASS |
| Perubahan revisi eksternal saat form terisi: attestation direset, Approve dinonaktifkan, tidak ada commit kedua | PASS |
| Timeout 20 dtk setelah server commit: state ambigu dipertahankan, replay key/ETag/body sama → 200 tanpa duplikasi audit | PASS |
| 72 jam: `EXPIRED` + `PHOTO_RETENTION_EXPIRED`, foto 410, approve terlambat 409, token akses 401 dan UI sesi berakhir | PASS |
| RBAC: Dosen 403 pada queue/detail/preview/approve/users; Mahasiswa 403 pada empat endpoint Admin dan 200 pada `/me/consent`; route Admin tidak dimount untuk keduanya | PASS |
| Aksesibilitas terukur: kontrol ≥48dp (129px @432dpi), kontras teks putih pada `#D41115` 5,40:1, dialog urut judul→subjek→notice→batal→konfirmasi, teks nama 120 karakter/NIM 32 karakter terbungkus | PASS terbatas |
| TalkBack, font scaling, announcement audio | NOT_RUN (tidak ada izin perubahan settings) |

Dua instance disposable dipakai karena host restart menghentikan instance pertama; bukti dari kedua instance memakai APK dan source yang sama. Root fixture dihapus; bukti tersimpan di `research/fl-enr02a-host/evidence-local/enr03-r1/` (JSON ringkasan + screenshot tanpa data biometrik manusia).

### F.2 Perbaikan defect dari acceptance

1. **Polling 5 detik menghapus halaman antrean tambahan.** `refresh()` memuat ulang halaman pertama tanpa `append`; halaman yang sudah dimuat pengguna hilang. Perbaikan: polling otomatis hanya berjalan pada layar detail; antrean diperbarui lewat tombol refresh atau foreground-resume. Dibuktikan dengan flow aktual + backend HTTPS `limit=1` (dua halaman → tetap dua setelah callback polling).
2. **UUID huruf besar diterima form tetapi ditolak validator keputusan.** Form memakai regex case-insensitive, sedangkan `validDecision` memakai `UUID_V4` case-sensitive, sehingga tombol aktif lalu gagal dengan galat precondition yang menyesatkan. Perbaikan: referensi dikonversi ke huruf kecil saat menyusun intent konfirmasi; backend tetap menerima heksadesimal huruf besar. Dibuktikan pada perangkat: Approve 202 dan `campus_check_ref` tersimpan lowercase.

Percobaan awal yang gagal dicatat: fixture test memakai metode kampus yang tidak diterima validator; dua tipe fixture TypeScript; setup `__DEV__` unit; harness E2E salah menganggap startup `fixture_control:"stdin-jsonl"` sebagai ack; serta invalidasi preview melalui setState sinkron dalam effect ditolak lint. Semua diperbaiki sebelum run final. Batas SecureStore pada smoke login web tetap ada; bukan PASS UI terautentikasi.
Perintah export dengan `npm --prefix … exec` sempat memakai root repository dan ditolak karena `package.json` tidak ada di root; perintah dijalankan ulang dengan cwd staging dan export final lulus.

## G. Temuan keamanan dan perbaikan

Independent UI review menemukan empat isu dan closure review tidak menyisakan temuan pada versi yang diperiksanya. Penyesuaian preview akhir agar stabil saat polling serta pemisahan galat validasi diverifikasi melalui smoke ekspresi TSX dan regresi lokal, bukan acceptance renderer independen:
1. **Rotasi access token normal** tidak lagi dianggap logout. Controller memakai kredensial terbaru untuk operasi yang sama; provider dimiliki layout terautentikasi dan di-dispose saat user/role keluar.
2. **Polling 5 detik** tidak lagi membatalkan pembacaan lambat; refresh melewati operasi yang masih berjalan.
3. **Operasi ambigu** dipertahankan melewati blur/background (memory-only) dan hanya replay dengan key/ETag/body yang sama; kembali ke route memuat detail tanpa membuka keputusan baru.
4. **Preview** terikat pada submission/revision: polling dengan revisi sama mempertahankan tampilan; error, stale yang sudah menetap, expiry, background, role loss, atau perubahan revisi menutupnya. Pemulihan dari error tidak membuka preview otomatis.
5. **Penolakan server 400/422** tidak lagi diperlakukan sebagai hasil ambigu; pending dibersihkan dan Admin dapat memperbaiki setelah refresh. Respons 2xx dengan receipt rusak tetap `unknown`/ambigu dan hanya boleh di-replay dengan operasi yang sama.

Independent backend review menemukan dan menutup:
- Replay keputusan yang sudah commit tidak lagi gagal 409 hanya karena subject kemudian suspended/unlinked.
- Filter "Semua status" tidak mengirim `status=` kosong yang ditolak backend.
- Segmen path statik Windows dengan trailing dot/space ditolak sebelum `os.Stat`, dengan pemeriksaan `filepath.Rel`.
- Detail submission dengan akun tidak tertaut dapat dibuka tanpa mengarang UUID/status akun.

Batas yang tetap ada:
- Preview native memakai data URI; menurut source RN/Fresco, data URI tidak membuat entry disk-cache, tetapi bitmap tetap berada di bitmap memory cache Fresco dan tidak ada API JS untuk evict satu gambar. Ini **bukan klaim zeroization**; hanya synthetic.
- Transport native tanpa streaming tidak dapat membuktikan server yang memalsukan `Content-Length` tidak membuffer lebih dari 1 MiB sebelum pengecekan; panjang aktual tetap ditolak setelah diterima.
- Web tidak mendukung penyimpanan sesi `expo-secure-store`; login web memang ditolak oleh desain existing, sehingga UI terautentikasi tidak dapat diuji di browser tanpa bypass yang dilarang.
- D-030 dan residual ENR02B-R2 tidak berubah.

Temuan acceptance perangkat pada ENR03-R1 (snapshot historis; F1/F2 ditutup pada FL-STAB01, F3 tetap terbuka):
- **F1 (P2, auth existing).** Logout setelah access token kedaluwarsa tidak mencabut sesi server. `POST /auth/logout` memakai access token; setelah clock maju 72 jam responsnya 401, token lokal dihapus, tetapi baris sesi (refresh masih berlaku sampai batas absolut 30 hari) tetap tidak revoked. Logout dengan access token valid terbukti mencabut tepat satu sesi. Perbaikan menyentuh alur auth/session sehingga memerlukan keputusan terpisah.
- **F2 (P2, login existing).** Setelah beberapa siklus login/logout cepat, layar login menampilkan redbox `VideoPlayer.pause` ditolak karena shared object sudah dilepas (`expo-video`). Cold restart memulihkan sesi dan login berikutnya berhasil; bukan regresi route Admin dan tidak menghalangi acceptance.
- **F3 (P3, UI Admin staging).** Tombol konfirmasi dialog dapat sesaat nonaktif saat refresh detail 5 detik berjalan (fail-closed, pulih otomatis); dua percobaan tekan pertama tercatat tidak dieksekusi.

**Penutupan FL-STAB01 (27 Oktober 2026):** refresh-proof logout kini mencabut sesi
server setelah access expiry; offline failure dilaporkan jujur. Lifecycle video
membatalkan animasi dan ownership sebelum Expo release. Reproduksi sebelum/
sesudah, regresi dan acceptance TECNO tercatat di
[`FL_STAB01_TECHNICAL_REPORT.md`](FL_STAB01_TECHNICAL_REPORT.md).
Tidak ada perubahan F3, role/OTP design, biometrik atau izin production.


## H. Aksesibilitas dan UX

- Tombol/aksi baru minimal 48dp; input dan pilihan memiliki label aksesibilitas, role radio/checkbox, dan status `disabled/busy`.
- Pesan galat memakai `accessibilityRole="alert"` dan live region; status keputusan memakai live region.
- Warna aksi utama memakai `accentPressed` existing dengan teks putih: kontras luminansi terhitung **5,40:1** (`#D41115`) dan **7,20:1** (`#B00E12`); bukan perubahan palet global.
- Perangkat (R1): tombol aksi terukur 129px pada 432dpi = tepat 48dp; kontras piksel native teks putih pada tombol primer **5,40:1**; dialog konfirmasi mengumumkan urutan judul→subjek→notice→batal→konfirmasi; nama 120 karakter dan NIM 32 karakter terbungkus penuh tanpa pemotongan.
- Teks selalu menyebut fixture sintetis dan membedakan review/attestation dari verifikasi manusia atau ACTIVE.
- **NOT_RUN:** TalkBack, font scaling besar, dan announcement audio pada perangkat — tidak ada izin perubahan settings pada tugas ini. Fokus keyboard web juga belum diuji.

## I. Build dan smoke web

- Web export final dari staging berhasil (1093 modul). Server Go menyajikan export pada origin HTTPS yang sama dengan `/api/v1`; tidak ada bypass API.
- Browser memuat judul `FaceLink ENR02A Staging`, layar "Continue with FaceLink", dan route `/admin-enrollment` tanpa sesi kembali ke login. Tidak ada UI review yang tampil tanpa sesi.
- Login OTP web berhenti pada pesan penyimpanan aman karena `expo-secure-store` tidak tersedia di web. Ini perilaku existing, bukan regresi dan bukan alasan menambah bypass.
- APK debug staging dibangun dan (pada R1) **diinstal/dijalankan** di TECNO LJ7; hash APK terpasang sama dengan artefak yang diaudit. Debug build memuat JS dari Metro.
- Cleanup R1: staging di-force-stop, reverse `tcp:8089`/`tcp:8091` dihapus, Metro/relay/backend tugas dihentikan, root fixture disposable dan tooling sementara dihapus; APK, source staging, evidence, dan settings HP dipertahankan (brightness dikembalikan ke baseline `2`).

## J. Pengujian perangkat / hasil

Izin perangkat eksplisit diberikan sebelum semua ADB; izin TalkBack/font scale terpisah **tidak** diberikan, jadi keduanya tetap NOT_RUN.

Dijalankan dan lulus (TECNO LJ7, APK `ai.facelink.enr02host`):
- install/launch APK, login OTP Admin/Dosen/Mahasiswa dengan akun sintetis;
- antrean/filter/pencarian/empty/offline, detail identitas, preview, Approve/Reject/Retry, replay timeout, expiry 72 jam, RBAC, logout;
- background/resume (preview tertutup, tidak membuka ulang otomatis) dan cold restart;
- wrapping nama/NIM panjang serta pengukuran 48dp/kontras.

Tetap NOT_RUN: TalkBack, font scaling, rotasi paksa, screenshot recents, dua reviewer pada dua perangkat fisik (konkurensi diuji lewat dua sesi API + satu UI), dan cold restart saat operasi ambigu (pending bersifat memory-only; backend tetap otoritatif).

Web authenticated UI juga **NOT_RUN** karena keterbatasan penyimpanan sesi di atas.

## K. Blocker dan residual

1. **Acceptance perangkat dasar selesai.** Residual: TalkBack/font scaling belum diuji karena tidak ada izin perubahan settings; itu bukan blocker staging sintetis.
2. `CAMPUS_ROSTER_IN_PERSON` masih ditolak validator backend existing; UI hanya menawarkan `CAMPUS_AUTHORIZED_CHECK`. Dukungan metode kedua adalah keputusan produk/domain, bukan perubahan senyap.
3. Fresco bitmap memory cache tidak dapat dievict dari JS; preview tetap synthetic-only.
4. Operasi ambigu hilang bila proses aplikasi mati (memory-only). Setelah cold start tidak ada replay otomatis; backend tetap otoritatif dan UI harus memuat status, bukan membuat key baru.
5. `/healthz` dan bentuk galat lama di luar scope tidak diubah.
6. APK debug memerlukan Metro; belum ada bukti APK standalone atau instalasi release.
7. F1 (logout setelah access token kedaluwarsa tidak mencabut sesi server) memerlukan keputusan perbaikan auth terpisah; F2 (crash `VideoPlayer.pause` pada layar login) dan F3 (tombol konfirmasi sesaat nonaktif saat refresh) dicatat sebagai temuan, bukan diperbaiki di tugas ini.

## L. GO/PARTIAL/NO-GO

**PARTIAL (staging synthetic) dengan acceptance UI Android PASS.** Implementasi, unit/controller, HTTPS E2E dengan Go nyata, build staging, static-host guard, dan acceptance perangkat sintetis selesai. **GO** untuk melanjutkan ke perencanaan FL-ENR04 hanya setelah keputusan Product Owner; tidak ada start otomatis.

**NO-GO:** enrollment production, foto/wajah manusia, template YuNet/SFace production, database Product Owner, perubahan OTP/session/role, endpoint baru, migrasi biometrik, commit/push/merge otomatis, atau deployment. **STOP** setelah laporan; FL-CORE02/live enrollment tidak dimulai otomatis.

## M. Keputusan yang dibutuhkan

1. Apakah metode attestation `CAMPUS_ROSTER_IN_PERSON` perlu didukung backend? Saat ini hanya `CAMPUS_AUTHORIZED_CHECK` yang benar-benar diterima.
2. Apakah F1 (revoke sesi saat logout setelah access token kedaluwarsa) akan diperbaiki pada tugas auth terpisah? Perubahan menyentuh OTP/session yang dilarang pada tugas ini.
3. Apakah F2 (crash `VideoPlayer.pause` di layar login) dan F3 (tombol konfirmasi sesaat nonaktif) dijadwalkan sebagai perbaikan UI terpisah?
4. Apakah preview enrollment production nanti memerlukan jaminan cache native yang lebih kuat daripada data URI/Fresco memory cache; itu membutuhkan desain renderer tersendiri dan tidak boleh diklaim dari uji ini.

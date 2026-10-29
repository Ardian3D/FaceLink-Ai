# FL-GIT01 — Source Preservation & GitHub Integration Preparation

**Status: preservation source terverifikasi; publikasi HOLD / menunggu persetujuan.** Tidak ada staging, commit, push, PR creation, merge, branch/worktree Git baru, uninstall atau ADB pada FL-GIT01. FL-ENR06-R1 tetap PASS untuk main synthetic Android E2E; crash FL-STAB02 tetap UNRESOLVED, bukan fixed.

Laporan ini mencatat baseline dan rencana, bukan izin mengunggah seluruh workspace. Daftar exact-file berada di [`FL_GIT01_PUBLICATION_MANIFEST.json`](FL_GIT01_PUBLICATION_MANIFEST.json). Klasifikasi aman berarti tidak ditemukan blocker pada screening lokal yang dilakukan, bukan sertifikasi keamanan, lisensi atau kesiapan production.

## A. Git baseline aktual

- Git tidak tersedia melalui PATH. Executable existing `%LOCALAPPDATA%/Temp/facelink-enr04-tools-IwO0S9/git/cmd/git.exe` benar-benar dijalankan: `git version 2.56.0.windows.4`. Tidak menginstal Git atau mengubah konfigurasi global.
- Branch aktif `feat/fl-otp01-email-auth`; HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`; branch aktif tidak memiliki upstream.
- Baseline awal: **60 modified tracked + 218 untracked = 278 changed/untracked candidates**. Total visible files:221 tracked +218 untracked =439. Index bersih; tidak ditemukan tracked deletion atau rename dalam diff yang diperiksa.
- Ada20 local branches. Seluruh nama, SHA dan cached upstream/tracking status dicatat dalam manifest. Local `main` masih menunjuk `dca341ab9beb35b0c1820336d0e836a2f6a744b8`; bukan representasi remote main terbaru. Cached upstream refs tidak disamakan dengan branch remote yang masih ada.
- Remote fetch/push aktual: `https://github.com/Ardian3D/FaceLink-Ai`, tanpa credential pada URL yang dilaporkan. Tidak mengubah remote/config.
- `git fetch --no-tags origin main` berhasil; hanya memperbarui metadata/ref fetch, bukan worktree atau local main. Tidak pull/reset/clean/stash/rebase/force-push.

## B. Status remote dan PR #19

Verifikasi baru melalui [GitHub API PR #19](https://api.github.com/repos/Ardian3D/FaceLink-Ai/pulls/19) dan fetch/ls-remote lokal:

- [PR #19](https://github.com/Ardian3D/FaceLink-Ai/pull/19): `merged=true`, closed, merged_at `2026-10-26T20:33:58Z`.
- Merge commit dan fetched `origin/main`: `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`.
- PR source head: `aa36f2d243f8761879baf38a5b2735afab8f1b52`; sudah ancestor HEAD lokal.
- Merge parents: `c1ba1ec789e2aa5fdade793fd898caefdb816a28` dan PR source head di atas. Tree merge PR identik dengan tree PR source head.
- `HEAD...origin/main`: **21 local-only commits /1 remote-only merge commit**; merge-base adalah PR source head. Seluruh21 local-only commits bertanda `+` pada patch-equivalence check terhadap main.
- `ls-remote --heads --tags origin` hanya mengiklankan `refs/heads/main`; tidak ada advertised ref lain yang mengandung21 commit tersebut saat pemeriksaan. Ini tidak membuktikan commit tidak pernah pernah tersalin pada clone/ref privat atau ref yang sudah dihapus.

**Implikasi:** source PR #19 sudah berada dalam ancestry lokal. Jangan merge PR #19 lagi atau menimpa main untuk mengatasi angka divergence. Remote-only commit tersebut adalah merge-history, bukan bukti seluruh source lokal sudah terintegrasi.

## C. Perubahan lokal yang belum terintegrasi

- Dibanding fetched main:90 tracked paths berbeda, ditambah218 untracked paths. Terdapat perubahan lokal yang sudah committed, sehingga daftar dirty files saja tidak cukup untuk publikasi.
- Tambahan9 hand-authored source yang di-ignore menghasilkan **317 exact paths dalam delta publikasi awal**. Manifest juga mencakup dua deliverable FL-GIT01 baru, sehingga totalnya319 paths; angka ini bukan perintah stage semuanya.
- Area: OTP/auth/session dan operations; CORE01/02 people/roster/manual attendance; backend synthetic enrollment/processing/crypto AADv2; native capture package dan camera research; Python IPC/model integration; mobile enrollment/review/staging; tests, contracts dan laporan.
- Dependency manifests/locks yang diperiksa (`backend/go.mod`, `go.sum`, mobile `package.json`/`package-lock.json`, Python `requirements.txt`) sama dengan HEAD maupun main. Tidak menambahkan dependency pada tugas ini.
- Catatan dependency metadata: source secure-storage baru mengimpor `golang.org/x/sys` langsung, sementara `go.mod` existing masih menandainya indirect. Versinya sudah tersedia dan restored-package test/vet lulus; tidak menjalankan tidy, mengganti versi atau diam-diam menambah manifest ke commit plan.
-9 source tersembunyi: tiga Kotlin bridge `research/fl-cam00-proof/plugins/android/` dan enam source template `research/fl-cam03-host/templates/`. Path tepat tercantum dalam manifest dengan `hidden_by_ignore=true`. Penyebab: nested `android/` rule dan root `templates/` rule. Source tersebut sudah disalin/hash-verified ke snapshot privat; ignore rules original belum diubah.
- Orphan `research/fl-enr02a-host/stage/src/features/enrollment/EnrollmentEntryCard.tsx` tidak dibaca current prepare script dan tidak direferensikan; ikut snapshot stage privat, bukan source yang diasumsikan dapat diregenerasi atau calon publikasi.
- Inventaris first-party/ignored/artefak mencatat1553 file. Sebanyak29 dependency/cache/generated-intermediate trees diringkas, bukan seluruh node_modules/venv/build-object dienumerasi. APK output directories diperiksa terpisah. Ada47 binary/large-artifact entries berjumlah1.83GB dalam cakupan katalog; bukan ukuran keseluruhan workspace.

## D. Source aman untuk GitHub

Manifest exact-file memisahkan:

| Klasifikasi delta beserta deliverable FL-GIT01 | Jumlah | Perlakuan |
|---|---:|---|
| `source-safe-screened` |295| Boleh menjadi kandidat setelah seluruh gate integrasi selesai; bukan izin stage/push saat ini |
| `manual-review` |23| Tahan sampai redaksi/provenance/ignore-policy/portabilitas/whitespace selesai ditinjau |
| `private-as-is` |1| Raw device baseline tidak boleh di-stage; hanya derivative tersanitasi yang dapat dipertimbangkan |

Source domain/key-management bukan material kuncinya. SQL migration/test code bukan database/embedding. Synthetic test literals bukan credential nyata. Tidak menambahkan seluruh direktori ignored dengan `git add -f`.

Tetap privat: `.env`, SQLite/WAL/SHM/journal, TLS private keys dan material DPAPI, model weights, APK/JAR/AAR/EXE, local video/design assets, generated staging/Android trees, raw device logs/tombstones/screenshots dan `evidence-local/`. Dua video lokal memuat orang yang dapat dikenali; bukan publication fixtures. Katalog/backup privat tidak boleh di-upload bersama source.

## E. Temuan secret/privacy

### Disclosure terkonfirmasi

1. **Identitas akun nyata dan riwayat autentikasi pada dokumentasi.** `PROJECT_STATE.md` dan `docs/EMAIL_OTP.md` memuat mailbox, UUID/status akun serta timestamp dari pengamatan database/login nyata. `scripts/test-email-headers.ps1` memakai mailbox penerima nyata dalam fixture sintetis. Raw values tidak disalin ke laporan ini. Original disimpan privat; rencana publikasi harus memakai reserved-domain identities, UUID rekaan dan ringkasan waktu non-identifying.
2. **Disclosure historis sudah publik.** Satu mailbox akun nyata ada pada `PROJECT_STATE.md`, blob `2526359ebb0120e2696c8ff5adae92baf2c36894`, line93 pada versi tersebut, diperkenalkan commit `aa36f2d243f8761879baf38a5b2735afab8f1b52` yang terjangkau origin/main. Identitas operasional juga ditemukan pada33 versi blob dalam cakupan riwayat yang dicocokkan. Redaksi versi terbaru atau `.gitignore` tidak menghapus history/clone/cache. Pemilik perlu menilai privacy containment historis secara terpisah; tidak melakukan history rewrite/force-push.
3. **Identifier perangkat dan workstation.** Raw `research/fl-cam03-host/evidence/device-baseline.json` serta `research/fl-enr06r1-host/device.cjs` mengandung identifier fisik yang sama; helper dan beberapa laporan memakai path workstation/profil Windows. `docs/ANDROID_SETUP.md` juga memuat nama profil lokal pada baseline line104. `device-install.json` memuat path instalasi perangkat. `research/fl-cam03-host/release-audit.json` ditahan untuk normalisasi absolute build paths; path tersebut tidak mengandung nama profil dan bukan temuan secret. Tahan raw baseline; gunakan input operator/environment dan metadata tersanitasi pada salinan publikasi.
4. **Provenance JPEG tertanam belum ditutup.** `apps/mobile/src/features/enrollment/fixture.ts` dinyatakan sebagai pola sintetis oleh source, tetapi audit tidak menemukan generator/pin yang membuktikan provenance default JPEG tersebut. Ini bukan temuan bahwa gambar manusia/secret, dan bukan fixture procedural mannequin FL-ENR04-R2 yang berbeda. Perlu konfirmasi sebelum publikasi bytes tertanam.

### Secret scan dan batasnya

- Local-only heuristic scan mencakup1095 reachable Git objects,616 blobs,601 text blobs, serta439 current visible files. Tidak mengirim isi file ke scanner eksternal. Scope sejarah adalah reachable local refs dan fetched main; bukan jaminan terhadap unreachable objects, ref GitHub yang tidak teradvertise, clone, cache atau secret yang tidak cocok pola.
- Tiga JWT-shaped hits historis/public adalah static redaction-test fixtures di accesslog/logging tests; bukan token sesi nyata. Satu credentialed-URL hit current adalah negative loopback URL fixture, bukan credential operasional.
- **Tidak ditemukan live JWT/session/refresh token, SMTP password, API/HMAC/encryption key atau private key yang terkonfirmasi telah di-commit/publikasikan dalam cakupan ini.** Tidak menyimpulkan ketiadaan seluruh secret.
- Keberadaan `%LOCALAPPDATA%/FaceLink/secrets/{smtp-auth.secret,smtp.config.json,email-otp.key}` diperiksa metadata saja. Isi tidak dibaca, didekripsi atau disalin. SQLite owner, private keys dan `.env` tidak diekspor ke snapshot publikasi.
- **Tidak melakukan rotasi.** Disclosure identitas bukan bukti credential compromise. Jika review selanjutnya mengonfirmasi credential pernah terunggah, credential harus dianggap bocor: containment/rotasi dengan izin pemilik, bukan sekadar ignore atau penghapusan commit terbaru.

Review security dan pemetaan integrasi independen dilakukan read-only terhadap publication candidates, wiring dan hidden source; hasilnya menjadi dasar hold. Tidak mengklaim sertifikasi security/production.

## F. Preservation status

Snapshot lokal terpisah: `%LOCALAPPDATA%/FaceLinkAI/preservation/FL-GIT01-2026-10-29T17-34-46-259Z/`. ACL directory dilindungi dari inheritance dan diverifikasi hanya memberi FullControl kepada current Windows user serta SYSTEM, sebelum menulis snapshot. Bukan enkripsi atau backup off-device.

-439 visible source/metadata files disalin byte-for-byte;9 ignored hand-authored source diselamatkan terpisah dan ditempatkan dalam restore tree. **448 primary source files** diverifikasi SHA-256 pada salinan restore.
-289 generated Android/stage source tambahan disalin/hash-verified **privat**, bukan calon publikasi. Total737 source copies; manifest terpisah untuk source utama, hidden source dan generated source.
- `repository.bundle` dibuat dengan `--all`, lalu `git bundle verify` exit0. Seluruh reachable local refs/commits tetap dipertahankan tanpa mengubah branch. Bundle privat dapat mengandung historical identity disclosures: jangan upload atau push `--all`.
- Source manifest, baseline branches/status, redacted history audit, workspace catalog, test output dan restore proof disimpan di directory privat yang sama.
- Delapan protected artefacts dari baseline STAB02 (APK utama/tiga staging-backup APK, dua video, dua model) hash-nya tetap. Artefak/evidence lain tetap di lokasi original; tidak dihapus.
- Database aktif/sidecars, keys dan binary besar **tidak dibackup sebagai salinan data**; tidak membuat klaim consistent SQLite backup atau full disaster recovery. Original tetap utuh. Dependency caches dapat dibangun ulang dan tidak diduplikasi.
- Tidak ada source product yang diubah untuk audit ini. Perubahan repository FL-GIT01 terbatas pada laporan/manifest baru dan header status README/PROJECT_STATE; final document supplement disalin ke snapshot setelah penulisan.

## G. Regression

Hash seluruh438 files baseline STAB02 dibanding current source sebelum laporan FL-GIT01: hanya empat status docs yang sudah diubah pada STAB02 berbeda, serta satu laporan STAB02 baru. Tidak ada perubahan code/test/dependency sejak baseline tersebut. Karena itu tidak menjalankan ulang seluruh suite tanpa alasan.

**[INFERENCE] Reuse hasil full suite historis** mengandalkan laporan tidak adanya perubahan source antara run FL-ENR06-R1 dan snapshot STAB02, lalu pembandingan hash baru. Snapshot tersebut dibuat sesudah run, **bukan test-time per-file hash record**. Karena itu tabel historis bukan klaim seluruh suite baru dijalankan/lulus pada FL-GIT01; bukti runtime baru dibatasi combined2/2 dan restored AAD-swap test/vet package.

| Pemeriksaan | Bukti dan status |
|---|---|
| Go tests + vet | Reuse hasil FL-ENR06-R1:18 paket PASS/4 tanpa tes, test+vet exit0; source/test/migration hashes relevan cocok baseline |
| Python | Reuse hasil full suite217 PASS/1 SKIP pada source yang tidak berubah |
| Mobile tests/typecheck/lint | Reuse229 PASS/9 SKIP + typecheck/lint exit0; opt-in236 PASS/2 SKIP dan combined terpisah2/2 tetap dilaporkan terpisah |
| Auth/session regression | Reuse Go/session + mobile/auth hasil di atas; controller terarah STAB02 juga85/85 PASS |
| AAD v2 regression | Reuse full crypto/security/domain suite; **fresh restored-snapshot cross-record-swap regression PASS dan vet package exit0**; integrated smoke juga memverifikasi envelope2/binding valid |
| Synthetic integrated smoke baru | **2/2 PASS**, explicit Node executable/environment, actual TS→HTTPS Go→real Python→ACTIVE/AADv2→withdrawal/cleanup; bukan Android UI |
| Response-loss baru | Uncertain TS state, exact body/key/ETag replay202→200, approval audit bertambah1 lalu0 saat replay, cleanup0 |
| Snapshot restore/bundle |448 primary source hashes cocok; bundle verify exit0;289 generated source juga dicocokkan |
| Git whitespace | **FAIL/HOLD**: EMAIL_OTP blank EOF, TEST_PLAN blank EOF, detector patch trailing whitespace; belum diperbaiki pada original |
| Sensitive review | HOLD files/identities/provenance sesuai D/E; tidak sama dengan clean secret-scan certificate |

Smoke baru: `FACELINK_ENR05_E2E=1`, `NODE_EXTRA_CA_CERTS=<local dev CA>`, explicit Node menjalankan `apps/mobile/node_modules/vitest/vitest.mjs run tests/enrollmentCombinedE2E.test.ts`. Trace `e93b17ce-3ea0-4180-96d5-9c7cff6aef36`; evidence `research/fl-enr02a-host/evidence-local/enr05/combined-e93b17ce-3ea0-4180-96d5-9c7cff6aef36.json`. `child_eof=true`, `root_removed=true`; foto/template/active/ciphertext0 sesudah withdrawal/reconciliation. Tidak ada ADB, kamera, akun owner atau owner DB digunakan.

Restore runtime proof baru, dijalankan dari private `worktree/backend`, bukan original workspace: `go test ./internal/enrollmentsecure -run '^TestTemplateRejectsSamePersonDifferentRecordSwap$' -count=1 -v` PASS dan `go vet ./internal/enrollmentsecure` exit0. Ini menguji penolakan transplant record berbeda pada source hasil restore; bukan klaim full suite baru atau fresh public clone tanpa prerequisites.

Percobaan wrapper pertama salah memilih `process.execPath` yang ternyata `omp.exe`, bukan Node; hanya mencatat `Working...` lalu worker timeout. Tidak dihitung sebagai bukti pengujian. Invocation explicit Node berikut adalah hasil2/2 di atas. Tidak menganggap timeout tooling sebagai kegagalan aplikasi.

Whitespace detail pada baseline: `docs/EMAIL_OTP.md:337`, `docs/TEST_PLAN.md:753`, `research/fl-cam00-proof/plugins/detector/patch-detector.cjs:90`. Git `--no-index --check` exit1 dengan output kosong berarti diff normal, bukan whitespace failure; satu untracked file benar-benar melaporkan trailing whitespace. LF→CRLF warnings dicatat, bukan dianggap kehilangan source atau alasan mengubah config global.

Batas reproduksi publik: weights, dev CA/private key, local video dan JPEG positive fixture tidak ikut repository; perlu provisioning lokal berizin. Jangan mengklaim fresh clone otomatis menjalankan opt-in real-model smoke tanpa prerequisites. Perubahan sanitasi/ignore/source setelah approval harus diuji sesuai dampaknya; hasil lama tidak otomatis mencakup perubahan tersebut.

## H. Usulan branch dan kelompok commit

**Rekomendasi:** new branch `integration/fl-git01-source-preservation` dari fetched origin/main pada **worktree terpisah**, bukan checkout/reset dirty worktree. Belum dibuat. Salin source terkini yang telah diaudit/disanitasi menurut manifest; jangan salin ignored trees secara massal.

Tradeoff eksplisit:21 original local commits tetap utuh di branch asal dan restricted bundle, tetapi ancestry tersebut tidak dimasukkan ke branch publikasi. Ini **source-export integration**, bukan squash/rebase/history rewrite. Alasan: commit history lokal juga memuat operational identity disclosures; sanitasi tip lalu push seluruh branch asal tetap mempublikasikan versi lama. Jika pemilik memerlukan ancestry21 commit ikut publik, perlu review/keputusan berbeda sebelum publikasi; jangan diam-diam mengganti strategi.

Kelompok logis berurutan, path tepat di manifest:

1. `01-ignore-policy`: pengecualian exact-file untuk sembilan source, tanpa membuka data/build trees.
2. `02-python-runtime`: IPC/pipeline, diagnostic/procedural-fixture scripts, tests dan README terkait.
3. `03-backend-otp-contracts`: OTP operations + CORE/manual + synthetic enrollment/AADv2, seluruh shared router/domain changes, tests, migrations dan canonical API/data/security contracts. Jangan memecah file shared menjadi pseudo-commit historis yang tidak compile.
4. `04-native-camera-hosts`: native package, CAM research/bridge/templates dan metadata evidence yang lulus review. Raw private baseline tidak masuk.
5. `05-mobile-and-enrollment-staging`: auth/session/video, enrollment/review consumers dan tests bersama ENR prepare/templates/host tooling yang diperlukan; tidak memisahkan test consumer dari prepare script-nya.
6. `06-status-reports`: laporan, README/status/backlog/test-plan dan manifest FL-GIT01 yang tersanitasi.

Kelompok ini bukan izin membuat sebagian branch yang kehilangan dependency/fixture penting. Tutup semua publication holds terlebih dahulu; verifikasi intermediate commit sesuai scope dan full integrated final tree sebelum push.

**Invariant yang diperiksa:** `cmd/server` tidak mengisi dependency Enrollment; mobile workflow memerlukan explicit staging flags, route enrollment hanya dihasilkan dalam staging tree. Migrations00007/00008 tetap di `backend/synthetic-migrations/` dan hanya disusun `enrollmentfixture.PrepareMigrations` ke fresh disposable directory.00005/00006 adalah nonbiometric normal migrations; publikasi source bukan izin menjalankannya pada DB pemilik. Tidak ada threshold/model/policy change pada FL-GIT01.

## I. Rencana push/PR aman dan approval gate

**STOP sebelum commit/push/PR creation/merge.** Rencana sesudah persetujuan terpisah:

1. Tutup23 manual reviews, raw-private evidence handling, default embedded-fixture provenance dan tiga whitespace findings pada **salinan integrasi**, original tetap preserved. Ganti hardcoded device/workstation inputs tanpa ADB. Terapkan hanya sembilan source exceptions; update exact manifest/hash sesudah redaksi.
2. Re-fetch main dan verifikasi remote tidak bergerak; buat separate worktree/branch dari main. Tidak pull berisiko, merge main otomatis, reset atau stash pada original.
3. Copy audited source delta yang benar terhadap main, termasuk OTP code yang sudah committed lokal tetapi belum di remote. Jangan copy `.git`, private config/data/keys/binaries/evidence-local. Jangan stage raw-private baseline atau mempublikasikan original bundle/history.
4. Stage exact allowlist, review staged diff dan file list/size, ulang local sensitive/whitespace checks serta impacted regressions; verifikasi production enrollment default-OFF dan synthetic migration isolation.
5. Setelah commit approval: buat kelompok commit yang ditinjau. Sesudah push approval, push **hanya** `HEAD:refs/heads/integration/fl-git01-source-preservation` dari worktree baru dengan upstream yang tepat; bukan `--all`, force push atau main.
6. Sesudah PR-creation approval, buat PR ke main, jelaskan source-export provenance, tests, private prerequisites, historical privacy finding dan crash UNRESOLVED. Tidak merge otomatis.

**Persetujuan yang diminta sekarang:** izinkan persiapan worktree/branch terpisah, source-export (bukan ancestry21 commit), sanitasi salinan/portable helper inputs, narrow ignore exceptions, whitespace cleanup, provenance review dan regresi terdampak. Persetujuan persiapan ini **tidak** memberi izin commit/push/PR/merge; tindakan tersebut tetap memerlukan keputusan eksplisit berikutnya. Historical public privacy containment merupakan keputusan terpisah; tidak melakukan rotasi atau history rewrite secara otomatis.

## J. Kandidat APK cleanup dan risiko

**Tidak ada inventaris perangkat baru.** Daftar berikut adalah referensi historis FL-STAB02, bukan bukti package masih terpasang saat FL-GIT01.

| Package historis | Rencana dan risiko |
|---|---|
| `ai.facelink.mobile` | Wajib dipertahankan; jangan uninstall, clear-data atau install-over |
| `ai.facelink.enr02host` | Kandidat sesudah kebutuhan diagnosis selesai; uninstall menghapus sesi/preferences/cache dan mungkin evidence privat |
| `ai.facelink.enr06admin` | Kandidat sesudah retensi/evidence review; dampak data lokal sama |
| `ai.facelink.stab02release` | Kandidat setelah pembanding tidak diperlukan; non-debuggable private data belum diekspor |
| `ai.facelink.cam03host` | Prototype lama; data/evidence privat belum diperiksa atau dibackup |
| `ai.facelink.cam00proof` | Prototype lama, pernah mendeklarasikan CAMERA; tidak diluncurkan pada FL-GIT01, data/evidence belum diperiksa |

Sebelum cleanup di tugas lain: minta izin perangkat baru, inventaris ulang APK/package/versi/hash, tentukan retensi/backup data privat dan evidence, lalu minta approval exact packages serta dampak uninstall. APK host bukan backup private data aplikasi. **Tidak ADB/uninstall/clear-data pada FL-GIT01.**

## Penutupan

Source sudah diamankan secara lokal; publikasi belum layak dieksekusi tanpa menutup privacy/provenance/whitespace gates dan persetujuan strategi. Tidak menyatakan source sudah masuk GitHub atau crash sudah fixed. Tidak memulai fase baru otomatis.

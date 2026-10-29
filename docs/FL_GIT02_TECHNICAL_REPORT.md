# FL-GIT02 — Audited Source Integration Preparation

Tanggal: 30 Oktober 2026. **READY untuk review Product Owner; BELUM DIIZINKAN untuk staging/commit/push/PR/merge.** Ini kesiapan source terintegrasi, bukan sertifikasi keamanan, production readiness, atau closure crash FL-STAB02.

## A. Baseline dan preservasi

- Workspace asal: branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`; baseline aktual FL-GIT02: **60 tracked modifications, 220 untracked**. Angka 218 pada laporan FL-GIT01 adalah snapshot sebelum dua deliverable FL-GIT01 ditambahkan.
- Worktree terpisah: `C:/visual-studio-code/facelink-ai-integration-fl-git01`; branch `integration/fl-git01-source-preservation` dari fetched `origin/main` `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`. Upstream otomatis telah dilepas agar tidak mengarah ke main.
- Fresh fetch dan API resmi mengonfirmasi [PR #19 merged](https://github.com/Ardian3D/FaceLink-Ai/pull/19). Source PR sudah menjadi ancestor original; tidak di-merge ulang. **21 commit lokal original tidak disalin sebagai ancestry**: worktree integrasi tetap pada base main dan belum memiliki commit baru.
- Hash **441 file original**, **9 source tersembunyi**, dan **786 file backup FL-GIT01** tidak berubah. Branch/HEAD, record status, dan hash index original juga tidak berubah. Perbandingan status menormalkan delimiter NUL baseline vs newline output akhir, bukan mengabaikan perubahan.
- Backup FL-GIT01 tetap restricted di luar repository; bukan sasaran redaksi. Tidak reset/stash/clean/rebase/pull berisiko atau menghapus artefak. APK, model, video dan evidence lama tetap pada original.

## B. Ekspor exact-file

Manifest awal memuat 319 kandidat: 295 source-screened, 23 manual-review, 1 raw-private. **318 berkas diekspor**, seluruh 23 review ditutup, dan satu raw baseline privat dikecualikan. Sembilan source ignore sudah termasuk hitungan 318, bukan tambahan ganda.

Dua deliverable FL-GIT02 baru membuat **320 included paths**; manifest final berisi 321 record termasuk satu pengecualian privat. Tidak menggunakan git add, wildcard export, atau stage-all. Manifest ini adalah daftar review, **bukan izin staging otomatis**.

| Kelompok source awal | Berkas included |
|---|---:|
| Ignore policy | 2 |
| Python runtime | 11 |
| Backend/OTP/contracts | 119 |
| Native camera/research hosts | 84 |
| Mobile/enrollment staging | 69 |
| Status/report historis | 33 |

## C. Status 23 manual reviews

Semua **INCLUDED setelah review**, nol HOLD. Status rinci dan hash per path ada di manifest; perubahan hanya diterapkan ke salinan integrasi.

| Path | Disposisi |
|---|---|
| `.env.example` | Ditinjau; kebijakan/assertion dipertahankan |
| `backend/README.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `docs/ANDROID_SETUP.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `docs/DECISIONS.md` | Ditinjau; kebijakan/assertion dipertahankan |
| `docs/EMAIL_OTP.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `docs/FL_ENR04_R2_TECHNICAL_REPORT.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `docs/FL_ENR06_TECHNICAL_REPORT.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `docs/FL_STAB02_TECHNICAL_REPORT.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `docs/TEST_PLAN.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `PROJECT_STATE.md` | Disanitasi; identitas/path/riwayat privat dihapus |
| `backend/internal/config/config_test.go` | Disanitasi; identitas/path/riwayat privat dihapus |
| `backend/internal/httpapi/email_handlers_test.go` | Ditinjau; kebijakan/assertion dipertahankan |
| `backend/internal/smtpemail/smtp_test.go` | Disanitasi; identitas/path/riwayat privat dihapus |
| `research/fl-cam03-host/evidence/device-install.json` | Disanitasi; identitas/path/riwayat privat dihapus |
| `research/fl-cam03-host/release-audit.json` | Disanitasi; identitas/path/riwayat privat dihapus |
| `research/fl-enr06r1-host/device.cjs` | Disanitasi; identitas/path/riwayat privat dihapus |
| `scripts/invite-admin.mjs` | Disanitasi; identitas/path/riwayat privat dihapus |
| `scripts/setup-smtp.ps1` | Disanitasi; identitas/path/riwayat privat dihapus |
| `scripts/test-email-headers.ps1` | Disanitasi; identitas/path/riwayat privat dihapus |
| `.gitignore` | Pengecualian source exact-file sempit |
| `research/fl-cam00-proof/.gitignore` | Pengecualian source exact-file sempit |
| `research/fl-cam00-proof/plugins/detector/patch-detector.cjs` | Whitespace aman; 14 cooked templates identik |
| `apps/mobile/src/features/enrollment/fixture.ts` | Provenance sintetis dibuktikan; komentar asal/hash ditambah |

Tambahan di luar daftar manual: `docs/FL_CAM03_TECHNICAL_REPORT.md` memakai root staging portabel; README dan PROJECT_STATE memperlihatkan status FL-GIT02 terbaru. Laporan dan manifest FL-GIT01 dipertahankan sebagai snapshot historis, bukan verdict FL-GIT02.

## D. Sembilan ignored source dan whitespace

Tiga Kotlin bridge berikut dibuka hanya lewat exception filename pada `research/fl-cam00-proof/.gitignore`:

- `research/fl-cam00-proof/plugins/android/Cam01PrivacyModule.kt`
- `research/fl-cam00-proof/plugins/android/Cam01PrivacyPackage.kt`
- `research/fl-cam00-proof/plugins/android/FaceLinkCaptureModule.kt`

Enam template CAM03 dibuka hanya lewat exception filename pada root .gitignore:

- `research/fl-cam03-host/templates/cam03-host-regression.test.ts`
- `research/fl-cam03-host/templates/cam03-metro-overlay.cjs`
- `research/fl-cam03-host/templates/cam03-native-check.tsx`
- `research/fl-cam03-host/templates/cam03-playback-metadata.test.ts`
- `research/fl-cam03-host/templates/cam03-playback-metadata.ts`
- `research/fl-cam03-host/templates/cam03-video-observer.ts`

Hash sembilan source cocok original. Negative ignore probes mempertahankan generated Android tree, file tak terdaftar, JPEG/model/database/key sebagai ignored. Tidak membuka keseluruhan direktori template atau Android secara rekursif. Temuan EOF pada EMAIL_OTP/TEST_PLAN dibersihkan. Enam spasi pada search-template detector dipertahankan dengan interpolasi konstan; pembandingan AST TypeScript melalui Node membuktikan **14 nilai cooked template identik**.

## E. Provenance dan privacy

Default inline JPEG berasal dari FFmpeg lokal 9.0.1, `lavfi testsrc2=size=112x112:rate=1`, satu frame, `-q:v 6`, dalam scope sintetis FL-ENR02A yang diotorisasi. Byte hasil decode identik dengan artefak lokal arsip: **3.334 byte**, SHA-256 `957096ec49bafff4915ed1d33250222a918c95a2ad9869985dfb63a3e74d19a9`. Tidak ada input kamera, foto orang atau download pada generator yang ditelusuri. Ini berbeda dari mannequin positif 512x512/17.817 byte dengan hash `cf572a...cabe`. **Tidak meminjam klaim CC0 mannequin atau menetapkan/mengubah lisensi.** Root JPEG mentah tidak diekspor.

Identitas mailbox/profil/perangkat/path lokal dan snapshot/urutan autentikasi nyata diubah menjadi placeholder atau hasil agregat, bukan direkayasa sebagai observasi sintetis. Alias peran publik D-038/D-039, makna hasil PASS/PARTIAL dan batas penerimaan dipertahankan. Dua review independen mencakup backend/Python/scripts serta mobile/native/docs. Review kedua menemukan duplikasi disclosure akun dan nama profil; parent memperbaiki seluruh lokasi terkait dan reviewer menutup temuan pada targeted closure.

Screening lokal seluruh **318 kandidat** memakai identifier aktual yang diketahui, mailbox pribadi, profil Windows, serial perangkat, private-key material, dan pola kredensial provider: **nol temuan tersisa pada cakupan itu**. Ini dilengkapi manual review, bukan jaminan semua secret tidak ada. Tidak mengunggah source sensitif, raw biometric, keys, DB atau evidence privat ke scanner eksternal.

**Risiko historis terpisah:** mailbox pribadi sudah ada pada history yang terjangkau origin/main. Redaksi tip branch tidak menghapus history tersebut. Tidak melakukan rewrite, force-push, atau rotasi kredensial. Tidak ada live credential leak terkonfirmasi dalam scoped review; bukan klaim ketiadaan secret pada seluruh history.

## F. Pengujian dan smoke aktual pada integrasi

| Pemeriksaan | Hasil baru |
|---|---|
| Go `test -count=1 ./...` + `vet ./...` | PASS, exit 0; 18 paket uji, 4 tanpa tes |
| Python `python -m pytest tests -q` | PASS, exit 0; satu skip |
| Mobile `npm run typecheck`, `npm run lint` | PASS, exit 0 |
| Main mobile Vitest invocation | 236 PASS / 2 SKIP; 16 file PASS / 1 SKIP |
| Student HTTPS opt-in terpisah | 1/1 PASS dengan FACELINK_ENR_E2E=1 |
| Combined real Python HTTPS | 2/2 PASS, termasuk dalam main invocation |
| AAD cross-record swap regression khusus | PASS, TestTemplateRejectsSamePersonDifferentRecordSwap |
| Native/JS contracts | 36/36 PASS |
| SMTP header parser regression | PASS: multi-hop/direct, tanpa body/client-IP |
| Node syntax | PASS, 21 file JS/CJS/MJS |
| Device-helper negative smoke | ANDROID_SERIAL kosong ditolak sebelum subprocess ADB |
| Whitespace | PASS pada tracked diff dan seluruh kandidat baru |
| Build APK / Android runtime / ADB | NOT_RUN; source native tidak diubah, tidak diperlukan untuk sanitasi ini |

Dua skip pada main mobile run: gate Student HTTPS memakai nama env FACELINK_ENR_E2E, sedangkan invocation awal memasok FACELINK_ENROLLMENT_E2E; jalur Student kemudian benar-benar dijalankan terpisah dan PASS. Satu skip lain adalah pengujian symlink POSIX di Windows. **Tidak menjumlahkan invocation berbeda sebagai satu run 238 PASS.** Python satu skip tidak diklaim lulus.

Smoke combined menjalankan client/controller TypeScript aktual, OTP normal, Go HTTPS, SQLite WAL schema8 disposable, Python YuNet/SFace asli, template AAD v2, server ACTIVE, withdrawal dan reconciliation. Manifest/pemrosesan cocok dengan canonical SHA yang sama; satu processing call, embedding 128 finite, envelope2/binding valid. Sesudah withdrawal: REVOKED, photo/template/active/ciphertext seluruhnya **0**; child EOF dan root disposable dihapus. Kasus response-loss terpisah: HTTP202 committed, replay exact body/key/CAS menghasilkan200, approval audit delta1 dan replay delta0, cleanup0. Bukan Android atau pemutusan Wi-Fi fisik.

Dependensi mobile dipasang dari cache dengan npm ci offline; ada warning upstream uuid/eslint deprecated dan postinstall unrs-resolver belum di-allow. Native Node mengeluarkan warning package ESM typeless. Tidak mengubah dependency untuk menyembunyikan warning. Kegagalan tooling sementara: Bun require absolut ke TypeScript gagal; pembandingan AST dijalankan sukses lewat Node native. Tidak ada functional test failure final.

Runtime lokal pengujian memakai salinan dependency Python privat di luar Git, model pinned lokal, fixture sintetis pinned dan **CA/key ephemeral baru**; bukan key/database Product Owner. Tidak menyalin credential pemilik atau menonaktifkan verifikasi TLS. Raw evidence hasil tes tetap ignored, bukan isi publication manifest.

## G. Struktur, dependency dan diff

Normal migration directory berisi 00001–00006. Migration enrollment 00007/00008 tetap di sibling `backend/synthetic-migrations`, dikomposisi ke direktori disposable hanya oleh harness. Main backend tidak menginjeksi enrollment, route produksi tetap default-OFF. Tidak menjalankan migration pada DB existing.

Go mod/sum, mobile package/lock, Python requirements sama dengan original setelah normalisasi LF/CRLF checkout; tidak ada perubahan versi dependency. Metadata x/sys indirect yang sudah ada tidak dirapikan lewat tidy spekulatif. Tidak mengubah auth/role/threshold/assertion/pipeline untuk memaksa PASS.

Statistik source terhadap base main, sebelum dua dokumen FL-GIT02 baru: **64 tracked paths**, +4.176/-728 baris; **254 new source paths**, 58.369 baris. Total 318 source paths, +62.545/-728 baris termasuk file baru. Dua deliverable FL-GIT02 dicatat terpisah. Ini source export lintas pekerjaan terdahulu, bukan seluruh perubahan dibuat pada tugas ini.

Manifest final: [FL_GIT02_PUBLICATION_MANIFEST.json](FL_GIT02_PUBLICATION_MANIFEST.json). Setiap included path mempunyai SHA-256 ekspor; hash manifest atas dirinya sendiri sengaja null untuk menghindari siklus. File private excluded tidak mempunyai payload/hash ekspor. Tidak ada file staged dan tidak ada commit baru.

## H. Verdict dan approval gate

**READY FOR OWNER REVIEW — NOT AUTHORIZED FOR PUBLICATION.** Nol manual/provenance HOLD tersisa pada source candidate yang diperiksa. Kesiapan ini tidak menutup risiko history publik, crash UNRESOLVED FL-STAB02, D-030/recovery/backup, PAD/liveness, kalibrasi manusia, native memory/cache zeroization, atau production enrollment.

Tidak ada commit/push/PR/merge, history rewrite, ADB/uninstall, perubahan SMTP/DNS, DB pemilik, auth policy, atau tahap fitur baru. Tidak memulai FL-UI01/Face Attendance/human enrollment.

## I. Rekomendasi commit groups dan PR — belum dijalankan

Urutan source-coherent yang direkomendasikan: (1) ignore hygiene; (2) Python IPC/runtime; (3) backend/OTP/CORE/contracts beserta wiring dan seluruh dependensi enrollment yang diperlukan agar compile tetap koheren; (4) native/research hosts; (5) mobile/enrollment staging; (6) sanitized status/reports. Jangan pecah shared router/server wiring ke pseudo-feature commits yang tidak dapat compile. Exact membership ada pada field group manifest; bukan instruksi git add seluruh manifest sekaligus.

Setelah review Product Owner, minta izin spesifik commit, push branch dan pembuatan PR sebagai tindakan berikut yang terpisah; PR harus membandingkan branch integrasi ini dengan main dan tidak membawa 21 ancestor lokal. Nyatakan risiko disclosure historis serta keterbatasan reproduksi dari clone publik: model, fixture positif, CA, dependencies, APK/video dan evidence lokal sengaja bukan payload publik. **STOP setelah persiapan; jangan push-all atau merge otomatis.**

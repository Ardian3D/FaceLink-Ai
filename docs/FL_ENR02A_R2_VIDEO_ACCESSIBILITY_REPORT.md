# FL-ENR02A-R2 — Video Lifecycle & Android Accessibility Stabilization

Tanggal: 26 Oktober 2026. **STATUS: PARTIAL; GO terbatas untuk FL-ENR02B synthetic-only.**

Defect lifecycle video yang dapat direproduksi sudah diperbaiki dan diuji A/B pada TECNO LJ7. Accessibility utama diperbaiki dan font scale 2.0 diuji. Validasi TalkBack penuh belum selesai; tidak ada klaim accessibility PASS menyeluruh. Tidak ada kamera, human capture, production enrollment, migrasi database pemilik, OTP/session change, commit/push/merge, atau pekerjaan FL-ENR02B.

## A. Baseline dan preservasi

- Branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Sebelum R2: staged0, modified35, untracked48. Tidak reset/stash/clean; seluruh pekerjaan sebelumnya dipertahankan.
- Laporan R1, implementasi video login, StepTracker, enrollment screen/flow/model, dan riwayat `ANDROID_SETUP.md` §17d dibaca.
- Product Owner memberi izin baru untuk ADB/update staging, TalkBack sementara, font scale sementara, dan lifecycle video. Kamera tidak dibuka.
- Main APK SHA256 awal/akhir `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`; aplikasi utama tidak diubah.
- Setting HP akhir: font1.0, TalkBack off, service `null`, touch exploration0, rotation0/accelerometer1, ADB reverse kosong. Staging tetap terpasang dan force-stopped.

## B. Reproduksi warning video

Build pra-fix + sesi tesintesis yang masih tersimpan. Saat TalkBack diaktifkan, proses staging restart; layar login memasang dua `VideoBackground` selama session restore, lalu redirect ke dashboard dan melepas player/VideoView. LogBox menampilkan:

```
Cannot set prop 'player' on view 'class expo.modules.video.TextureVideoView'
→ Cannot use shared object that was already released.
```

- Non-fatal; tidak ada FATAL EXCEPTION/ANR pada pemindaian logcat.
- 25 cold start biasa tanpa TalkBack tidak memunculkan warning; ini konsisten dengan race timing, bukan warning deterministik.
- Sesudah fix, restart dengan sesi sama dan TalkBack aktif mendarat di dashboard tanpa LogBox warning.
- Bukti lokal: `research/fl-enr02a-host/evidence-local/r2/talkback-current.png`, `video-warning-evidence.json`, `video-fix-after-restart.png`.

## C. Root cause dan fix

**Root cause yang didukung bukti:** layar masuk memasang video ketika `status` masih `memulihkan`. Jika session restore menemukan sesi sah, layar langsung `Redirect`; dua player dan dua `VideoView` dilepas pada commit yang sama sementara timing native Android (terutama dengan TalkBack) masih menerapkan prop player. Warning berasal dari urutan mount→redirect/release tersebut, bukan dari video itu sendiri.

**Fix minimal (`apps/mobile/app/index.tsx`):**

```ts
const videoAktif =
  status !== "memulihkan" && status !== "signedIn" && status !== "unregistered";
...
{videoAktif ? <VideoBackground /> : null}
```

- Video tidak dipasang selama session restore atau saat redirect ke role/dashboard.
- Video tetap dipasang untuk signedOut/failed/checking, sehingga layar login yang benar-benar dipakai tidak kehilangan latar yang disetujui.
- Tidak menyembunyikan LogBox, tidak try/catch, tidak menghapus video, tidak mengubah animasi/desain, dan tidak menonaktifkan cleanup.
- A/B device: pra-fix warning muncul; post-fix urutan restart yang sama tidak memunculkan warning.
- Batas: interleaving microtask native tidak diinstrumentasi secara forensik. Jalur teardown lain tidak semuanya dapat dipaksa terjadi.

## D. Accessibility findings dan perbaikan

Diperbaiki:

- Back button 40dp → target minimum48dp + hitSlop.
- Tombol retry error text-only → target minimum48dp + hitSlop.
- Perubahan phase/status sekarang diumumkan ke TalkBack melalui `AccessibilityInfo.announceForAccessibility`; ringkasan murni diuji, termasuk larangan mengumumkan ACTIVE basi sebagai “Wajah terdaftar”.
- Heading utama dan judul panel memakai `accessibilityRole="header"`.
- Status badge memiliki label gabungan “Persetujuan…”/“Status pendaftaran…”.
- Checkbox memiliki role checkbox, checked state, hint, dan row target yang besar.
- Progress scan memiliki label “Kemajuan pemindaian sintetis” + nilai0–100.
- Uploading memiliki label/busy state.
- StepTracker tidak lagi memakai role `progressbar` yang keliru untuk daftar langkah; tiap langkah tetap memiliki label status.
- Teks status offline membedakan **status terakhir yang diverifikasi** dari **status saat ini yang belum dapat diverifikasi**.
- Pesan ACTIVE basi tidak lagi salah menyalahkan template: snapshot kedaluwarsa, template belum siap, dan consent tidak aktif dibedakan.
- Baris info TTL dan header scan dapat membungkus pada font besar.
- Kontras teks badge terang memakai `textPrimary`; warna hanya pada dot/ikon, bukan teks status.

Audit:

- Kontras utama gelap: muted/surface6.75:1, warning10.31:1, success10.82:1. Terang: muted4.72:1, warning4.76:1, accent5.04:1.
- **Residual kontras:** teks putih pada tombol primary merah gelap (`#FF171B`) ≈3.90:1 dan light success sebelum perbaikan ≈4.13:1. Light success teks badge sudah diganti; putih pada tombol merah adalah token brand global di luar redesign yang diizinkan. Tidak diubah tanpa keputusan desain.
- Tree perangkat menunjukkan Back 129×129 px (48dp), checkbox checkable, tombol disabled/enabled, dan label status terbaca.
- Font scale2.0: intro dan consent dapat discroll, teks membungkus, tombol/checkbox tidak terpotong; tidak ada overlap workflow.
- TalkBack diaktifkan: fokus visual hijau teramati pada Back dan kontrol utama; layanan TTS aktif. One-time onboarding/permission TalkBack dan panjang notice membuat traversal ucapan penuh tidak selesai dalam sesi otomatis ini. Tidak ada klaim urutan baca/ucapan menyeluruh PASS.

## E. Android device evidence

- TECNO LJ7, package `ai.facelink.enr02host`, sesi sintetis nyata dan backend HTTPS disposable.
- Warning pra-fix dan ketiadaan warning post-fix dibandingkan pada urutan restart yang sama.
- Font scale1.0/2.0 diuji pada intro/consent; setting dikembalikan1.0.
- TalkBack diuji sementara; setting dikembalikan off dan touch exploration0.
- Tidak ada permission/setting global lain yang diubah; tidak ada kamera.

## F. Main/staging regression

| Pemeriksaan | Hasil |
|---|---|
| Main `npm run typecheck && npm run lint && npm test` | **143 passed, 1 opt-in E2E skipped** |
| Staging perintah yang sama | **143 passed, 1 skipped** |
| E2E opt-in R1 | Tidak dijalankan ulang pada R2; tidak diperlukan untuk perubahan lifecycle video/a11y |
| `npx expo export --platform android --output-dir artifacts/export-r2-final` | Lulus;1491 modules |
| Gradle staging debug ARM64 | **BUILD SUCCESSFUL 6m59s** setelah perubahan JS; 557 task, 33 executed |
| Backend `go test -count=1 ./... && go vet ./...` |16 paket uji lulus/5 tanpa tes; vet lulus |
| Device lifecycle | Pra-fix warning, post-fix tanpa warning pada restart sesi; tanpa FATAL |
| Font scale | Intro/consent2.0 dapat discroll dan tidak terpotong |

Tidak ada perubahan backend. Tidak ada resource leak yang terbukti dalam observasi; bukan forensic memory/player leak audit.

## G. Remaining residuals

1. **TalkBack penuh belum PASS.** Satu kali onboarding/permission prompt sistem dan traversal notice panjang tidak diselesaikan otomatis; ucapan aktual tidak ditranskripsikan. Label/role/target/announcement telah diperbaiki dan tree diverifikasi.
2. **Kontras putih pada tombol primary merah gelap** masih ≈3.90:1. Ini token brand global; perubahan akan memengaruhi seluruh aplikasi dan membutuhkan keputusan desain/pemilik.
3. **TalkBack/font scale** hanya diuji pada intro/consent, bukan seluruh state pending/active/expired/withdrawal secara visual dengan font2.0. Layout state lain mengikuti pola scroll/flex yang sama dan R1 sudah menguji fungsinya.
4. Root cause video didukung A/B dan urutan mount/redirect, tetapi exact native microtask interleaving tidak diinstrumentasi. Build native/APK tidak diubah; JS bundle final diverifikasi.
5. Tidak ada human capture, Face Recognition nyata, PAD, production, migrasi Product Owner, power-loss, atau ESP-CAM.

## H. GO/PARTIAL/NO-GO menuju FL-ENR02B

**PARTIAL + GO terbatas:** defect lifecycle video yang dapat direproduksi sudah diperbaiki; tidak ada crash/leak terukur; alur utama enrollment memiliki label/role/target/announcement yang diperbaiki dan lolos regression. Sisa residual adalah validasi TalkBack penuh dan keputusan kontras brand global.

**NO-GO:** production enrollment, human capture, kamera, migrasi database Product Owner, ESP-CAM, commit/push/merge/PR19, atau klaim accessibility menyeluruh PASS. **STOP — FL-ENR02B tidak dimulai otomatis.**

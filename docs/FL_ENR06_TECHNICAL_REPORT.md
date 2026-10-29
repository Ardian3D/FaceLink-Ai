# FL-ENR06 — Full Android Synthetic Enrollment Acceptance

Tanggal persiapan: 28 Oktober 2026. **Status: PARTIAL — Student Android mencapai PENDING_APPROVAL; kelanjutan tanpa perangkat diminta pemilik karena TECNO sedang dipakai.** Acceptance Android end-to-end belum selesai. Tidak ada kamera, wajah manusia, atau production.

## A. Git baseline

- Branch `feat/fl-otp01-email-auth`; HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Local `origin/main`: `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`; divergence `21/1`. PR #19 sudah merged pada pekerjaan sebelumnya dan tidak di-merge ulang.
- Baseline: 60 tracked modifications dan 207 untracked files. Seluruh pekerjaan FL-CORE/FL-CAM/FL-ENR/FL-SEC dipertahankan; tidak reset/stash/clean/rebase/pull/switch.
- APK staging sebelumnya `c8cb730c590022d2dc538df3ace7ef50d4b726d393614f11c6d75a48a4ea8134` disalin ke `research/fl-enr02a-host/artifacts/enr06-preserved-c8cb…apk` sebelum build baru.
- APK utama `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`, video, dan model pinned tetap tidak berubah.
- Tidak ada commit/push/merge.

## B. Device permission dan isolasi staging

**Izin perangkat sebelumnya diberikan untuk ADB terbatas termasuk screenshot sintetis.** APK staging telah dipasang dan Student UI dijalankan; kemudian sambungan ADB hilang. Pemilik kini meminta kelanjutan tanpa HP. Tidak ada ADB atau interaksi perangkat dalam kelanjutan offline ini. Izin sebelumnya bukan alasan untuk mengganggu HP yang sedang dipakai.

APK kandidat yang dibangun offline:

| Item | Nilai |
|---|---|
| Application ID | `ai.facelink.enr02host` (berbeda dari main `ai.facelink.mobile`) |
| APK | `research/fl-enr02a-host/artifacts/enr06-d9a32e5067dba37a9e37668fee45f3a048c54b0c2c7763feef434851ac359967.apk` |
| Ukuran | 102.262.250 byte |
| SHA-256 | `d9a32e5067dba37a9e37668fee45f3a048c54b0c2c7763feef434851ac359967` |
| Signature | debug `CN=FaceLink ENR02A Staging Debug` SHA-256 `e0992e2706f8fbbe23cc7c8b846a833b5f61ac08bcd9f5ea930e2560689da68d` |
| min/target SDK | 24 / 36 |
| Debuggable | Ya (debug staging, bukan production) |
| Launchable | `ai.facelink.mobile.MainActivity` dalam package staging |

Permission APK hasil audit `aapt`: `INTERNET`, `VIBRATE`, `CHANGE_WIFI_MULTICAST_STATE`, `ACCESS_NETWORK_STATE`, `ACCESS_LOCAL_NETWORK`, `WAKE_LOCK`, `USE_BIOMETRIC`, `USE_FINGERPRINT`, dan permission receiver dinamis milik package staging. **Tidak ada** `CAMERA`, `READ_MEDIA_IMAGES`, `READ/WRITE_EXTERNAL_STORAGE`, atau `SYSTEM_ALERT_WINDOW`.

- APK memuat `res/xml/network_security_config.xml` dengan `cleartextTrafficPermitted=false` sebagai base, CA sistem + `res/raw/dev_ca.pem` (SHA-256 sama dengan `backend/dev/certs/dev-ca.pem`), dan cleartext hanya untuk `127.0.0.1`/`localhost`/`10.0.2.2` agar Metro dev dapat berjalan. Verifikasi TLS API tidak dimatikan.
- Debug APK tidak memuat JS bundle; Metro wajib berjalan pada `127.0.0.1:8081`.
- Main app tidak ditimpa: package, storage/Keystore namespace, dan debug key staging terpisah. Enrollment main tetap default-OFF dan route staging hanya ada di generated tree.
- Staging memakai `synthetic_flow`: tidak ada CAMERA, tidak ada modul kamera, tidak ada live scanner. Guided capture hanya menghasilkan fixture sintetis di memori.
- Fixture positif: JPEG prosedural mannequin `procedural-mannequin-heightfield-v1-01.jpg`, 17.817 byte, SHA-256 `cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe`, artefak yang sama yang lulus YuNet/SFace pada FL-ENR04-R2. Tidak ada gambar manusia, kandidat baru, atau download.
- Fixture diinjeksikan hanya ke `stage/src/features/enrollment/fixture.ts` (generated, Git-ignored) dengan provenance pada `enr02a-source-manifest.json`. Tidak masuk tracked source, galeri, atau layanan luar.
- Backend device run: `enrollment-staging-serve -integration-fixtures`, HTTPS loopback, SQLite WAL schema8 disposable, key purpose-separated di memori, identitas sintetis, OTP normal, worker paused, dan control stdin. Port 8091/8081 sebelumnya dihubungkan melalui reverse setelah izin; kelanjutan offline tidak memakai koneksi perangkat.

## C. Perubahan source offline

1. `research/fl-enr02a-host/prepare-enr02a.cjs`
   - Flag preparation-only `FACELINK_ENR06_FIXTURE=1`; wajib `synthetic_flow` + Admin staging.
   - Verifikasi strict: file ada, bukan symlink/non-regular, ukuran 1.001–1.048.575 byte, SHA-256 pinned lowercase; gagal tertutup.
   - Injeksi base64 hanya ke generated fixture module, dengan header provenance; manifest mencatat hash/byte/camera=false.
2. `apps/mobile/src/features/enrollment/{model,api,flow,EnrollmentScreen}.ts*`
   - Student pending countdown sekarang memakai `server_now` terautentikasi, bukan jam perangkat; elapsed lokal di-clamp dan tidak bisa memperpanjang TTL. Tanpa `server_now` yang valid, deadline tidak dikarang (`—`).
3. Tes: guard fixture (hash/size/regular/missing/opt-in), generated module + guided capture sekali pakai, deadline server vs jam perangkat skew/rollback, dan regresi token-null existing.

## D. Regression offline yang dijalankan

| Pemeriksaan | Hasil aktual |
|---|---|
| Python tests | 217 passed, 1 skipped |
| Go tests | 18 packages ok, 4 tanpa tests |
| `go vet ./...` | Exit 0 |
| Mobile typecheck + lint | Lulus |
| Mobile full suite (opt-in E2E tidak aktif) | 229 passed, 9 skipped |
| Combined HTTPS+real-Python E2E (FL-ENR05) | 2/2 passed |
| Native/JS contract offline | 36 passed |
| Android staging build ARM64 | BUILD SUCCESSFUL (557 tasks) |
| APK permission/package/TLS audit | Sesuai tabel B |

Hasil offline tidak membuktikan rendering/lifecycle/cache Android. Hasil perangkat parsial yang tersimpan dicatat terpisah di bawah.

### Kelanjutan tanpa HP — hasil run baru

Pemilik meminta lanjut tanpa TECNO yang sedang dipakai. Tidak ada perubahan kode aplikasi, ADB, emulator, build/install, atau migrasi database Product Owner pada kelanjutan ini.

| Perintah dari `apps/mobile` | Hasil run baru |
|---|---|
| `npm run typecheck && npm run lint && npm test` | Typecheck/lint exit0; 229 passed, 9 skipped |
| `FACELINK_ENR05_E2E=1 NODE_EXTRA_CA_CERTS=<repo>/backend/dev/certs/dev-ca.pem npx vitest run tests/enrollmentCombinedE2E.test.ts` | 2 passed; actual TS→Go HTTPS→real Python pada database disposable baru |

Evidence smoke baru: `research/fl-enr02a-host/evidence-local/enr05/combined-<redacted-id>.json`. Hasil `PASS`, `android_ui=NOT_RUN`: pending → APPROVED_PROCESSING (active0) → real-Python ACTIVE dengan binding template valid → withdrawal/REVOKED dan photo/template/active/ciphertext0. Kasus response-loss terpisah: approval202, exact replay200, satu audit approval dan nol audit approval tambahan; cleanup0. Child berhenti lewat EOF dan root disposable dihapus. Ini **run baru**, bukan kelanjutan submission Android pending sebelumnya.

Baseline Git di A merupakan catatan sesi sebelumnya, bukan verifikasi ulang: `git` tidak tersedia pada PATH Windows saat kelanjutan ini (`git status` command-not-found; `where.exe git` tidak menemukan executable). Tidak menjalankan commit/push/merge atau operasi pemulihan Git. Go full suite/vet, Python full suite dan build pada tabel awal merupakan hasil sebelumnya; tidak dijalankan ulang dalam kelanjutan dokumentasi ini.

### Bukti Android parsial sebelum koneksi putus

- Installed staging APK teramati memiliki SHA-256 sama dengan kandidat pada B.
- Student login OTP → notice/checkbox consent eksplisit → guided scan sintetis → upload → layar `Menunggu verifikasi Admin`.
- Screenshot pending menampilkan sisa retensi sekitar `71j 59m`; submission `<redacted-id>`.
- Snapshot `device/acks.jsonl`: `PENDING_APPROVAL`, schema8/WAL, photo_rows1, ciphertext_files1, template_rows0, active_rows0, processing_calls0.
- Canonical SHA-256 `00eaa7792b75bd3f7f2ad10bfcbbb5fd3afe64fec4b626761a0e42b239cace76`; expiry `2026-10-31T12:19:24.760Z`, 72 jam sejak submission menurut catatan run.
- Setelah relaunch, Student dashboard dan pending tampil kembali. Ini bukti pemulihan terbatas; bukan PASS seluruh background/cold-start/session lifecycle. Wait pada beberapa percobaan gagal karena route/teks yang diharapkan tidak sedang tampil.
- Tap profil membuka Expo Tools yang menutupi sebagian tombol; sebelum logout/Admin dijalankan koneksi ADB terputus. Admin approval, real-model activation, withdrawal/cleanup dan preview-cache Android pada chain ini **NOT_RUN**.
- Evidence lokal: `research/fl-enr02a-host/evidence-local/enr06/device/`, khususnya `06-student-pending.png`, `09-restored-pending.png`, dan `acks.jsonl`. Evidence ini historis; bukan snapshot live database sekarang.

## E. Device gate — izin yang diminta

Daftar berikut merekam batas izin perangkat sebelumnya. Setelah pemilik meminta kelanjutan tanpa HP, operasi ini tidak dijalankan lagi; sisa acceptance menunggu perangkat tersedia dan arahan pemilik.

1. `adb devices -l` dan `getprop` read-only untuk identifikasi perangkat.
2. `adb install -r` APK staging yang checksum-nya tercatat.
3. `adb reverse tcp:8091 tcp:8091` dan `tcp:8081 tcp:8081`.
4. Launch staging app; `input tap/text/keyevent` untuk menggerakkan UI.
5. `uiautomator dump` + `adb pull` untuk pohon UI/aksesibilitas.
6. Screenshot host-side (`screencap`) untuk bukti visual—kecuali Product Owner memilih tanpa screenshot.
7. `run-as ai.facelink.enr02host` untuk memeriksa tidak ada file gambar/JPEG tersimpan di storage privat app.
8. `logcat` terfilter PID app untuk lifecycle/error tanpa token/gambar.
9. `force-stop`/launch ulang untuk lifecycle test.
10. Di akhir: hentikan app, hapus port reverse milik sesi ini; **tidak** mengubah setting global, Wi-Fi, developer options, atau package main.

Tidak termasuk: kamera, wajah manusia, ESP-CAM, upload live, database Product Owner, production enrollment/attendance, perubahan threshold/model, SMTP/DNS, atau uninstall tanpa izin.

## F. Positive chain Android yang belum lengkap

Target tetap satu proses backend dan satu submission/artefak; hanya langkah Student hingga pending yang sudah teramati pada perangkat:

1. Student login OTP normal → consent notice → setuju → guided scan → upload fixture pinned → PENDING_APPROVAL + sisa waktu server.
2. Admin login OTP → queue → submission yang sama → binding/NIM/roster/consent → protected preview (hash canonical sama) → Approve.
3. Buktikan 202 belum ACTIVE; jalankan `worker.step` (Python asli) → embedding valid → template AADv2 → ACTIVE resmi.
4. Student refresh → ACTIVE; withdraw → client tidak mempercayai cached ACTIVE → server REVOKED/template_ready false → reconcile → foto/template/ciphertext 0.
5. Blind spot/lifecycle/negative diuji terpisah dan dilabeli sesuai lapisan (Android observation vs automated test vs backend evidence).

## G. Remaining blockers

- Perangkat tidak tersedia karena sedang dipakai pemilik; kelanjutan kini offline-only. Student pending terbukti sebagian, tetapi full Android chain/acceptance belum dapat diklaim.
- TalkBack dan perubahan font scale memerlukan izin setting terpisah.
- Native bitmap/preview cache, recents screenshot, dan OS decoder retention tidak dijamin; akan diperiksa sejauh yang diizinkan tanpa klaim zeroization.
- D-030, backup/insiden, coherent DB rollback, PAD/liveness, akurasi manusia, full camera JPEG, face attendance, dan F3 flicker tetap BLOCKED.

## H. GO/PARTIAL/NO-GO

**PARTIAL:** persiapan offline, build dan audit APK selesai; Student Android mencapai pending sebelum koneksi putus. **NO-GO untuk klaim full Android acceptance** sampai sisa chain benar-benar dijalankan di perangkat. Kelanjutan tanpa HP hanya memverifikasi lapisan otomatis/backend/client dan tidak menggantikan acceptance Android. Tidak memulai FL-UI01, human enrollment, atau production.

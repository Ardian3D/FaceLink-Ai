# FL-ENR06-R1 — Final Android Integrated Enrollment Acceptance

Tanggal: 29 Oktober 2026. **Hasil jalur primer: PASS_PRIMARY_ANDROID_CHAIN. Verdict keseluruhan: PARTIAL**, karena satu crash native React Native inspector ditemukan pada pengujian lifecycle tambahan dan belum diremediasi. Bukan izin enrollment manusia, production, face attendance, migrasi Product Owner, atau FL-UI01. STOP setelah laporan.

## A. Device authorization

Preflight offline selesai sebelum ADB. Product Owner secara eksplisit mengizinkan ADB terbatas, instal/update staging, interaksi TECNO LJ7 dan screenshot sintetis. Setelah blocker pergantian akun terkonfirmasi, pemilik memilih dua package staging terisolasi; setelah audit APK kedua, pemilik secara terpisah mengizinkan instal/update `ai.facelink.enr06admin` dan acceptance dua package.

- Device teridentifikasi TECNO LJ7, Android 16, 1080×2460.
- Student: `ai.facelink.enr02host`; Admin: `ai.facelink.enr06admin`. Keduanya memakai UI/client existing dan penyimpanan sesi aplikasi terpisah.
- Student APK SHA-256: `d9a32e5067dba37a9e37668fee45f3a048c54b0c2c7763feef434851ac359967`, 102262250 byte.
- Admin APK SHA-256: `67d69b92a84d5462e0acfbd47ba99cf35ea412b69612e8d2411bddcc1686487a`, 100185277 byte. Build ARM64 offline sukses; debug signature FaceLink terverifikasi. Checksum installed APK masing-masing cocok dengan APK audit.
- Tidak ada permission CAMERA, media/storage atau overlay pada kedua APK audit. Permission `USE_BIOMETRIC`/`USE_FINGERPRINT` berasal dari dependensi penyimpanan kredensial; bukan permission kamera dan tidak dipakai untuk capture pada acceptance ini.
- Tidak ada perubahan setting global HP, Wi-Fi/data seluler, firmware, kamera atau galeri. Aplikasi utama `ai.facelink.mobile` tidak dipasang ulang atau ditimpa.

## B. Baseline Git and preservation

Baseline branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`; local `origin/main` `95d31d8fa03d9bdaaee035da9a0bba6b606b4bc00`; divergence HEAD/main 21/1. PR #19 sudah merged; tidak di-merge ulang. Baseline: 60 tracked modifications dan 212 untracked files, dipertahankan. Tidak ada reset, stash, clean, rebase, pull, commit, push atau merge pada tugas ini.

Hash sebelum/sesudah cocok untuk APK utama, `video-clips.local.js`, `video-clips.none.js`, `video1.mp4`, `video2.mp4`, dan kedua ONNX model. APK/evidence lama tetap disimpan. Tidak ada migration baru, perubahan OTP/session/role, model, threshold, quality gate, KeyProvider atau database Product Owner.

## C. Staging and HTTPS

Satu environment baru final `run4`: Go staging integration mode, SQLite WAL schema 8 disposable, purpose-separated in-memory test keys, identitas Student/Admin sintetis, normal Email OTP, real Python YuNet/SFace dan worker awalnya paused. Runtime lokal: Metro 8083, HTTPS relay 8094, Go HTTPS 8095. ADB reverse hanya untuk port milik tugas. Relay memverifikasi upstream TLS dengan dev CA; bukan `InsecureSkipVerify`.

Satu JPEG prosedural pinned dari R2 digunakan kembali, bukan foto manusia atau fixture baru:

- Source SHA-256: `cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe`.
- Canonical SHA-256 sesudah normalisasi Go: `00eaa7792b75bd3f7f2ad10bfcbbb5fd3afe64fec4b626761a0e42b239cace76`.
- JPEG hanya masuk generated staging tree yang diabaikan Git. Model/threshold tidak diubah. Main enrollment route tetap OFF.

Host test-only `research/fl-enr06r1-host/session.cjs` memiliki stdin Go, Metro dan HTTPS relay; control berupa file lokal, bukan endpoint produk. Relay fault hanya untuk acceptance terkontrol. Source dump UI bounded disimpan di `research/fl-enr06r1-host/ui/`; ini tooling uji Android lama, bukan perubahan aplikasi atau layanan production.

## D. Student Android flow

Run final memakai trace `a2d0f78c-cef7-4c8e-b8cd-62fbe8d0aa22` dan submission **`41f0abdb-44a5-48d1-9dec-822e5ee474b1`** sepanjang positive chain:

1. Login Student melalui UI Email OTP normal; kode hanya dibaca dari file delivery disposable ke memori dan diketik, tidak dimasukkan ke evidence.
2. Notice consent, checkbox dan persetujuan melalui UI.
3. Guided `synthetic_flow` menghasilkan pinned fixture, diunggah oleh client aplikasi existing.
4. Android menampilkan pending; snapshot backend mengonfirmasi `PENDING_APPROVAL`, satu encrypted pending photo, nol template dan nol active enrollment.
5. Background/resume menampilkan pending dari backend; tidak menggabungkan submission Android lama dari FL-ENR06/R1 run sebelumnya.

## E. Admin Android flow

Admin login OTP pada package kedua, tanpa logout Student. Queue dan detail membuka submission final yang sama; UI memperlihatkan account-person-NIM-roster sintetis serta UUID referensi identity fixture.

- Protected preview berhasil dengan SHA canonical yang sama dengan submission/input processor.
- Background/resume Admin menutup preview; preview tidak otomatis tampil lagi.
- Attestation binding dan consent dilakukan melalui form existing.
- Fault `stale-etag` pada relay menghasilkan **HTTP 412**, tetap `PENDING_APPROVAL`, tidak ada template/ACTIVE. Form dimuat ulang sebelum keputusan berikutnya.
- Approve valid menghasilkan upstream **HTTP 202 / APPROVED_PROCESSING**, bukan ACTIVE. Sebelum worker: template0/active0.
- Relay membuang respons upstream202 setelah commit; request native berikutnya identik body/key/ETag mendapat **HTTP 200**, audit approval tetap satu.
- **Batas response-loss Android:** tidak teramati layar uncertain atau klik replay eksplisit di UI; native transport mengulangi request sebelum UI uncertain teramati. Ini bukti duplicate transport/idempotensi setelah response loss, bukan bukti manual `AdminReviewFlow.replay()` dari layar uncertain Android. Controller uncertainty/replay tercakup terpisah pada E2E otomatis existing.

## F. Real YuNet/SFace execution

Control worker-step kemudian memproses **submission final yang sama** melalui implementasi Go/Python existing, tanpa processor palsu atau SQL activation shortcut.

- Satu processing call, satu wajah, embedding 128 dimensi finite, manifest verified.
- Canonical photo hash dan processed hash sama dengan hash pada §C; preview terotorisasi juga cocok.
- Go menerima hasil Python asli melalui HMAC private IPC dan memeriksa ulang domain/fencing sebelum activation.
- Model dan pipeline fingerprint tetap real implementation R2/SEC01; tidak menurunkan threshold deteksi 0,75 atau quality gate.
- Tidak mencatat embedding, JPEG, key atau token dalam laporan/evidence metadata.

## G. AAD v2 and ACTIVE

Snapshot backend setelah worker/reconciliation: `ACTIVE`, `template_ready=true`, envelope version2, dan `template_binding_valid=true`. Pembacaan test-only membangun binding dari relasi tersimpan dan berhasil memvalidasi template AADv2; bukan mempercayai UUID di envelope atau mengubah row supaya lolos.

Student yang tetap login membuka kembali enrollment pada package Student; UI actual menampilkan **“Wajah terdaftar”** dan keterangan bahwa backend menyatakan template siap serta consent aktif. Screenshot final disimpan. Ini bukan klaim akurasi wajah manusia, PAD/liveness atau face attendance production.

## H. Withdrawal and deletion

Pada submission final, relay backend-offline membuat status Android **“Aktif (belum terverifikasi)”**; UI tidak lagi menampilkan “Wajah terdaftar” sebagai status terpercaya. Setelah koneksi relay dipulihkan, status backend dapat diverifikasi lagi.

Student menarik consent melalui dialog native. Snapshot final: `REVOKED`, `template_ready=false`; immutable consent grant identity tetap. Reconciliation selesai: **photo0/template0/active0/ciphertext-file0**. UI menampilkan **“Persetujuan ditarik”** dengan alasan `STUDENT_REQUEST`.

Negative submission terpisah `26d1c650-c7ff-4b1a-a6f1-2a7ed6e42892` hanya untuk preview/logout tambahan, tidak dipakai menggantikan positive chain. Submission itu juga ditarik dan dibersihkan. Total seluruh database final: photo0/template0/active0; ciphertext files0.

## I. Lifecycle and security

### Hasil teramati dan batas

| Skenario | Bukti/perbedaan jenis pemeriksaan |
|---|---|
| Student background/resume pending | UI Android dan snapshot submission final |
| Student session restore | Relaunch package menampilkan sesi Student; status ACTIVE dimuat kembali dari backend |
| Admin preview background/resume | Preview tertutup; metadata/hash preview cocok sebelum ditutup |
| Backend offline saat ACTIVE | Fault relay lokal; UI menjadi belum terverifikasi, bukan ACTIVE terpercaya. Wi-Fi HP tidak diubah |
| Admin preview offline + logout | Pada negative submission: preview tertutup saat backend offline, tetap tertutup sesudah resume; logout204; route Admin sesudah logout kembali ke layar login |
| ETag conflict | Actual Android approve412; tidak ACTIVE; muat ulang dan approve valid |
| Approval response loss | Actual upstream202 dibuang; native duplicate mendapat200 dengan body/key/ETag sama; audit approval tunggal. UI uncertain tidak teramati |
| Logout Student sebelum Admin pada run2 | Logout204 menaikkan biometric epoch; pending menjadi REVOKED/SUBJECT_INELIGIBLE. Guard tidak diubah; dua package menjadi solusi isolasi |
| File persistence check | `run-as` files/cache kedua package tidak memuat pathname `.jpg/.jpeg/.png/.webp` sesudah cleanup; bukan proof tidak ada file tanpa ekstensi, decoder cache, native memory atau forensic remnants |
| Follow-up switches sesudah crash | Enam background/package-switch cycles; fatal crash buffer tidak bertambah. Tidak menutup defect crash yang sudah teramati |
| Access expiry/72h/retry/concurrency/restart | Regresi otomatis existing lulus; tidak diubah menjadi klaim setiap skenario telah didemonstrasikan melalui UI Android final |

### Crash native yang tetap terbuka

Sesudah primary chain sudah REVOKED dan selama cleanup negative preview/logout, Student staging sempat berhenti. Crash buffer:

`java.lang.NullPointerException` → `com.facebook.react.devsupport.CxxInspectorPackagerConnection$WebSocketDelegate.didReceiveMessage(Native Method)` → callback `CxxInspectorPackagerConnection.kt:202`.

Stack mengidentifikasi boundary native dev inspector, **bukan** penyebab akar yang sudah terisolasi. [INFERENCE] lifecycle delegate/WebSocket dapat terlibat; belum ada reproduksi deterministik yang membuktikannya. Tidak menambahkan catch/suppression atau patch spekulatif pada React Native/node_modules. Aplikasi dipulihkan tanpa mengubah row; consent negative kemudian ditarik melalui UI dan cleanup berhasil. Enam switches sesudah pemulihan tidak memunculkan fatal baru, tetapi ini tidak membuktikan defect telah hilang. Ini blocker untuk menandai seluruh lifecycle acceptance Android sebagai PASS.

Independent read-only security/lifecycle reviews meninjau host/relay, privacy evidence dan boundary existing. Findings helper OTP shared-latest-challenge dimitigasi dengan serialisasi request→committed delivery→entry→dashboard per package; **helper bukan concurrency-safe**, bukan perubahan auth produk. Cap control-file yang semula berpotensi error berulang diperbaiki sebelum read: satu overflow ack, shutdown dan return. Smoke overflow aktual: exit0/root_removed=true. Review relocation UI tidak menemukan defect baru berbasis bukti pada scope yang ditinjau; crash native yang teramati sesudahnya tetap dicatat terbuka, bukan tertutup oleh review statis.

## J. Regression and defects

### Fix minimal pada source aplikasi

Polling audit `enrollment.review_opened` menambah history **di atas** form Admin, sehingga checkbox/Approve bergeser saat dibaca atau disentuh. Reproduksi Android run3 memperlihatkan perpindahan bounds dan sentuhan gagal. Fix: pindahkan panel history setelah form keputusan; endpoint, auth, polling, audit dan validasi keputusan tidak diubah.

Berkas aplikasi: `apps/mobile/src/features/adminEnrollment/AdminEnrollmentScreen.tsx`. Generated staging copy disinkronkan; bukan perubahan dependency/native APK. Android run4 setelah fix: bounds checkbox sebelum/sesudah tetap `[111,1236,969,1366]` sementara history bertambah 33→35. Confirm412 dan approval202 lalu real ACTIVE membuktikan form dapat dipakai. Tidak menulis test source-text/penyalinan JSX untuk relocation ini.

### Pemeriksaan yang dijalankan

| Pemeriksaan | Hasil |
|---|---|
| Final `go test -count=1 ./... && go vet ./...` dari backend | Exit0, 18 paket lulus, 4 paket tanpa tes; vet exit0 |
| Final Python `services/face-engine/.venv/Scripts/python.exe -m pytest services/face-engine/tests -q` | Exit0, satu skip; seluruh tes non-skipped lulus |
| Final mobile `npm run typecheck && npm run lint && npm test` | Exit0; 229 passed, 9 skipped, 14 test files passed/3 skipped |
| Mobile suite dengan inline flags opt-in | 236 passed/2 skipped; Admin dan Student HTTPS berjalan, combined masih skip pada invocation ini |
| Combined HTTPS dengan explicit subprocess environment `FACELINK_ENR05_E2E=1` | 2/2 passed, real Python positive lifecycle dan response-loss. Dijalankan terpisah, tidak diklaim sebagai satu invocation238PASS |
| Android final primary chain | PASS: one submission UI Student→Admin→real Python/AADv2 ACTIVE→Student withdrawal→REVOKED/cleanup0 |
| Native lifecycle tambahan | FAIL: satu crash inspector; recovery/cleanup berhasil, fix native belum dibuat |
| Tooling host `node --check session.cjs/device.cjs` | Exit0; host/relay juga dijalankan aktual pada acceptance |
| Saved Java dump-tool source compile | Exit0 dengan android.jar+uiautomator.jar+JUnit; warning source8 bootstrap/deprecated API. Percobaan compile sebelumnya lupa android.jar dan gagal, classpath dikoreksi |

Regresi Go/Python bukan pengganti bukti native. Run1 terputus oleh timeout VM tool yang ikut mematikan server/relay/Metro; tidak dihitung sebagai PASS. Run2 membuktikan logout fence, bukan positive chain. Run3 mendiagnosis form berpindah; tidak digabung ke PASS run4. Java dump tooling menggunakan legacy private bridge karena idle-wait dump standar pada Android/video ini timeout atau mengambil autofill window; depth/node limits diterapkan. Tooling ini tidak mengubah app dan bukan jaminan kompatibilitas lintas Android.

## K. UI issues for FL-UI01 and remaining limits

- Fixed: form bergeser akibat riwayat audit bertambah di atas form.
- Tetap terbuka: crash native dev inspector satu kali, root cause/remediasi belum terbukti.
- Dev-client Tools bubble bertumpuk dengan profil kanan atas; pengujian memakai area profil yang tidak tertutup. Ini observasi debug staging, bukan dasar redesign production.
- F3 konfirmasi/modal flicker yang sudah dicatat belum dinyatakan selesai. Label/button berada di ScrollView; perlu audit visual tersendiri, bukan melemahkan validasi untuk memudahkan automation.
- Tidak menjalankan TalkBack atau mengubah font scale karena membutuhkan izin setting tersendiri. Tidak mengklaim audit aksesibilitas lengkap.
- Preview close/logout terbukti pada UI dan protected endpoint; tidak menjamin native bitmap/cache zeroization, screenshot/recents sanitization atau forensic erase. D-030 recovery/irreversible deletion, backup/incident retention, full-database rollback freshness, PAD/liveness dan kalibrasi/akurasi manusia tetap terbuka.

## L. Verdict, artifacts and safe stop

**PARTIAL secara keseluruhan.** Acceptance positive integrated Android pada satu submission benar-benar **PASS**; bukan hasil gabungan dari Android pending dan laptop ACTIVE yang berbeda. Tidak memberi GO tanpa syarat untuk tahap fungsional berikutnya karena crash native tambahan belum ditutup dan beberapa lifecycle/UI scenarios hanya dibuktikan otomatis atau belum dijalankan di device. Tidak ada security guard yang dilonggarkan agar PASS.

Evidence lokal (diabaikan Git): `research/fl-enr02a-host/evidence-local/enr06-r1/`.

- `baseline.json`, `staging-preflight.json`, `admin-apk-preflight.json` serta APK tersimpan di `research/fl-enr02a-host/artifacts/`.
- `run4/primary-proof.json`, `snapshots.json`, `http.jsonl`, `acks.jsonl`, `form-stability.json`, `student-offline-text.json`.
- Screenshot pending, preview closed, ACTIVE, offline-unverified, withdrawal, preview/logout negative dan crash. Tidak merekam screenshot saat kode OTP terisi.
- `run4/native-crash.txt`, `negative-and-runtime.json`, `device-cleanup.json`, `preservation-and-cleanup.json`, `exit.jsonl`.
- Host trace tidak mengarang worker request ID. Korelasi primary memakai satu submission UUID dan SHA canonical/input yang sama; Python request UUID tetap private IPC independent.

Penutupan terverifikasi: kedua staging di-force-stop, reverse8083/8094 dilepas, temporary device UI jar/XML dihapus, host exit0 dan root disposable terhapus. Port8083/8094/8095 tidak listening; root run1 juga tidak tersisa. APK staging tetap terpasang sebagai evidence; tidak di-uninstall. Tidak mengubah aplikasi utama atau menghapus APK/video/evidence lama. Temporary host compile/tool directory dibersihkan; sumber reproduksi kecil disimpan, bukan binary JAR.

Untuk reproduksi berizin berikutnya: siapkan source staging seperti README harness, dua APK ber-ID terpisah yang diaudit, satu backend disposable baru dengan pinned real processor, lalu login Student dan Admin **tanpa logout Student sebelum activation**. Host: `node research/fl-enr06r1-host/session.cjs` (default run4; wajib pakai nama run baru melalui `FACELINK_R1_RUN` agar evidence lama tidak ditimpa). Gunakan helper device hanya setelah izin baru; compile `ui/DumpUI.java` bersama `ui/android/test/RepetitiveTest.java` dengan classpath Android platform android.jar, uiautomator.jar dan JUnit4, dex melalui d8, dan push JAR ke path yang dipakai helper. File `session.cjs` bukan one-command Android acceptance: interaksi UI/OTP tetap serial dan APK audit/izin tetap prasyarat. Shutdown melalui control `shutdown`; bukan menghentikan sistem/device global.

Keputusan yang dibutuhkan: prioritaskan tugas diagnosis/remediasi crash dev inspector dan penerimaan lifecycle yang tersisa; setiap rebuild/install/ADB berikut memerlukan scope serta izin baru. **Tidak memulai FL-UI01, human enrollment, production atau task berikutnya otomatis.**

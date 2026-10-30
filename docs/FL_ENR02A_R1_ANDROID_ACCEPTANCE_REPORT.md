# FL-ENR02A-R1 — Physical Android Synthetic Enrollment UI Acceptance

Tanggal: 26 Oktober 2026. **STATUS KESELURUHAN: PARTIAL.**

Alur inti enrollment sintetis pada TECNO LJ7 dan backend HTTPS nyata **PASSED** untuk skenario yang dijalankan. Satu warning lifecycle host `expo-video` tetap ada; validasi TalkBack/accessibility menyeluruh tidak dijalankan. Tidak ada fitur baru, capture kamera/manusia, enrollment production, migrasi database Product Owner, atau pekerjaan FL-ENR02B.

## A. Baseline & preservation — PASSED

- Branch `feat/fl-otp01-email-auth`; HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Baseline R1: staged0, modified35, untracked47 entri. Seluruh pekerjaan uncommitted dipertahankan; tidak reset/stash/clean/commit/push/merge.
- Laporan ENR02A, kontrak ENR01B API §5.3–5.4, otorisasi §4.4–4.5 dan model data consent/submission dibaca. `cmd/server` tetap tidak menginjeksi enrollment; nil service tidak mendaftarkan route.
- Product Owner memberikan izin baru secara eksplisit: **“Tersedia — izinkan pengujian”**, sebelum ADB/install.
- Main APK SHA256 awal/akhir sama: `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`. Main `ai.facelink.mobile` tidak ditimpa atau dihapus.
- Enam setting HP awal/akhir identik: brightness4, mode0, timeout1800000, accelerometer_rotation1, user_rotation0, stay_on_while_plugged_in15. Nilai hanya dibaca, bukan diubah.

## B. Device and staging package — PASSED

TECNO LJ7 fisik, portrait1080×2460. Density teramati physical440/override432; tidak disetel oleh task ini.

APK staging `ai.facelink.enr02host`, versi0.1.0, ARM64 debug Development Build:

-100190664 byte; SHA256 `62751219c14fb1f104f3ceb2ba5170eb55c16e5727ab7c9054632174a67bea89`.
- Sebelum task staging belum terpasang; `adb install -r` berhasil hanya untuk APK staging.
- `aapt` sebelum install dan package inspection perangkat sesudah pengujian tidak menemukan permission CAMERA. APK juga tanpa media/storage/overlay permission.
- CAMERA app-op terbaca `Uid mode: CAMERA: ignore`; task tidak mengubah app-op/permission. Tidak ada kamera yang dibuka.
- Aplikasi staging tetap terpasang tetapi force-stopped saat selesai; tidak di-uninstall.

## C. HTTPS staging connection — PASSED

- Runtime baru dengan SQLite WAL, akun/roster sintetis, kunci ephemeral test-only, JPEG pola sintetis dan processor sintetis. Tidak menggunakan database atau akun nyata Product Owner.
- API hanya `https://127.0.0.1:8091`. ADB reverse device8091 ke host8091; koneksi API aplikasi tetap HTTPS tervalidasi dengan CA development di debug app.
- `openssl verify -CAfile dev/certs/dev-ca.pem -verify_ip 127.0.0.1 dev/certs/server.pem`: OK. Node HTTPS client menggunakan CA eksplisit dan `rejectUnauthorized:true`; permintaan health/OTP/review berhasil. Tidak ada `InsecureSkipVerify`, `-k`, atau pemadaman certificate validation.
- Curl Windows Schannel awal gagal karena revocation status tidak tersedia. Tidak di-bypass; bukti koneksi berikutnya menggunakan Node HTTPS dengan validasi CA/identity dan aplikasi Android aktual.
- Metro staging memakai device8089 → host8090 relay IPv4 → Metro IPv6 localhost8089. Galat awal development-client EOF berasal dari binding IPv4/IPv6, diperbaiki lewat relay loopback, bukan perubahan auth/TLS atau source aplikasi.
- Fault injection API memakai raw TCP relay loopback8093 tanpa terminasi TLS. Relay dihentikan untuk memutus socket; tidak mengubah Wi-Fi/data/global setting HP. Penghapusan reverse saja awalnya inconclusive karena socket lama bertahan; tidak dihitung sebagai bukti offline.

## D. UI screens tested — PASSED untuk skenario berikut

| Layar/aksi fisik | Observasi |
|---|---|
| Startup/login | Development Build terbuka; mahasiswa sintetis login Email OTP normal, dibantu input Product Owner |
| Intro | Consent belum diberikan, enrollment belum terdaftar; staging/synthetic label terlihat |
| Notice | Teks `synthetic-v1` penuh dapat di-scroll; explicit checkbox diperlukan |
| Preparation | Panduan portrait, satu wajah, synthetic/no-camera dan tombol mulai terlihat |
| Guided scan | Oval portrait, instruksi, progress dan cancel; tanpa image preview/camera permission |
| Pending | Menunggu verifikasi Admin; bukan ACTIVE; TTL terlihat |
| Rejected | Pengajuan ditolak, reason aman, reload dan fresh-consent retry tersedia setelah deletion complete |
| Active | Wajah terdaftar hanya setelah backend ACTIVE dan template_ready=true |
| Error/offline | Aktif (belum terverifikasi), stale warning dan retry, bukan heading Wajah terdaftar |
| Withdrawn | Persetujuan ditarik/Dicabut, tidak mempertahankan ACTIVE |
| Expired | Pengajuan kedaluwarsa, PHOTO_RETENTION_EXPIRED, foto dihapus, opsi persetujuan baru |

Screenshot PNG dan metadata tersimpan lokal di direktori ignored `research/fl-enr02a-host/evidence-local/r1/`; tidak ada foto kamera/manusia sebagai payload enrollment. Media video login existing tidak diubah; screenshot login yang sempat memuat video tidak dipertahankan. Evidence gambar enrollment hanya layar simulasi tanpa wajah.

## E. Consent behavior — PASSED

- Login tidak memberi consent otomatis. Consent ditampilkan terpisah dari enrollment.
- Tombol Setujui & lanjutkan disabled sebelum checkbox; enabled sesudah explicit acceptance.
- Grant dari perangkat tersimpan melalui HTTPS. Tiga grant sintetis tercatat pada database runtime perangkat.
- Setelah rejection, UI mula-mula menyediakan reload selama cleanup; sesudah COMPLETE menyediakan persetujuan baru.
- Renewal tetap menampilkan terminal submission lama, lalu `SUBMIT` backend mengaktifkan Mulai pemindaian. Retry perangkat menghasilkan submission baru, bukan menghidupkan record rejected.

## F. Simulated scanning — PASSED

- Frame berasal dari fixture JPEG pola sintetis, bukan sensor. Guided progress dan oval diamati pada HP.
- Hardware Back saat scan kembali ke preparation; queue tetap0.
- HOME saat scan membatalkan workflow lokal; resume kembali preparation; queue tetap0.
- Scan yang diselesaikan mengirim satu submission ke backend; tidak memakai shutter manual sebagai jalur utama.
- Tidak mengklaim deteksi wajah, recognition, PAD, liveness atau akurasi model nyata.

## G. Pending / Reject / Expire / Active — PASSED

Tiga submission runtime perangkat menghasilkan state akhir **REJECTED1, REVOKED1, EXPIRED1**.

1. Submission perangkat pertama: PENDING_APPROVAL → Admin fixture reject200 → Pengajuan ditolak pada HP.
2. Fresh consent/retry: submission berbeda → PENDING_APPROVAL → Admin approval202 → worker → ACTIVE/template_ready=true → Wajah terdaftar pada HP.
3. Setelah withdrawal dan grant baru: pending ketiga → clock fixture dimajukan73jam → backend EXPIRED. Deadline awal tetap `submitted_at+72h`; tidak mengubah timestamp retensi di SQL. Untuk mengatasi cooldown OTP yang clock-nya dibekukan, offset kemudian maju74/75jam. Setelah login ulang normal, HP menampilkan Pengajuan kedaluwarsa/PHOTO_RETENTION_EXPIRED. Jam HP tidak diubah.

Automated HTTPS terpisah juga membuktikan batas72jam+1detik dan replay pada23jam tidak memperpanjang TTL. Itu bukan observasi layar, dan bukan menggunakan runtime/DB perangkat yang sama.

## H. Withdrawal — PASSED

- Tombol perangkat membuka konfirmasi native BATAL/TARIK.
- TARIK menghasilkan Persetujuan ditarik/Dicabut pada HP, kemudian HTTPS mengonfirmasi REVOKED/template_ready=false/deletion COMPLETE.
- Pembacaan backend yang dilakukan segera sesudah tap sempat mendahului commit dan masih ACTIVE; tidak dipakai sebagai bukti withdrawal final. Sesudah respons perangkat, backend mengonfirmasi REVOKED.
- Akhir runtime:0 template,0 pending-photo rows. Audit withdrawal/revocation/deletion tersimpan. `consent.withdrawn` berjumlah2 mencakup replacement consent sebelumnya, bukan dua klik withdrawal oleh pengguna.

## I. Accessibility / layout — PARTIAL

- Layout portrait, judul, notice, instruksi, tombol, status dan pesan error diamati lewat screenshot serta hierarchy aktual. Teks notice panjang memakai scroll; label tombol panjang membungkus, tidak dipotong.
- Tidak terlihat overlap antartombol pada layar inti yang diperiksa. Developer menu bubble milik Development Build menutupi sebagian ikon staging kanan atas, bukan tombol workflow.
- A11y hierarchy menunjukkan role Button, label Back/scan/consent, disabled/enabled button dan `android.widget.CheckBox` dengan checkable=true/checked=false sebelum acceptance.
- Tidak mengaktifkan TalkBack atau mengubah font/rotation/global setting. Urutan baca screen-reader, magnification, ukuran font alternatif dan accessibility menyeluruh **NOT_RUN**. Tidak mengklaim accessibility PASS penuh.

## J. Android lifecycle — PARTIAL

- Startup, hardware Back, HOME/resume saat scan, force-stop/cold relaunch dan pemulihan sesi mahasiswa saat masih valid berhasil. State backend ACTIVE dibaca ulang setelah relaunch.
- Setelah clock test maju>72jam, sesi device ditolak secara sah; login ulang OTP normal diperlukan dan berhasil dengan bantuan Product Owner. Tidak ada pembacaan token SecureStore, injeksi sesi, atau auth bypass.
- Saat lifecycle teramati LogBox non-fatal: `Cannot set prop 'player' ... TextureVideoView` / `Cannot use shared object that was already released`. Aplikasi tetap dapat menjalankan workflow. Warning yang sama telah dicatat sebelum R1 di `ANDROID_SETUP.md` §17d.
- Analisis independen menemukan jalur `useVideoPlayer`/native player prop sebagai area terkait, tetapi urutan race native belum terbukti. Observer CAM03 hanya instrumentasi dan bukan fix. Tidak menyembunyikan warning, menghapus video, atau mengubah lifecycle SDK/main berdasarkan dugaan.
- Tidak ada FATAL EXCEPTION pada sampel log yang diperiksa; ini bukan jaminan semua lifecycle bebas crash.

## K. Regression and security — PASSED untuk scope yang diuji

| Jenis bukti | Perintah/skenario | Hasil |
|---|---|---|
| Automated backend HTTPS | `FACELINK_R1_BACKEND=1 go test ./internal/httpapi -run '^TestR1HTTPSAcceptance$' -count=1 -v` |3 subtest lulus: GrantReplayRejectRenew, ApproveActiveWithdraw, PendingExpiry72Hours |
| Device UI + actual API | Email OTP, consent, scan/submission, reject/retry, ACTIVE, offline-stale, withdrawal, expiry | Teramati terpisah pada HP dan respons HTTPS |
| Main mobile regression | `npm run typecheck && npm run lint && npm test` |139 passed,1 opt-in E2E skipped |
| Backend setelah harness direstorasi | `go test -count=1 ./... && go vet ./...` |16 paket uji lulus,5 tanpa tes; vet lulus |
| Smoke backend setelah cleanup | `go run ./cmd/enrollment-synthetic` | HTTPS nyata; grant201/submit201/replay200/approve202/active200/withdraw200; preview Dosen/Mahasiswa403; manual tanpa key201; akhir0 template/0 ciphertext |

Automated HTTPS: grant201, submit201, replay200 ID sama/count1, approval202, reject/regrant, revoke, TTL72h immutable, expired preview410/approval409. Mahasiswa/Dosen ditolak403 pada preview/approve/queue/detail; self selector injection400. Audit actor/entity/count dan protected-log assertions lulus. Setup automated memakai harness auth sintetis existing dan sesi fixture baru setelah time advance; bukan bukti OTP. OTP dibuktikan terpisah pada runtime perangkat/Admin fixture melalui endpoint existing.

Snapshot audit runtime perangkat: submitted3, approved1, activated1, rejected1, revoked1, expired1, photo.deleted3; akhir status1 rejected/1 revoked/1 expired,0 foto/0 template. Sampel log PID yang diperiksa tidak memuat marker JPEG/base64 payload atau field access/refresh token. Tidak mengklaim zero-telemetry/forensic log proof seluruh SDK.

Security review independen terhadap runtime/harness tidak menemukan actionable issue dalam scope. Prod server tetap OFF; tidak ada endpoint test-clock atau bootstrap auth baru.

**Cleanup teramati:** staging force-stopped, HTTPS shutdown exit0, Metro/relay dihentikan, reverse mappings kembali kosong, XML UI sementara di HP dihapus. Root disposable aktif dan root pertama yang tertinggal setelah runtime sebelumnya berhenti sudah dihapus. Staging tetap terpasang. APK historis, video, evidence, main app dan database Product Owner dipertahankan.

Tidak ada perubahan source/auth permanen dari R1. Injeksi clock dan dua test harness sementara diarsipkan sebagai evidence ignored di luar source/build path; `main.go` dipulihkan ke clock.Real dan signature semula. Laporan ini, PROJECT_STATE, TEST_PLAN, BACKLOG dan ringkasan README adalah perubahan dokumentasi task.

## L. Remaining defects / limits

1. **OPEN — warning lifecycle expo-video host.** Dapat muncul sebagai LogBox yang menutupi bagian bawah layar debug. Exact native interleaving belum terisolasi; perlu tugas reproduksi/fix tersendiri yang mempertahankan video dan main app. Tidak diperbaiki atau disamarkan oleh task validasi ini.
2. **NOT_RUN — accessibility menyeluruh/TalkBack**, varian font/orientasi, device release-build/embedded-JS khusus R1, app storage forensic proof dan long-run endurance.
3. **Harness limit — fake clock membekukan cooldown OTP.** Beberapa resend tidak menghasilkan kode baru; clock fixture dimajukan monoton sebelum permintaan normal berikutnya. Tidak mengubah rate-limit/auth production. Kode fixture diminta pemilik untuk input manual; tidak dimasukkan ke laporan/evidence publik.
4. **Tidak diuji:** human capture, YuNet/SFace nyata, PAD/recognition accuracy, production, power-loss, migrasi Product Owner. Konflik kebijakan D-030 tidak diselesaikan oleh validasi ini.

## M. GO / NO-GO for FL-ENR02B

**GO terbatas:** hasil ini dapat dipakai sebagai baseline review desain/integrasi sintetis FL-ENR02B. Alur inti UI→HTTPS backend sintetis sudah terbukti pada perangkat.

**NO-GO:** memulai FL-ENR02B tanpa tugas/izin baru, mengklaim seluruh acceptance tanpa residual, mengaktifkan capture/enrollment manusia atau production, migrasi database Product Owner, ESP-CAM, commit/push/merge atau PR19.

**Rekomendasi berikut:** triage/reproduksi lifecycle `expo-video` host dan lengkapi accessibility yang belum diuji; Product Owner meninjau residual sebelum memberikan tugas berikutnya. **STOP — FL-ENR02B tidak dimulai.**

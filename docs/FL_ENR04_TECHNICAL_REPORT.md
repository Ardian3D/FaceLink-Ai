# FL-ENR04 — Python YuNet/SFace Secure Go Backend Integration

Tanggal: 27 Oktober 2026. **Status: PARTIAL.** Integrasi lokal sintetis/disposable dan real-model negative path terbukti. **REAL_MODEL_POSITIVE_NOT_PROVEN.** Tidak ada izin manusia, production, kamera, face attendance, atau FL-ENR05 otomatis.

## A. Post-merge Git baseline dan preservation

PR [#19](https://github.com/Ardian3D/FaceLink-Ai/pull/19) terverifikasi merged pada `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`. Tree origin/main identik dengan shared merge-base `aa36f2d243f8761879baf38a5b2735afab8f1b52`; lokal tetap branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`, dengan21commit lokal. Baseline FL-ENR04:55 tracked modified/178untracked, sebelum berkas tugas ini; jangan menyamakan angka itu dengan perubahan tugas ini. Pekerjaan FL-CORE/FL-CAM/FL-ENR/FL-STAB dan evidence lama dipertahankan. Tidak menjalankan reset/stash/clean/checkout/pull/merge/commit/push. Tidak menganggap pekerjaan lokal sudah masuk main.

## B. Arsitektur Python dan dependency

Reuse `Detektor`, `Embedder`, dan `nilai_frame` existing, bukan engine kedua. Modul `pipeline.py` menambah validasi JPEG sebelum decode, checksum dua model sebelum native load, quality/landmark guard dan validasi output. `ipc.py` menerima satu request dan keluar; tidak membuka port. Go meluncurkan child baru untuk setiap inference: overhead startup/model load diterima demi cancellation/reaping sederhana dan lifetime data terbatas pada satu pekerjaan. Maksimum satu call per processor; enrollment juga memegang slot/lease durable existing.

Runtime yang dijalankan: Windows10.0.26200 x64, Python3.12.14, OpenCV4.12.0 (`opencv-python==4.12.0.88`), NumPy2.2.6, pytest8.4.2. Dependensi tidak ditambah; semuanya lokal/gratis. OpenCV threads1 dan OpenCL dimatikan pada pipeline. Tidak ada Google Cloud, LLM, telemetry, API berbayar, atau download bobot baru.

## C. Provenance, lisensi, checksum

| Model | Artefak lokal | Byte | SHA-256 | Lisensi resmi |
|---|---|---:|---|---|
| YuNet | face_detection_yunet_2023mar.onnx |232589|8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4|[MIT, OpenCV Zoo](https://github.com/opencv/opencv_zoo/blob/main/models/face_detection_yunet/LICENSE)|
| SFace | face_recognition_sface_2021dec.onnx |38696353|0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79|[Apache-2.0, OpenCV Zoo](https://github.com/opencv/opencv_zoo/blob/main/models/face_recognition_sface/LICENSE)|

Hash byte lokal diperiksa ulang terhadap pin downloader/config existing; lisensi resmi dibaca dari upstream. Pipeline memeriksa ulang kedua checksum sebelum membuat runtime. Weights tetap di lokasi models yang diabaikan Git; tidak diubah/ditimpa/di-commit. Hashing dan loading membuka path terpisah: direktori model harus dipercaya dan tidak writable oleh pihak tidak tepercaya selama operasi.

## D. Implementasi deteksi YuNet

Decode hanya JPEG8bit baseline/progressive, grayscale/3komponen yang memenuhi batas1MiB,2.073.600pixel dan sisi4096. SOF diperiksa sebelum alokasi OpenCV; hasil decode harus sama dimensinya. APP1 ditolak, bukan menerapkan orientasi EXIF diam-diam. Canonical Go existing sudah membatasi sisi1920/minimal112 dan re-encode tanpa metadata sebelum penyimpanan/enkripsi.

Deteksi memakai threshold existing tanpa penurunan. Baris hasil harus15nilai finite; jumlah dibatasi1024. Bounding box, skor, lima landmark berbeda/nonkolinear, koordinat dalam frame/box dan konsistensi row diperiksa. Tidak ada wajah → NO_FACE; lebih dari satu → MULTIPLE_FACES; tidak memilih wajah terbesar untuk memaksakan sukses. Quality gate existing menolak wajah terlalu kecil, buram, gelap/terang.

## E. Alignment dan ekstraksi SFace

`alignCrop` menerima row YuNet lengkap, bukan crop kotak atau landmark palsu. Crop wajib112×112×3 uint8, output feature wajib128nilai finite; normalisasi L2 dan dtypefloat32. Pipeline memvalidasi unit norm toleransi1e-5. Tidak ada vektor acak/fallback di processor nyata. Tes domain boleh memakai dependency deterministik, tetapi tidak dihitung sebagai bukti model positif.

## F. Secure Go/Python IPC

Kontrak terimplementasi: `API_CONTRACT.md` §6.9. Private stdin/stdout, tanpa socket Python. Frame uint32BE panjang + HMAC-SHA25632byte + JSON; request≤1.400.000byte, response≤16.384byte. Secret acak32byte, UUIDv4 dan artefak digest baru per child. Secret bukan CLI/URL/log; environment child hanya variable runtime yang diperlukan, thread limits, dan secret. Python menghapus variable secret setelah membaca.

Go memeriksa HMAC, exact schema/non-null/duplicate/unknown/trailing bytes, version, UUID, digest, seluruh manifest, face count, finite128 dan unit norm. Python memverifikasi HMAC sebelum JSON/base64 dan digest sebelum inference. Invalid response/process crash/overflow/timeout tidak menghasilkan sukses. Deadline≤10detik, `exec.CommandContext`, bounded output, `WaitDelay`, `Run` menunggu/reap; slot tetap ditahan sampai child selesai. Tidak ada retry otomatis atau fallback ke fixture.

Trust boundary: executable Python, source, native library, model path, dan akun OS adalah trusted operator input. IPC HMAC bukan sandbox untuk child yang sudah terkompromi, bukan pertahanan terhadap administrator OS. Tidak ada hard OS-memory cap atau larangan jaringan/filesystem tingkat OS; implementation normal tidak menggunakannya untuk payload. Memori Python/Go/native dan string environment tidak dijamin ter-zeroize.

## G. Integrasi enrollment processor

`enrollmentcv.New(pythonExecutable,engineDir,modelDir)` mengimplementasikan `enrollment.Processor`. Hanya command `enrollment-staging-serve` menerima tiga flag opt-in `-cv-python`, `-cv-engine-dir`, `-cv-model-dir`; partial configuration ditolak. Kombinasi dengan `-admin-fixtures` ditolak karena seed/control Admin itu deterministik. Default command tetap fixture existing; bila real processor dipilih, error tidak beralih kembali ke fixture.

Tidak ada injeksi pada `cmd/server`, endpoint baru, migration/schema baru, perubahan OTP/session/role/mobile, atau migrasi database PO. Go tetap memegang approval, consent, identitas/roster, reviewer, hash, revision/epoch, attempt/lease, ciphertext, audit dan aktivasi. Semua guard diperiksa kembali pada transaksi final. Approval202 hanya APPROVED_PROCESSING. Staging shutdown membatalkan dan menunggu worker sebelum menutup/menghapus DB/root.

## H. Template terenkripsi

Reuse FL-ENR01B:128float32-le diserialisasi menjadi512byte lalu AES-256-GCM purpose `template`, key terpisah dari `pending-photo`, nonce unik dan AAD person/model/version/key-version existing. Tidak ada plaintext persistent atau embedding ke JavaScript/API/log. Transaksi menyatukan template/state/audit/purge; key unavailable dan audit failure tetap fail-closed. Tests existing activation/encryption/withdrawal/reconciliation tetap berjalan.

Bukti enkripsi sukses berasal dari domain fixture deterministik, **bukan** real YuNet-positive→SFace→ACTIVE pada task ini. Tidak mengubah D-030/recovery/retention policy.

## I. Similarity/matching interface

`pipeline.similarity(a,b,a_manifest,b_manifest)` menerima dua vektor unitfloat32/128 dengan model/pipeline fingerprint yang sama dan mengembalikan cosine[-1,1]. Skor bukan probabilitas identitas. Tidak ada production threshold, kandidat manusia, FAR/FRR, atau keputusan attendance. Modelversion/checksum/pipeline harus kompatibel; kalibrasi dan quality-gate evaluation tetap prasyarat pemilihan threshold produksi. Tidak ada PAD/liveness.

## J. Tes sintetis dan real-model smoke

| Kategori | Bukti nyata |
|---|---|
| Python domain/contract | JPEG invalid/oversize/forged dimensions/EXIF/trailing, quality, multiple/no face, landmark invalid, NaN/Inf/dimensi/norm, checksum, similarity fingerprint, framing/auth/schema/digest/truncation/error sanitization |
| Go IPC helper | Auth/schema/nonce/digest/manifest, NaN/dimensi, process crash/overflow, cancellation/deadline/singleflight dan environment minimal |
| Enrollment domain | Lease expiry/replacement, immutable job binding, reviewer/subject revoked-restored, failure status/audit/no template; existing withdrawal/audit/key/restart/expiry tests |
| IPC nyata → Python → pinned model | Generated320×320 non-human JPEG: authenticated NO_FACE/face_count0/no embedding. Invalid JPEG→INVALID_IMAGE; missing/corrupt model→MODEL_UNAVAILABLE; bad HMAC/oversize→exit1 tanpa response/stderr |
| Executable Go → Python → model → domain | Staging disposable aktual: consent/upload/approve202 → FAILED_PROCESSING/NO_FACE; template_rows0, READY_photo_rows0, processing.failed1, photo.deleted1; root dihapus saat shutdown |

Real-model negative IPC total763,92ms pada satu smoke. Approval sampai failure domain1303,55ms pada satu smoke. Nilai tidak diklaim SLA. Model ter-load dan YuNet benar-benar dijalankan. SFace alignment/feature pada tes existing memakai landmark sintetis; itu component execution, bukan bukti YuNet menemukan wajah sah. **REAL_MODEL_POSITIVE_NOT_PROVEN.** Tidak memakai wajah manusia, data biometrik lama, kamera, atau menurunkan threshold untuk memaksa PASS.

## K. Security/failure review

Dua review independen read-only: security boundary dan lifecycle/domain. Tidak ada defect actionable yang terbukti pada trust boundary yang ditinjau; kesimpulan static bukan bukti universal. Parent smoke nyata menutup gap actual Go→Python→model minimal-environment yang disebut reviewer. Residual: tidak ada pengujian OS-level untuk descendant worker yang menahan inherited pipes atau shutdown di tengah CV child nyata; helper tests dan source `Run`/WaitDelay/workerDone membatasi jalur itu, bukan bukti semua native race hilang.

Run awal Go gagal pada4tes baru karena tes berusaha mengubah binding job yang sudah dilindungi trigger immutable. Tes diperbaiki untuk membuktikan write ditolak DB dan inference sah tetap dapat selesai; trigger/schema produk tidak diubah. Initial command pytest memakai path relatif yang tidak ditemukan; diulang memakai executable/path absolut dan lulus. E2E awal gate mahasiswa salah nama sehingga skipped; dijalankan lagi dengan `FACELINK_ENR_E2E=1` dan lulus. Tidak menyembunyikan kegagalan awal sebagai PASS.

## L. Resource/performance

Workstation: AMD Ryzen7 170 with Radeon Graphics,8core/16logical, RAM15,19GiB. Pengukuran satu proses Python aktual setelah import: model verification+init122,21ms; JPEG encode+YuNet negative140,99ms; peak working set126,59MiB; OpenCV threads1. CPU-time/working-set yang terbaca dari executable launcher venv hanya mengukur launcher dan **tidak digunakan**. Tidak mengukur SFace positive-path latency, throughput kelas, p95, FMR/FNMR, atau memori maksimum adversarial. Tidak ada hard memory sandbox; batas input/output/concurrency/time bukan pengganti evaluasi native resource lengkap.

## M. Regresi dan hasil

| Perintah | Hasil yang dijalankan |
|---|---|
| Python executable venv absolut `-m pytest .../services/face-engine/tests` |208passed,1skipped; skip existing karena gambar acak tidak menghasilkan deteksi positif |
| `go test -count=1 ./... && go vet ./...` di backend |18paket ok,4tanpa tes; exit0, mencakup OTP/session regressions |
| `npm run typecheck && npm run lint && npm test` di apps/mobile |Lulus;219passed/6skipped,14files passed/2skipped |
| Perintah sama di research/fl-enr02a-host/stage |Lulus;219passed/6skipped |
| `FACELINK_ADMIN_ENR_E2E=1 NODE_EXTRA_CA_CERTS=.../backend/dev/certs/dev-ca.pem npx vitest run tests/adminEnrollmentE2E.test.ts` |HTTPS actual Go+SQLite disposable5/5passed |
| `FACELINK_ENR_E2E=1 npx vitest run tests/enrollmentE2E.test.ts` |HTTP mahasiswa disposable1/1passed |
| Actual IPC/model dan actual Go staging smoke temporer |Hasil §J; root disposable dihapus |

Tidak ada ADB/Android build/perangkat karena perubahan hanya Go/Python/dokumentasi. Tidak ada claim UI/production acceptance baru.

Berkas tugas:
- Baru: `backend/internal/enrollmentcv/{processor.go,processor_test.go}`, `backend/internal/enrollment/processor_contract_test.go`.
- Backend existing: `internal/enrollment/processor.go` (allowlist status), `cmd/enrollment-staging-serve/{main.go,main_test.go}` (opt-in + callers + shutdown worker).
- Python: `face_engine/{pipeline.py,ipc.py}` baru; guard finite/crop/row di `detector.py`, `embedder.py`; tests baru `test_pipeline.py`, `test_ipc.py`.
- Dokumentasi: laporan ini, root README/PROJECT_STATE, backend dan engine README, API_CONTRACT, ARCHITECTURE, FACE_ENGINE, SECURITY_PRIVACY, TEST_PLAN, BACKLOG.

## N. Residual dan blocker

- Real-model positive detection→quality→SFace→encrypted ACTIVE belum terbukti karena tidak ada fixture sintetis sah yang diterima detektor. Tidak diganti fake inference.
- D-030 recovery timer belum direkonsiliasi; retention insiden/backup belum final. Tidak diubah.
- Preview bitmap cache belum disertifikasi aman untuk manusia; mirror/crop absolut dan lifecycle background encoding masih mempunyai gap terdahulu.
- Accessibility keseluruhan belum PASS; F3 dialog Admin sesaat nonaktif saat refresh tetap OPEN dan tidak diubah di task ini.
- Threshold recognition belum dikalibrasi; tidak ada PAD/liveness. Human enrollment tetap belum diizinkan.
- Model/native runtime/local paths dipercaya; tidak ada OS sandbox, hard memory cap atau jaminan zeroization.

## O. GO/PARTIAL/NO-GO menuju FL-ENR05

**PARTIAL:** real local inference boundary dan fail-closed domain integration tersedia serta terverifikasi untuk negative path. **GO terbatas untuk review/perencanaan FL-ENR05 oleh Product Owner**, bukan memulai pekerjaan atau live recognition. Positive pipeline perlu fixture sintetis sah dan bukti nyata sebelum dinyatakan PASS. **NO-GO manusia/production/face attendance/migrasi PO**; bukan izin membuka kamera, enrollment production, commit/push/merge atau deployment. **STOP setelah laporan; FL-ENR05 tidak dimulai otomatis.**

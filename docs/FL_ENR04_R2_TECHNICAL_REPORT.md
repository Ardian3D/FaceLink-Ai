# FL-ENR04-R2 — Bounded Synthetic Positive-Path Investigation

**Hasil fungsional: PASS — REAL_MODEL_POSITIVE_PROVEN. Rekomendasi keseluruhan: NO-GO menuju FL-ENR05 karena celah authenticated-record binding AAD v1 terkonfirmasi.** Pemeriksaan selesai; bukan izin manusia, production, kamera, migrasi Product Owner, atau remediasi kontrak ciphertext tanpa persetujuan terpisah.

## A. Git baseline dan preservation

Baseline sesi R2: branch `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`, origin/main `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`, merge-base `aa36f2d243f8761879baf38a5b2735afab8f1b52`, divergence21/1. Inventaris sebelum perubahan R2: **60 tracked modified,189 untracked**. Branch/HEAD/origin-main dibaca ulang saat melanjutkan dan tetap sama. PR#19 sudah merged; tidak di-merge ulang.

Seluruh pekerjaan FL-CORE/FL-CAM/FL-ENR/FL-STAB, source/evidence lama, APK/video dan perubahan uncommitted dipertahankan. Tidak ada reset/stash/clean/pull/switch/rebase/commit/push/merge. Git portable sesi existing dipakai untuk pembacaan baseline; reviewer tidak dapat memeriksa diff secara independen karena Git tidak ada di PATH shell review. Tidak mengklaim status worktree final bersih atau identik baseline: perubahan R2 memang belum di-commit.

## B. Diagnostic input pipeline

Preflight membaca keenam JPEG R1 existing, memverifikasi hash masing-masing terhadap screening historical, lalu membandingkan wrapper dengan `cv2.FaceDetectorYN` langsung. Model/checksum, threshold0,75, NMS0,3 dan topK5000 sama.

- Decode `cv2.imdecode`: BGR uint8, shape512×512×3; width/height diberikan ke `setInputSize` dalam urutan yang benar.
- Ordinary decode sama dengan decode yang mengabaikan orientation pada semua fixture; tidak ada EXIF APP1 pada fixture ini. Normalisasi Go tetap menolak orientasi ambigu dan menghapus metadata.
- Tidak memakai `blobFromImage`, swapRB, crop wajah baru, landmark buatan atau bounding box injeksi. Pembalikan channel hanya kasus diagnostik, bukan jalur acceptance.
- Wrapper lama selalu menskalakan sisi panjang ke960, termasuk input kecil512. Rasio square tetap benar, tetapi skala wajah relatif terhadap fitur model berubah.
- Pada input960, wrapper dan OpenCV langsung sama-sama menghasilkan NO_FACE untuk6/6. Pada native512, OpenCV langsung mendeteksi tepat satu wajah untuk6/6, confidence0,904450–0,909020. Jadi ketidakcocokan wrapper/API, orientasi dan channel bukan penyebab utama yang ditunjukkan; pembesaran yang tidak diperlukan adalah penyebab yang direproduksi.

Perbaikan `ukuran_masukan`: frame dengan sisi panjang≤960 dipertahankan native; downscale frame besar dan pemetaan koordinat existing tetap. Threshold, quality gate, model dan dependensi tidak diubah. Fingerprint preprocessing Go/Python menjadi **`enr04-synthetic-native-v2`**; fingerprint lama tidak diterima sebagai kompatibel.

Regresi baru: square512, rectangular640×480, non-multiple-of-32 321×257, dan real-model procedural positive. **4FAIL sebelum perbaikan**, termasuk real pipeline NO_FACE; **4PASS sesudah**. Tidak menggeneralisasi keberhasilan ini sebagai peningkatan akurasi seluruh ukuran atau wajah manusia.

Referensi API resmi yang dibaca:
- https://github.com/opencv/opencv_zoo/blob/main/models/face_detection_yunet/yunet.py
- https://github.com/opencv/opencv_zoo/blob/main/models/face_detection_yunet/demo.py

Evidence pre-fix: `research/fl-enr02a-host/evidence-local/enr04-r2/input-diagnostic.json`; tidak ditimpa sesudah perbaikan.

## C. Fixture provenance

**Kandidat unik baru:0.** Temuan preflight sudah memungkinkan penggunaan fixture existing; tidak membuat tiga kandidat kosmetik, mencari fixture tambahan, mengubah generator atau menjalankan rangkaian R3/R4.

Fixture positif: kandidat01 `procedural-mannequin-heightfield-v1`,512×512. Pixel berasal dari persamaan height-field2,5D, normal/shading, seed NumPy dan primitive drawing OpenCV lokal yang telah discreen pada R1. Tidak berasal dari foto/scan/mesh/texture manusia, dataset identitas, mahasiswa, selfie, face swap, internet, generator terlatih atau layanan eksternal. Generator/output berlisensi CC0-1.0 fixture-only; lisensi repository/model tidak diubah. Provenance tidak berubah karena bytes yang diproses sama.

- Fixture SHA256: `cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe`.
- Go canonical JPEG SHA256: `00eaa7792b75bd3f7f2ad10bfcbbb5fd3afe64fec4b626761a0e42b239cace76`.
- Semua fixture/evidence tetap di `evidence-local` yang ignored. Tidak memasukkan JPEG, embedding atau weights ke Git. Hash mengikat bytes, bukan detektor apakah asset manusia; operator tetap harus memakai direktori fixture yang diotorisasi.

## D. YuNet detection results

Model nyata YuNet2023mar, SHA256 `8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4`. Model diverifikasi sebelum native runtime dibuat. Threshold tetap0,75.

Pengukuran positive fixture01 pada jalur wrapper setelah perbaikan: **satu wajah, confidence0,907666, landmark valid, quality OK**. Bukti native-positive semua enam pada preflight adalah bukti deteksi; full positive lifecycle hanya diklaim untuk kandidat01. Tidak menggunakan hasil channel-reversed sebagai acceptance.

## E. SFace positive results

SFace2021dec nyata, SHA256 `0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79`. Lima landmark berasal dari deteksi YuNet asli. `FaceRecognizerSF.alignCrop` menghasilkan112×112×3; `feature` asli menghasilkan embedding128float32, finite dan unit-normalized. Pipeline normal juga memverifikasi geometri/quality/vector, bukan hanya panggilan komponen langsung.

Embedding, crop, JPEG payload dan key tidak dicetak atau disimpan dalam evidence. `fill(0)` pada array yang dimiliki skrip bukan klaim zeroization seluruh memori/native runtime.

Pengukuran satu run pada CPU lokal; bukan SLA, benchmark akurasi atau hard memory limit:

| Tahap | Hasil |
|---|---:|
| Model initialization |104,206ms|
| JPEG decode |1,018ms|
| YuNet |23,409ms|
| Alignment |0,207ms|
| SFace extraction |56,447ms|
| Normal full pipeline, model sudah loaded |50,600ms|
| Authenticated Python IPC round trip, child sekali-pakai |509,117ms|
| Go ProcessNext, canonical fixture |498ms|
| Lifecycle HTTP/DB/inference test total |877ms|
| Python peak working set terukur |146,047MiB|

Panggilan komponen dan full pipeline diukur terpisah sehingga tidak dijumlahkan. IPC child exit0 dan sudah direap. Python3.12.14/OpenCV4.12.0/NumPy2.2.6 tetap; runtime/model tidak diganti.

## F. Go/Python encrypted activation

`TestEnrollmentRealModelSyntheticLifecycle` memakai SQLite WAL disposable, akun/person/roster/identity/consent sintetis, purpose keys acak test-only yang berbeda dan concrete `enrollmentcv.Processor`. Tidak ada fallback fake processor. Opt-in kosong tetap explicit SKIP; konfigurasi parsial/invalid FAIL; positive failure tidak diturunkan menjadi PASS.

Seluruh18request lifecycle melintasi **`httptest.NewTLSServer` loopback nyata**. Client memakai root pool eksplisit untuk sertifikat server uji, memverifikasi hostname/chain/peer, menolak redirect, tanpa `InsecureSkipVerify`. Harness auth existing memakai JWKS lokal sintetis; tidak mengubah OTP/session/role atau mengirim SMTP.

Alur yang berhasil: consent fixture→satu JPEG canonical terenkripsi→PENDING_APPROVAL→review Admin sah→HTTP202/APPROVED_PROCESSING, belum template-ready→Go decrypt artefak yang hash-nya sama dengan reviewed job→HMAC IPC request/digest/manifest tervalidasi→YuNet/SFace nyata→Go validasi128float finite/unit→AES-256-GCM template→aktivasi transaksional/audit→ACTIVE/template_ready=true.

Authorized test-only template decrypt menghasilkan512byte; ciphertext528byte. Manifest/ownership disimpan sesuai record; purpose key terpisah, nonce foto/template berbeda. Wrong person/model/version/purpose/key dan perubahan ciphertext/nonce/AAD ditolak. Pending-photo canonical hash/record/user/consent binding ikut diperiksa. Assertion ini membuktikan **binding template v1**, bukan binding immutable template UUID; keterbatasan H tetap berlaku. Tidak ada embedding pada JavaScript/API JSON atau material terproteksi di application logs yang diperiksa.

## G. Withdrawal dan deletion

Setelah real-model ACTIVE, mahasiswa fixture withdraw melalui HTTPS. Backend menghasilkan REVOKED, template_ready=false, deletion_state=COMPLETE. Setelah reconcile: template rows0, photo rows0, active enrollment rows0, incomplete deletion jobs0, ciphertext files0. Audit aktor/event/order dan metadata allowlist diperiksa.

Ini logical revoke/deletion dan penghapusan file pada disposable store; **bukan jaminan forensic erasure SQLite/WAL/media**. Database/folder test dibersihkan oleh harness. Tidak ada database Product Owner, persistensi template manusia, foto live atau ADB.

## H. AAD review — celah terkonfirmasi, kontrak tidak diubah

**P2: authenticated-record substitution pada template envelope v1.** `backend/internal/enrollmentsecure/crypto.go` mengikat hanya person_id/model_id/model_version/key_version. `Binding.RecordID` diabaikan untuk template; processor melakukan Seal sebelum membuat template UUID. Submission/consent/fingerprint lengkap juga bukan bagian AAD template.

Diagnostik temporary `TestR2DiagnosticTemplateAADSameSubjectRecordSubstitution` dijalankan pada dua snapshot SQLite in-memory terisolasi. Masing-masing menyimpan satu template untuk person/model/version sama, tetapi template/submission ID dan synthetic unit vector berbeda. Table DDL dan immutable UPDATE trigger berasal dari migration sintetis existing. Purpose key dipisahkan; plaintext hanya di memori.

Langkah reproduksi yang dijalankan:
1. Buat dua envelope v1 dengan `Seal`, binding subject/model/version/key sama dan `RecordID` berbeda; simpan ciphertext pada masing-masing snapshot.
2. Buktikan roundtrip sah dan bahwa UPDATE envelope biasa ditolak immutable trigger.
3. Untuk model ancaman **penulis DB offline**, pertukarkan seluruh enam field envelope: envelope_version,algorithm,key_version,aad,nonce,ciphertext, dengan DELETE/reinsert. Identitas template/person/activation/consent/submission/model target tetap.
4. Baca melalui lookup target metadata lalu `Open` memakai binding target. **2/2 substitutions diterima dan2/2 menghasilkan payload record lain**.
5. Empat kontrol penolakan: subject lain, model lain, model-version lain, key lain; semuanya ditolak. Tidak ada plaintext tersimpan.

Batas penting: UNIQUE(person_id) melarang dua template legal bersamaan. Snapshot mewakili dua versi/histori terisolasi, bukan dua record current legal. Diagnostik mematikan FK dan tidak memasang lifecycle INSERT guard/parents; DELETE/reinsert memodelkan privilege offline yang bisa melewati guard. **Tidak membuktikan authorized mutation path, remote endpoint exploit, AES-GCM forgery, plaintext disclosure, atau kemampuan attacker membaca key.** Sumber envelope historis dan key version yang masih dapat dibuka adalah prasyarat serangan. `Open` adalah crypto helper terotorisasi, bukan endpoint matching baru.

Hasil aman disimpan pada `evidence-local/enr04-r2/aad-diagnostic.json`; temporary diagnostic dihapus setelah eksekusi/review agar full suite tidak mengunci perilaku rentan sebagai expected success.

**Usulan kontrak, belum implementasi:** AAD/envelope v2 dengan domain/purpose/version separation dan length-prefix canonical; wajib template_id immutable, person_id, model_id/version, key_version. Generate template ID sebelum Seal dan persist ID yang sama. **Submission identity direkomendasikan** karena claim provenance terkait submission dan nilainya tersedia saat sealing; putuskan apakah consent_id serta checksum detector/model/pipeline juga bagian authenticated claim. Decryptor menurunkan expected fields dari authorized target record, bukan mempercayai envelope-supplied identity. Jangan reinterpretasi v1 sebagai v2 atau fallback diam-diam. Tentukan penolakan v1/re-enrollment disposable terpisah, tanpa migrasi production. Tidak meredesain KeyProvider atau kebijakan D-030.

## I. Security/regression tests dan berkas

Perintah yang dijalankan dari root kecuali ditandai lain:

```powershell
services/face-engine/.venv/Scripts/python.exe -m pytest services/face-engine/tests/test_native_scale.py
services/face-engine/.venv/Scripts/python.exe -m pytest services/face-engine/tests
services/face-engine/.venv/Scripts/python.exe services/face-engine/scripts/measure_existing_fixture.py --fixtures research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified --output research/fl-enr02a-host/evidence-local/enr04-r2/positive-measurement.json
# backend/
go test -count=1 ./...
go vet ./...
# apps/mobile/
npm run typecheck
npm run lint
npm test
```

Positive opt-in dari `backend/`, memakai environment berikut dan tidak menaruh secret pada CLI:

```powershell
$env:FACELINK_CV_SYNTHETIC_JPEG='<repo>/research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified/procedural-mannequin-heightfield-v1-01.jpg'
$env:FACELINK_CV_SYNTHETIC_SHA256='cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe'
$env:FACELINK_CV_PYTHON='<repo>/services/face-engine/.venv/Scripts/python.exe'
$env:FACELINK_CV_ENGINE_DIR='<repo>/services/face-engine'
$env:FACELINK_CV_MODEL_DIR='<repo>/services/face-engine/models'
go test ./internal/httpapi -run '^TestEnrollmentRealModelSyntheticLifecycle$' -count=1 -v
```

Actual run menggabungkan positive test dan temporary AAD diagnostic pada dua paket. Positive PASS dengan18verified HTTPS requests; diagnostic PASS berarti observasinya berjalan, **bukan celah keamanan sudah diperbaiki**.

HTTPS/HTTP client E2E existing dijalankan dari `apps/mobile/` dengan `FACELINK_ADMIN_ENR_E2E=1`, `FACELINK_ENR_E2E=1`, `NODE_EXTRA_CA_CERTS` menunjuk development CA, lalu `npx vitest run tests/adminEnrollmentE2E.test.ts tests/enrollmentE2E.test.ts`. Environment shell-only; `.env` tidak diubah.

| Pemeriksaan | Hasil aktual |
|---|---|
| Regresi native scale |4FAIL sebelum,4PASS setelah|
| Python lengkap |217PASS,1SKIP|
| Go lengkap + vet |18paket ok,4tanpa tes; vet exit0|
| Real-model HTTPS lifecycle |PASS; actual18requests; ACTIVE→withdraw→REVOKED|
| AAD diagnostic |Terima substitusi2/2; ordinary UPDATE dan4kontrol ditolak|
| Main mobile typecheck/lint |Lulus|
| Main mobile tests |219PASS,6SKIP|
| E2E existing Admin/mahasiswa |6/6PASS|
| Independent preprocessing/model/evidence review |Tidak ada defect actionable tambahan|
| Independent security review |Mengonfirmasi satu P2 AAD record-binding; NO-GO tetap|

Fault/concurrency/IPC/crypto/consent regresi existing pada suite mencakup timeout, crash, malformed/oversized/unauthorized IPC, model missing/corrupt, invalid embedding/manifest, failed encryption retained-safe, revoked consent/reviewer, expired submission/lease, concurrent/duplicate worker single activation, idempotency, restart/reconciliation dan audit rollback. Kasus dependency fault memakai processor deterministik dan tidak diklaim sebagai real-model extraction. Perubahan R2 tidak menyentuh auth/session/role/mobile runtime.

Review read-only memeriksa source; tidak menjalankan ulang checks. Runtime hasil di tabel diperoleh parent. Tidak ada Android build/perangkat/kamera: surface berubah adalah preprocessing Python, manifest Go dan harness TLS, bukan native mobile.

Berkas runtime/test/script berubah atau baru:
- `services/face-engine/face_engine/detector.py`: hilangkan upscale input kecil.
- `services/face-engine/face_engine/pipeline.py`, `backend/internal/enrollmentcv/processor.go`: fingerprint preprocessing baru.
- `services/face-engine/tests/test_native_scale.py`: regresi batas/geometri dan real positive.
- `backend/internal/httpapi/enrollment_realmodel_integration_test.go`: actual TLS, status/response/privacy assertions.
- `services/face-engine/scripts/diagnose_yunet_input.py`: existing-input comparison, no candidate generation.
- `services/face-engine/scripts/measure_existing_fixture.py`: metadata-only native/alignment/extraction/IPC/resource measurement.
- Dokumentasi: laporan ini, README, PROJECT_STATE, docs/BACKLOG, API_CONTRACT, SECURITY_PRIVACY, TEST_PLAN dan face-engine README.

Tidak ada perubahan schema/migration, model weights/threshold, encryption contract, KeyProvider, production server integration, dependency, OTP/session/role, SMTP/DNS, mobile UI atau APK/video.

## J. Remaining blockers

1. **AADv1 authenticated-record binding P2 terkonfirmasi dan belum diremediasi**; perubahan ciphertext tanpa keputusan kontrak dilarang dalam R2.
2. Positive fixture hanya membuktikan jalur teknis sintetis, bukan identitas manusia, FAR/FRR, PAD/liveness, kacamata atau kalibrasi threshold absensi.
3. D-030 recovery timer/retensi metadata/backup belum final; tidak diubah. Preview native-cache tidak menjamin zeroization; mirror/crop/background encoding dan aksesibilitas lama tidak diuji ulang; F3 dialog refresh tetap di luar scope.
4. Tidak ada OS sandbox/hard memory cap. Resource adalah observasi run ini, bukan batas yang ditegakkan. Logical cleanup bukan secure media erase.
5. Tidak ada izin manusia, enrollment production, face attendance, database Product Owner, kamera/ADB atau FL-ENR05 otomatis.

## K. PASS/PARTIAL/NO-GO recommendation

**Positive-path acceptance: PASS.** Fixture prosedural sah existing→YuNet asli→landmark/alignment→SFace128float32finite→authenticated Go IPC→encrypted template ACTIVE→withdraw→REVOKED/cleanup terbukti tanpa threshold manipulation, synthetic fallback atau direct SQL activation.

**Keseluruhan: NO-GO menuju FL-ENR05**, mengikuti gerbang user bahwa temuan keamanan/integritas template tidak boleh disembunyikan oleh functional PASS. Perlu keputusan kontrak AAD versi baru dan otorisasi task remediasi terpisah. Ini bukan kegagalan model yang ditutupi atau klaim bahwa endpoint telah dieksploitasi.

**STOP setelah laporan R2. Tidak memulai R3/R4/FL-ENR05, migrasi, manusia atau production secara otomatis.**

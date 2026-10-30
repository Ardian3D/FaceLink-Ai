# FL-ENR04-R1 — Real-model positive screening dan encrypted activation

**Status: PARTIAL — REAL_MODEL_POSITIVE_NOT_PROVEN.** Enam kandidat prosedural lokal semuanya ditolak oleh YuNet asli pada threshold existing0,75. Pencarian fixture dihentikan pada batas enam kandidat unik. Tidak ada positive extraction, template real-model ACTIVE, atau withdrawal setelah real-model ACTIVE yang diklaim PASS. Negative-path FL-ENR04 tetap valid.

## A. Git baseline

Sebelum source changes, inventaris penuh tracked/untracked dibaca: **60 tracked modified,186 untracked**. Branch `feat/fl-otp01-email-auth`; HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`. Fetch non-destructive `origin` berhasil; `origin/main` tetap `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`, merge-base `aa36f2d243f8761879baf38a5b2735afab8f1b52`, divergence HEAD/main21/1. PR#19 tidak di-merge ulang. Source/evidence FL-CORE, FL-CAM, FL-ENR, FL-STAB, APK/video dan pekerjaan uncommitted dipertahankan. Tidak ada reset/stash/clean/switch/commit/push/merge atau migrasi database PO. Laporan FL-ENR04, kontrak, dan harness existing dibaca; tidak mengulang desain engine.

Berkas tugas:
- Baru: `services/face-engine/scripts/screen_synthetic_faces.py`.
- Baru: `backend/internal/httpapi/enrollment_realmodel_integration_test.go`.
- Ditambah: `backend/internal/enrollment/processor_contract_test.go` (encryption dependency failure dan duplicate worker).
- Ditambah: `services/face-engine/tests/test_pipeline.py` (matching invalid query/template dan incompatible model fingerprint).
- Dokumentasi: laporan ini, README, PROJECT_STATE, BACKLOG, TEST_PLAN dan README engine.
- Fixture/evidence berada di `research/fl-enr02a-host/evidence-local/enr04-r1/`, diabaikan Git; `git check-ignore` telah membuktikan rule berlaku. Tidak ada perubahan runtime model, threshold, quality, IPC, auth/session/role atau migration.

## B. Fixture provenance dan licensing

Fixture **`procedural-mannequin-heightfield-v1`**:512×512, enam variasi parameter tetap. Semua pixel dibuat dari persamaan height-field2,5D, normal/shading, seed NumPy4400–4405 dan primitive drawing OpenCV. Bentuk mata/hidung/mulut/alis/telinga adalah koordinat fiktif, bukan landmark yang disuntikkan ke model. Tidak ada foto/scan/mesh manusia, selfie, mahasiswa, selebritas, dataset identitas, model generator terlatih, image texture, file biometrik lama, download atau layanan eksternal.

Generator/output baru didedikasikan **CC0-1.0**, hanya untuk kode generator fixture dan hasilnya; bukan perubahan lisensi repository, dependency atau weights. [Teks CC0](https://creativecommons.org/publicdomain/zero/1.0/legalcode). Tidak ada asset pihak ketiga untuk fixture ini. Model YuNet/SFace tetap memiliki lisensinya sendiri sebagaimana laporan FL-ENR04.

SHA256 generator final: `bd659f9f51d9a826954ffd56d6391439a4d82cd6676a26fb5e4a48e4cdcb226f`.

| Kandidat | Byte JPEG | SHA256 |
|---|---:|---|
|01|17817|cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe|
|02|17628|54f67652c6a404d818eb152eaa5c3c5a3f91ef66de7c681bb2a643a0be3d97bc|
|03|17908|78fac2b5ae5d2e1e0627196ac51ce45b2c4d96e05876d1d5b4970972a383f7e0|
|04|18347|7698365378a0db8c374d77e87d5d70d9f6768e392fb99cb6900d3bd602e5e2b8|
|05|17875|2c235d4e1044e21eff104e9c44ad7c4317b6220ef28a136a22bea163cdf7b081|
|06|18430|2fc49ce7b283d03e11afcb3709f1aec484df115703250516e0a6d93894c6d015|

Keenam hash direproduksi6/6 pada Python3.12.14/OpenCV4.12.0/NumPy2.2.6. Byte identity lintas versi encoder/NumPy/OS tidak diklaim. Dua run memakai enam kandidat **yang sama**: run awal dan replay final setelah memperkeras validasi evidence IPC; hash JPEG tetap sama. Run awal dipertahankan di `fixtures-v1/`; run final di `fixtures-v1-verified/`. Tidak menambah kandidat setelah NO_FACE dan tidak menimpa evidence lama.

Reproduksi dari root, tanpa kamera dan dengan output directory baru/kosong:

```powershell
services/face-engine/.venv/Scripts/python.exe -O services/face-engine/scripts/screen_synthetic_faces.py --output research/fl-enr02a-host/evidence-local/enr04-r1/reproduction
```

Nama file: `procedural-mannequin-heightfield-v1-01.jpg` sampai `-06.jpg`; metadata aman `screening.json`. Output mencakup provenance/license, versi generator/runtime, pin model, hash fixture, counts/status/timing; tidak mencakup embedding, JPEG inline, key, token atau IPC payload. JPEG yang disimpan adalah asset prosedural yang secara eksplisit diizinkan, bukan hasil capture manusia atau persistent embedding.

## C. YuNet detection evidence

Dua weights lokal asli diverifikasi oleh Pipeline existing sebelum native load:
- YuNet2023mar: `8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4`.
- SFace2021dec: `0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79`.

**Semua6/6: face_count0, confidence[], NO_FACE**, threshold0,75; direct pipeline dan authenticated real Python IPC sama-sama menolak. JPEG diterima decode sesuai input policy, tetapi detektor tidak menghasilkan row/landmark. Quality evaluator menghasilkan NO_FACE; **tidak ada bukti quality-pass**, confidence kandidat di bawah threshold, atau landmark valid karena row tidak tersedia. Tidak menjalankan ulang dengan threshold lebih rendah untuk mengukur score tersembunyi.

Penyebab terukur berhentinya alur: **tidak ada satu deteksi pun pada konfigurasi existing**, sehingga alignment/extraction tidak berhak berjalan. Dugaan domain gap render stylized2,5D terhadap model bukan penyebab yang sudah dibuktikan. Fixture ini bukan render fotorealistik dan bukan bukti YuNet gagal pada seluruh jenis sintetis.

## D. SFace alignment/extraction evidence

**NOT_RUN untuk positive fixture R1**. Model SFace ter-load, tetapi tidak dipanggil dengan bounding box/landmark buatan. `alignment_ms` dan `sface_ms` bernilai null, bukan0 atau PASS. Tes komponen existing dengan landmark deterministik tetap dibedakan dari actual YuNet-positive pipeline. Tidak menyalin embedding buatan ke hasil model, menonaktifkan gate, atau mengubah NO_FACE menjadi success.

## E. Real Go/Python IPC

Harness opt-in baru: `TestEnrollmentRealModelSyntheticLifecycle`. Konfigurasi harus memuat seluruh5variabel di bawah. Semuanya kosong → explicit SKIP; konfigurasi parsial/invalid → FAIL. Path absolut, pin SHA fixture dan dua model diperiksa. Source/model path wajib menunjuk engine repo ini. Operator harus menjamin asal fixture sepenuhnya prosedural; hash mengikat bytes, bukan membuktikan apakah foto itu manusia.

```powershell
$env:FACELINK_CV_SYNTHETIC_JPEG = (Resolve-Path research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1/procedural-mannequin-heightfield-v1-01.jpg).Path
$env:FACELINK_CV_SYNTHETIC_SHA256 = 'cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe'
$env:FACELINK_CV_PYTHON = (Resolve-Path services/face-engine/.venv/Scripts/python.exe).Path
$env:FACELINK_CV_ENGINE_DIR = (Resolve-Path services/face-engine).Path
$env:FACELINK_CV_MODEL_DIR = (Resolve-Path services/face-engine/models).Path
Push-Location backend
try { go test ./internal/httpapi -run '^TestEnrollmentRealModelSyntheticLifecycle$' -count=1 -v }
finally { Pop-Location }
```

**Perintah dengan kandidat01 di atas benar-benar dijalankan dan FAIL**, bukan PASS: `pinned procedural fixture did not activate with real models: reason_code=NO_FACE`. Memang tidak ada fixture positif untuk memenuhi assertion. Jangan memakai konfigurasi ini pada full-suite lalu mengklaim all-pass. Untuk default suite, hapus5variabel opt-in dari shell setelah eksperimen.

Run mencakup disposable SQLiteWAL, synthetic Admin/student/person/class/verified binding, existing local-only auth/JWKS fixture, normal HTTP consent grant/upload/review/approve202, encrypted canonical JPEG, actual enrollmentcv Processor/Python/models dan reconciliation. Tidak ada call Google/SMTP/external atau perubahan auth. Tidak memakai direct SQL untuk membuat ACTIVE, clock advance, atau manipulasi reviewer/consent/expiry agar sukses.

Metadata aktual: fixtureSHA di atas; canonicalSHA `00eaa7792b75bd3f7f2ad10bfcbbb5fd3afe64fec4b626761a0e42b239cace76`;512×512; processing467ms,total785ms; jobFINISHED, reasonNO_FACE; template_rows0,active_rows0,photo_rows0,ciphertext_files0 setelah reconcile. Evidence aman: `go-realmodel-negative.json`.

## F. Encrypted template activation

**NOT_PROVEN dari model nyata.** Harness yang dieksekusi telah melewati pending-photo AEAD, canonical hash binding, identity record/key-version AAD dan rejection wrong record/user/consent/purpose key sebelum approval. JPEG pending ciphertext dibuka hanya oleh test terotorisasi di memori, tidak ditulis sebagai plaintext tambahan.

Karena model menolak, tidak ada template untuk membuktikan real128D→ciphertext→ACTIVE. Assertion template encryption/nonce/purpose separation/authorized Open/finiteness/model binding dan log filtering setelah positive branch belum dieksekusi. Keberadaan source assertion bukan hasil PASS.

Kontrak unchanged: AAD pending-photo mengikat submission/person/user/consent/key-version. AAD template D-022 mengikat **person_id,model_id,model_version,key_version**; bukan `template_id` atau `submission_id`/`consent_id`. Jangan menyatakan binding lebih kuat dari yang ada. Jika maksud requirement record adalah template/submission UUID dalam AAD, itu delta kontrak yang memerlukan keputusan terpisah; R1 tidak mengubah format/kebijakan secara diam-diam.

## G. Consent withdrawal/deletion

Withdrawal setelah **real-model ACTIVE: NOT_RUN**, karena real-model ACTIVE tidak terjadi. Tidak membuat ACTIVE palsu hanya untuk menjalankan withdrawal. Domain regression existing memakai processor deterministik membuktikan ACTIVE→withdraw→REVOKED/template_ready=false/no template/no photo/reconcile complete; HTTPS E2E existing juga lulus. Itu bukti domain, bukan positive YuNet/SFace evidence.

Pada run nyata yang ditolak, reconciliation menunjukkan tidak ada template, active enrollment, photo row atau ciphertext file tersisa. Test DB/store berada di `t.TempDir` terisolasi dan dibersihkan oleh harness; tidak menyentuh DB/key Product Owner. Source fixture prosedural di evidence-local sengaja dipertahankan untuk provenance, bukan bagian disposable enrollment store.

## H. Failure dan concurrency

Seluruh tes berikut dijalankan dalam regresi Go/Python dan lulus. Domain faults memakai dependency deterministik; tidak disamakan dengan real extraction.

| Fault | Bukti tes |
|---|---|
|Withdrawal saat inference|TestWithdrawalDuringProcessorFencesLateResult|
|IPC timeout/cancel|TestDeadlineKillsReapsChild; TestCancellationKillsReapsAndReleasesSlot|
|Python/process crash|TestProtocolAndProcessFailuresClose/crash (child helper fault)|
|Model unavailable|TestStructuredProcessorFailuresRemainClosedAndAudited/MODEL_UNAVAILABLE; Python missing/corrupt models|
|Invalid embedding|TestExtractionErrorsNeverActivateAndConservativelyPurge; IPC dimension/NaN/Inf/norm guards|
|AAD mismatch|TestEnvelopeAuthenticatesEveryTrustedField; TestTemplateAADPreservesD022Tuple|
|Failed template encryption|**Baru:** TestTemplateEncryptionFailureRetainsRecoverablePhotoWithoutActivation|
|Stale lease|TestInferenceLeaseRevocationAndImmutableBindings/expired-lease dan lease-replaced|
|Reviewer invalidated|TestReviewerDemotionRestorationPermanentlyFencesApproval; reviewer-revoked-restored|
|Backend restart|TestAbandonedRunningJobFailsOnRestart; TestEncryptedFileCrashRecoveryAndOrphanCleanup|
|Duplicate processing|**Baru:** TestConcurrentDuplicateProcessingActivatesExactlyOnce|
|Expired submission|TestExpiryIs72HoursAndNeverExtendedByReplay; TestPhotoExpiryDuringProcessorFencesLateActivation|

Tes gagal enkripsi baru memutus key setelah photo decrypt, membuktikan FAILED_PROCESSING/BIOMETRIC_UNAVAILABLE, no ACTIVE/template, failure audit, ciphertext/nonce/AAD/expiry foto retained **tidak berubah**, lalu explicit retry sesudah key pulih dapat berhasil. Tidak ada irreversible deletion otomatis akibat key loss; D-030 tidak berubah.

Tes duplicate baru menahan inference pertama, memeriksa PROCESSOR_BUSY dari Service yang sama maupun instance kedua dengan DB sama, lalu menguji worker polls berulang: satu inference/attempt/template/enrollment/start audit/activation audit, bukan duplikasi.

## I. Performance/resource measurement

Run final, enam kandidat identik, bukan SLA/benchmark accuracy:

| Tahap | Hasil teramati |
|---|---|
|Model verification+initialization|107,084ms|
|JPEG validation+decode|0,772–1,540ms|
|YuNet inference|86,332–118,825ms|
|Pipeline negative direct|79,610–111,109ms pada panggilan terpisah|
|Alignment|NOT_RUN — tidak ada deteksi sah|
|SFace feature inference|NOT_RUN — tidak ada deteksi sah|
|Authenticated Python IPC round trip|482,857–628,576ms termasuk child startup/model load/exit|
|Actual Go ProcessNext|467ms pada kandidat01 canonical|
|Total HTTP/domain harness|785ms|
|Peak working set teramati|172,324MiB pada proses **driver screening + renderer + model**, bukan peak khusus child/total aggregate|
|Process lifecycle|Enam subprocess normal exit0, synchronous collection/reaped; helper deadline/cancel tests lulus|

Pengukuran tidak memakai launcher venv sebagai CPU/model footprint. Tidak ada hard memory cap, OS sandbox, klaim zeroization atau throughput/p95. Timeout aktual normal tidak terjadi; timeout/cancel/crash diuji dengan controlled child helper, bukan mengklaim model native hang lalu pulih. Tidak ada leftover child yang diamati pada run normal; descendant-native race universal tidak diklaim tertutup.

## J. Matching contract

Tambahan5parameterized cases memeriksa query dan template berdimensi salah/NaN/Inf serta model version/checksum tidak cocok. Existing self-cosine/pipeline fingerprint/zero-vector guard tetap diuji. Python suite **213passed,1skipped**. Vektor pada matching contract tests deterministik dan bukan hasil fixture R1, karena extraction belum terjadi. Tidak ada threshold absensi production, FAR/FRR, spoof-resistance atau klaim pengenalan identitas.

## K. Regression

| Pemeriksaan | Hasil |
|---|---|
|`python.exe -m pytest services/face-engine/tests`|213passed,1skipped (existing positive detection check pada gambar acak)|
|`go test -count=1 ./... && go vet ./...` dari backend, opt-in vars tidak diset|18paket ok,4tanpa tes; vet exit0; positive integration baru explicit opt-in skip|
|Konfigurasi positif dengan kandidat01/hash|**FAIL exit1, NO_FACE**; no false ACTIVE; bukan suite PASS|
|Konfigurasi opt-in parsial|**FAIL exit1** sebelum model/DB: required config missing|
|`FACELINK_ADMIN_ENR_E2E=1 FACELINK_ENR_E2E=1 NODE_EXTRA_CA_CERTS=.../dev-ca.pem npx vitest run tests/adminEnrollmentE2E.test.ts tests/enrollmentE2E.test.ts`|2files/6tests passed: HTTPS Admin5 + HTTP mahasiswa1|
|Replay generator akhir `python -O`|Semua6 tetap NO_FACE; hash sama; secret/HMAC checks tetap aktif|
|Tampered HMAC pada screening `python -O`|Ditolak dengan sanitized RuntimeError|

Tidak ada shared runtime/mobile/API contract yang berubah; full mobile typecheck/lint/unit tidak diulang karena hanya harness/tests/dokumentasi berubah. Mobile client existing dipakai pada enam E2E di atas. Tidak menjalankan Android/ADB/build/perangkat atau kamera. Hasil regresi bukan substitusi positive criterion yang gagal.

## L. Independent security review

Dua review read-only: provenance/evidence/IPC screening dan crypto/domain harness. Tidak ditemukan defect actionable pada trust boundary yang ditinjau. Reviewer mengonfirmasi source tidak memuat human assets, model threshold override, fake-model fallback, direct SQL activation atau credential/embedding evidence.

Catatan robustness ditemukan: validasi evidence screening awal memakai assert yang dapat hilang pada `python -O`. Diperkeras menjadi conditional RuntimeError untuk process/bounds/framing/HMAC/version/request/digest/model binding. Replay final di mode optimasi dan tampered-HMAC smoke dijalankan; reviewer melakukan source-read closure. Ini tidak mengubah runtime IPC FL-ENR04. Source review bukan sertifikasi universal privacy/race; provenance JPEG tetap jaminan operator + generator reproducibility, bukan sesuatu yang bisa dibuktikan hash saja.

## M. Remaining limitations

- Seluruh kandidat stylized2,5D gagal positive detection. Tidak ada fixture sintetis photorealistic/3D penuh yang dibuktikan; eksperimen berhenti sesuai batas.
- Positive alignment, SFace128D, Go encrypted ACTIVE dan withdrawal sesudah real-model ACTIVE tetap **NOT_PROVEN/NOT_RUN**.
- Bagian positive-only harness telah dikompilasi dan ditinjau tetapi belum runtime-exercised; tidak dihitung sebagai PASS.
- Binding AAD template persis D-022; penambahan template/submission UUID tidak diotorisasi pada R1.
- Threshold recognition belum dikalibrasi; PAD/liveness belum ada. Satu fixture kelak pun tidak membuktikan accuracy/FAR/FRR.
- D-030 recovery timer, retention insiden/backup, preview cache manusia, mirror/crop/background encoding dan F3 dialog tetap residual existing; tidak diubah.
- Native model/interpreter/source paths dipercaya; tidak ada hard OS-memory/network/filesystem sandbox atau zeroization guarantee.

## N. GO/PARTIAL/NO-GO for FL-ENR05

**PARTIAL. REAL_MODEL_POSITIVE_NOT_PROVEN.** Tujuan positive acceptance belum tercapai, bukan diturunkan definisinya. Negative-path dan perlindungan no-false-ACTIVE terkonfirmasi; fault/concurrency/matching regressions lulus. Prasyarat berikutnya adalah fixture prosedural sah yang benar-benar diterima oleh konfigurasi model existing, dengan rencana eksperimen baru yang dibatasi dan disetujui pemilik bila pekerjaan dilanjutkan.

**Belum GO untuk implementasi FL-ENR05** atas hasil R1 ini. **NO-GO manusia/production/face attendance/migrasi PO.** Tidak ada penggunaan wajah manusia, live capture/upload, ESP-CAM, perubahan OTP/session/role/SMTP/DNS, penghapusan evidence lama atau operasi Git destruktif. **STOP setelah laporan R1; tidak memulai FL-ENR05 otomatis.**

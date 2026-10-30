# FL-SEC01 — AES-256-GCM Template AAD v2: Record Binding Remediation

**Status: PASS pada lingkungan sintetis/disposable.** Swap lintas-record yang sebelumnya mengembalikan plaintext kini ditolak. Real YuNet/SFace→IPC→template v2→ACTIVE→withdraw→REVOKED tetap berfungsi. Tidak ada izin human enrollment, production, migrasi Product Owner atau eksekusi FL-ENR05.

## A. Git baseline dan preservation

Baseline dibaca sebelum implementasi: branch `feat/fl-otp01-email-auth`; HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`; local tracking `origin/main` `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`; merge-base `aa36f2d243f8761879baf38a5b2735afab8f1b52`; divergence HEAD/main21/1. PR#19 telah merged menurut baseline sebelumnya dan konteks pemilik; tidak di-merge ulang. Tidak mengklaim local tracking ref merupakan fetch remote baru pada FL-SEC01.

Inventaris lengkap `git status --short --untracked-files=all` dibaca, termasuk seluruh tracked modifications dan untracked FL-CORE/FL-CAM/FL-ENR/FL-STAB. Worktree tidak bersih; perubahan lama dipertahankan. Tidak ada reset/stash/clean/rebase/pull/switch/commit/push/merge. Evidence lama, APK, video dan backup tidak dihapus. Baseline tersedia dalam transcript eksekusi; tidak menjalankan Git berulang sebagai pengganti verifikasi runtime.

Dokumentasi canonical, kontrak API/otorisasi/data, laporan R2, sumber crypto/domain/migration dan dependensi dibaca. LSP tidak dikonfigurasi; caller ditelusuri melalui pencarian terarah. Go+Chi, SQLite WAL, KeyProvider dan seluruh dependensi existing dipakai tanpa layanan berbayar atau perubahan role/auth.

## B. Kontrak AAD v2 final

Template baru memakai envelope version **2**, algorithm **AES-256-GCM**, nonce acak kriptografis12byte, tag16byte digabung ciphertext. Key purpose `template` tetap terpisah dari `pending-photo`; KeyProvider existing tidak didesain ulang. `key_version` tetap TEXT dan diambil lewat provider existing.

AAD adalah lowercase hex dari tuple fixed-order UTF-8, setiap field diawali panjang **byte uint32 big-endian**:

```text
("FaceLink", "2", "template",
 template_id, submission_id, person_id, consent_id,
 model_id, model_version, model_fingerprint, key_version)
```

Seluruh identitas record wajib canonical nonzero UUID. Model ID/version wajib nonempty, bounded4096byte dan UTF-8 valid di codec; domain manifest tetap lebih ketat sesuai validator existing. Fingerprint wajib lowercase hex64. Key version nonempty≤128byte. Encoding fixed tuple/length-prefix menghindari ambiguitas delimiter maupun concatenation; tidak melakukan normalisasi identitas secara diam-diam.

Fingerprint model/pipeline adalah lowercase SHA256 atas tuple length-prefixed yang sama bentuk encoding-nya:

```text
("FaceLink:model-pipeline:1", synthetic_only_as_"true"_or_"false",
 model_id, model_version, lowercase(model_checksum),
 detector_id, detector_version, lowercase(detector_checksum), pipeline_revision)
```

Fingerprint mencakup seluruh identitas model/detector/preprocessing yang stabil, bukan waktu, status consent, mutable revision, nonce atau hasil embedding. Versi AAD/envelope bukan versi key dan bukan `pipeline_revision`; ketiganya berbeda domain.

## C. Immutable identity binding

- Template UUID dibuat **sebelum Seal**; UUID yang sama dipakai saat INSERT.
- Binding memuat submission UUID, person, grant consent historis dan manifest fingerprint lengkap.
- Readiness merekonstruksi binding dari join template→submission→person→consent grant→activation. Join memeriksa subject/person/grant, account link/epoch, active submission/activation, approved review, purpose/method/status grant, tipe/dimensi/synthetic marker serta konsistensi model activation/template.
- Metadata manifest yang dibaca divalidasi sebelum fingerprint/decrypt. Stored AAD hanya dibandingkan dengan expected AAD dan diverifikasi oleh GCM, bukan dipercaya sebagai otoritas identitas.
- `consent_records.id` existing sudah merupakan immutable grant identity: trigger melindungi ID/person/subject dan bukti grant, termasuk granted_at/notice/policy. Withdrawal hanya memperbarui status/withdrawal metadata, tidak mengganti grant ID. Tidak perlu menambah consent-grant table/kolom baru.
- Consent/reviewer/lease/attempt/revision/epoch/hash dan availability tetap diperiksa sebelum irreversible activation. Tidak ada endpoint decrypt/template/matching baru atau SQL shortcut aktivasi pada real-model path.

## D. Source changes dan schema disposable

Runtime:
- `backend/internal/enrollmentsecure/crypto.go`: Binding diperluas, template AAD/envelopev2, version refusal sebelum key lookup, bounded canonical encoding. Validasi awal Seal tidak lagi membuang hasil encoding/hex sementara; encoding dilakukan sekali setelah key version diketahui.
- `backend/internal/enrollment/model.go`: `Manifest.Fingerprint()`.
- `backend/internal/enrollment/processor.go`: generate/bind/store template UUID yang sama.
- `backend/internal/enrollment/views.go`: reconstruct complete binding dari relasi SQL tervalidasi.
- `backend/internal/enrollmentfixture/migrations.go`: compose isolated00007+00008 hanya untuk disposable harness.
- **Baru** `backend/synthetic-migrations/00008_template_aad_v2.sql`.

Migration00008 forward-only membuat tabel pengganti, menyalin semua kolom/baris/envelope existing tanpa decrypt/konversi, lalu mengganti tabel serta membuat ulang immutability/binding guard. FK, uniqueness person/submission/activation dan nonce/key-version dipertahankan. Schema dapat menyimpan legacyv1 yang dikarantina, tetapi trigger INSERT menolak write template baru selainv2 dan memeriksa relasi domain.

**00007/checksum tidak diubah; default `backend/migrations` tidak ditambah/diubah oleh FL-SEC01.** Migration dijalankan hanya pada SQLite test/disposable. `cmd/server` tetap tidak menginjeksi enrollment; tidak membuka atau memigrasi database Product Owner.

Pengujian berubah/baru:
- `enrollmentsecure/crypto_test.go`, **baru** `crypto_security_test.go`.
- `enrollment/service_test.go`, `processor_contract_test.go`, **baru** `sec01_test.go`.
- `migrations/enr01b_test.go`, **baru** `migrations/sec01_test.go`.
- `httpapi/enrollment_realmodel_integration_test.go`: complete v2 binding, verified TLS, grant identity dan cleanup.

Dokumentasi: DECISIONS, DATA_MODEL, API_CONTRACT, SECURITY_PRIVACY, ARCHITECTURE, TEST_PLAN, BACKLOG, README root/backend, PROJECT_STATE dan laporan ini. Tidak ada perubahan mobile UI/runtime, Python/model/threshold/quality gate, OTP/session/role, SMTP/DNS, KeyProvider atau weights/dependency.

## E. Legacy v1 handling

`Seal` untuk template selalu menulisv2. `Open` template menolak legacyv1 dan unknown version dengan `ErrUnsupported` yang juga memenuhi `errors.Is(...,ErrIntegrity)`, sebelum provider/decryption; tidak ada fallback. Mengganti version tag atau stored AAD legacy tidak membuat upgrade valid. Pending-photo v1 tetap didukung sebagai **purpose/format berbeda**, bukan fallback template.

Readiness unsupported adalah false dengan `BIOMETRIC_UNAVAILABLE` existing; row/submission dapat tetap tercatat ACTIVE secara historis, tetapi template tidak tersedia/usable. Ini quarantine logis tanpa state/API baru, tanpa klaim versi unsupported telah dimigrasi. Reconcile/status/service recreation tidak otomatis menghapus/mengonversi row karena unsupported/corruption/key loss.

Upgrade fixture7→8 membuktikan semua envelope bytes/metadata v1 dan ledger checksum≤7 tetap; FK check bersih; new-v1 INSERT ditolak. Test domain memakai ciphertext legacy yang benar-benar authentic AES-GCM, bukan hanya v2 dengan angka versi diubah.

Tidak ada pencarian database non-disposable untuk mencari data v1. Bila kelak ditemukan: tutup penggunaan biometrik row unsupported, pertahankan data/evidence, laporkan metadata versi tanpa payload, dan minta persetujuan rencana kompatibilitas terpisah sebelum konversi/re-enrollment/migrasi/penghapusan. Tidak menjalankan strategi itu otomatis.

## F. Ciphertext-swap regression

Reproduksi sebelum perubahan runtime menjalankan `TestTemplateRejectsSamePersonDifferentRecordSwap`: **FAIL**, `cross-record template swap returned plaintext=true, error=<nil>`. Subject/model sama, hanya RecordID target berubah. Setelahv2: **PASS**, tidak mengembalikan plaintext.

Crypto tests menguji transplant envelope serta forged stored AAD yang disamakan dengan expected AAD target. Pada forged-AAD case, verifikasi GCM tetap menolak tag karena attacker tidak memiliki key. Jadi bukti tidak berhenti pada string comparison AAD.

SQLite tests mengubah identitas template/submission/grant secara independen untuk person/model/pipeline yang sama, menjaga join/FK tetap valid, dan memasang kembali seluruh enam field envelope yang disimpan. Readiness tetap false, ciphertext dipertahankan. Kasus grant sebelumnya juga diuji lewat enrollment→withdraw→grant baru dan replay envelope lama.

Manipulasi ini sengaja melewati trigger immutability tertentu pada database test untuk memodelkan offline DB writer. Itu **bukan** bukti endpoint HTTP bisa menulis identitas, bukan AES-GCM forgery dan bukan klaim seluruh offline tampering dicegah.

## G. Model/consent mismatch dan security acceptance

| # | Skenario wajib | Bukti/kontrol yang dijalankan |
|---|---|---|
|1|Normal templatev2 decrypt|Real AES-GCM roundtrip dan real-model authorized test decrypt|
|2|Swap ciphertext antar-template|Crypto rejection + SQLite complete-envelope transplant|
|3|Same person/model, template ID berbeda|FAIL-before→PASS-after; independent SQLite record-ID case|
|4|Submission reference berbeda|Crypto mismatch/forgedAAD + coherent SQLite identity substitution|
|5|Person mismatch|Crypto mismatch/forgedAAD; joined ownership guards|
|6|Consent grant mismatch|Crypto + coherent grant-ID substitution + previous-grant replay|
|7|Model/pipeline fingerprint mismatch|Complete fingerprint field tests, crypto mismatch dan SQLite provenance tampering|
|8|Key version mismatch|Dua versi sengaja resolve key bytes sama; AAD/tag tetap menolak perubahan versi|
|9|Ciphertext/tag tampered|GCM rejection, termasuk nonce/storedAAD/algorithm/length guards|
|10|Unknown AAD/envelope version|Unsupported sebelum key lookup, tanpa plaintext|
|11|Authentic legacyv1|Unsupported tanpa fallback; relabel/rewriteAAD juga ditolak|
|12|Withdrawal tidak merusak identitas AAD|Grant evidence historis sama; retained test-only envelope authentic setelah withdrawal, sementara service authorization revoked dan stored payload dihapus|
|13|Encryption failure tidak ACTIVE|Existing failed-template-encryption retained-safe, recovery guard dan audit regressions|
|14|Concurrent processing tidak mencampur record|Dua subject/vector berbeda, worker/lease busy, dekripsi record-bound memeriksa seluruh vector tujuan dan audit exactly-once|
|15|Replay/rollback metadata|Same-binding replay tetap authentic; grant lama/target berbeda ditolak; full coherent DB rollback tidak diklaim dicegah|

Encoding tests mencakup tuple yang collision bila digabung dengan delimiter maupun tanpa separator, forged storedAAD dan repeatedSeal determinism. Owned key copies diperiksa terhapus setelah Seal/Open success, authentication failure, short key dan provider yang mengembalikan key sekaligus error. Ini bukti terhadap buffer Go yang dimiliki provider/caller, bukan klaim seluruh native/process memory zeroized.

## H. Real YuNet/SFace positive E2E

Fixture R2 existing, **tidak membuat fixture baru**:
- `procedural-mannequin-heightfield-v1-01.jpg`,512×512, wholly procedural/nonhuman.
- SHA256 `cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe`.
- Go canonical SHA256 `00eaa7792b75bd3f7f2ad10bfcbbb5fd3afe64fec4b626761a0e42b239cace76`.
- YuNet2023mar SHA256 `8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4`.
- SFace2021dec SHA256 `0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79`.
- Pipeline `enr04-synthetic-native-v2`, threshold0,75 dan quality gate unchanged.

`TestEnrollmentRealModelSyntheticLifecycle` memakai concrete processor yang meluncurkan Python asli; tidak ada fake SFace fallback. Model hashes, face/landmark/quality/finite/unit vector, HMAC/request/digest/manifest dan canonical reviewed-image binding harus lolos agar state machine mencapai ACTIVE.

Actual **18request HTTPS** ke loopback disposable, CA eksplisit/hostname/chain/peer verified, tanpa TLS bypass. SQLite WAL dan keys test-only. Template envelope version2 diperiksa,128float32finite/unit, plaintext512byte hanya di memori test, ciphertext528byte AES-256-GCM. ProcessNext terukur504ms, lifecycle851ms pada satu run; bukan SLA/accuracy benchmark.

Evidence `research/fl-enr02a-host/evidence-local/sec01/realmodel-v2.json` hanya metadata/hash/counts/timing; tidak ada vektor, JPEG/base64/key/token. Application logs juga diperiksa terhadap material terproteksi. Fixture/provenance/evidence R2 lama tidak ditimpa.

## I. Withdrawal/reconciliation, audit dan failure handling

Consent fixture ditarik melalui HTTPS setelah real ACTIVE. Hasil: REVOKED, template_ready=false, deletion_state COMPLETE; template/photo/active enrollment/incomplete deletion rows0 dan ciphertext files0 setelah reconcile. Grant ID/evidence historis tetap; crypto-only retained test envelope tidak terpengaruh mutable withdrawal status, tetapi tidak lagi tersedia melalui service dan tidak disimpan lagi pada DB. Ini memisahkan authentication AAD dari authorization/current consent.

Regresi mencakup audit rollback saat preview/decision/activation/withdrawal, subject/reviewer eligibility, withdrawal saat inference, stale lease/attempt/revision/epoch/hash, expiry, model unavailable/timeout/crash/malformed output, key loss, corruption, staged-file recovery, restart/service recreation dan reconcile. Key/integrity unavailable tidak menjadi izin purge template otomatis; pemulihan dengan key asli diuji. Core/manual/auth tidak diubah.

Logical cleanup bukan forensic erase SQLite/WAL/SSD, paging atau salinan eksternal. **D-030 recovery timer dan irreversible production deletion tetap BLOCKED**; AADv2 tidak memutuskan timer, key replacement, backup policy atau penghapusan data asli.

## J. Go/Python/security regression

| Pemeriksaan | Hasil aktual |
|---|---|
| Attack regression sebelum fix |FAIL: plaintext diterima pada target RecordID lain|
| Attack/security suite setelah fix |PASS; mismatch/forgedAAD/legacy/domain/key-version/encoding cases|
| Go lengkap `go test -count=1 ./...` |18paket ok,4tanpa tes|
| `go vet ./...` |exit0|
| Python lengkap |217PASS,1SKIP|
| Real-model HTTPS E2E v2 |PASS;18verified requests; ACTIVE→REVOKED/cleanup|
| Existing client E2E |Admin HTTPS5 + mahasiswa HTTP1:6/6PASS|
| Mobile typecheck/lint |Lulus|
| Mobile tests |219PASS,6SKIP|
| Targeted encoding/key-copy/record-swap regressions tambahan |PASS|

Run Go lengkap pertama sempat gagal pada fixture baru: multi-statement SQL menerima argumen yang tidak terikat per statement sehingga subject kedua gagal FK; fixture legacy juga tidak mencapai ACTIVE. Setup diperbaiki menjadi Exec terpisah dengan parameter masing-masing dan RowsAffected untuk transisi legacy; assertion tidak dilemahkan. Setelah perubahan, full suite lulus. `go vet` pada run pertama tidak berjalan karena command chain berhenti di tests; hasil vet exit0 di atas berasal dari run sukses sesudah perbaikan.

Skip lingkungan: Python satu test skipped; Go secure package melewati unprivileged symlink test saat host tidak mengizinkan dan unsupported-publication test pada platform yang memang mendukung atomic publication. Opt-in positive test default absent-config SKIP tidak dihitung sebagai bukti; positive dikonfigurasi/dijalankan terpisah. Mobile default suite melewati enam opt-in E2E, kemudian seluruh enam dijalankan eksplisit dan lulus. Tidak ada Android build/ADB/kamera karena surface native tidak berubah.

Perintah utama dari root, kecuali folder ditandai:

```powershell
services/face-engine/.venv/Scripts/python.exe -m pytest services/face-engine/tests
# backend/
go test -count=1 ./...
go vet ./...
go test ./internal/enrollmentsecure -run 'TestTemplate|TestEnvelopeVersions|TestProviderKeyCopies' -count=1 -v
# apps/mobile/
npm run typecheck
npm run lint
npm test
```

Real-model opt-in dari `backend/` menggunakan lima environment variables fixture SHA/path, interpreter, engine dan model directory, sama dengan R2. Jalankan `go test ./internal/httpapi -run '^TestEnrollmentRealModelSyntheticLifecycle$' -count=1 -v`. Semuanya kosong tetap SKIP; konfigurasi parsial/invalid FAIL. Path dan exact SHA ada pada H serta resep `FL_ENR04_R2_TECHNICAL_REPORT.md` §I. Tidak menaruh secret pada command line.

Client E2E dari `apps/mobile/`: environment shell-only `FACELINK_ADMIN_ENR_E2E=1`, `FACELINK_ENR_E2E=1`, `NODE_EXTRA_CA_CERTS` ke development CA, lalu `npx vitest run tests/adminEnrollmentE2E.test.ts tests/enrollmentE2E.test.ts`. `.env` tidak diubah.

## K. Independent skeptical review

Security reviewer read-only menyelesaikan dua cakupan: crypto/encoding/version/legacy/tests, lalu domain/migration/caller/consent/fencing/audit/realmodel harness. **Tidak ada evidence-backed exploitable defect tersisa dalam cakupan yang ditinjau.** Review domain pengganti dilakukan setelah reviewer pertama berhenti tanpa laporan final; tidak menghitung review yang gagal sebagai PASS.

Catatan coverage crypto: boundary collision tanpa separator dan deterministicAAD ditambahkan; key-copy success/failure/Open coverage diperluas dan diuji. Saran untuk membedakan nil dari empty slice tidak merupakan vulnerability: kontrak user adalah tidak membocorkan plaintext dan seluruh jalur error source memang return nil; assertion length0 tidak diklaim membuktikan forensic memory erasure.

Reviewer menelusuri separate migration preserving-v1, guard newv1write, complete trusted joins, immutable grant, pre-Seal template ID dan caller cutover. Review bersifat static, tidak menjalankan tests; hasil runtime pada J berasal dari parent, bukan klaim reviewer menjalankan ulang checks. Review tidak mengasumsikan anti-rollback seluruh DB.

## L. Remaining risks

1. **Full coherent DB rollback/replay tidak dicegah AEAD.** Jika attacker mengembalikan ciphertext beserta semua identitas/metadata/state/audit yang konsisten, autentikasi lama dapat tetap valid. Pencegahan memerlukan trusted freshness/state di luar salinan DB, kontrol akses/integritas storage dan prosedur restore; tidak ditambahkan diam-diam pada tugas ini.
2. Key compromise atau process/same-user compromise berada di luar jaminan record binding. Purpose separation bukan sandbox OS. Random nonce96bit dan constraint unik mengurangi/menolak collision pada data tersimpan, bukan klaim matematis tidak mungkin collision selamanya.
3. Legacy quarantine bukan konversi/recovery. Data non-disposable tidak diinspeksi; tidak mengklaim tidak ada legacy di luar fixture. Sebarang kompatibilitas nyata memerlukan otorisasi terpisah.
4. D-030 timer/irreversible deletion dan retensi metadata/backup belum diselesaikan. Logical delete bukan secure media erase.
5. Fixture sintetis positif bukan bukti recognition manusia, kalibrasi FAR/FRR/threshold, toleransi kacamata atau PAD/liveness. Batas preview-cache/native/mirror/background/accessibility serta F3 lama tidak diselesaikan pada FL-SEC01.
6. Tidak ada manusia, kamera/ADB, ESP-CAM, enrollment production, perubahan auth/session/role/SMTP/DNS/model atau migrasi Product Owner.

## M. GO/PARTIAL/NO-GO menuju FL-ENR05

**FL-SEC01 PASS:** seluruh field kontrak terikat; swap target yang sebelumnya berhasil ditolak; authentic legacyv1 tidak fallback; real-modelv2 positive dan withdrawal tetap berjalan; migration test mempertahankan v1/ledger/FK dan tidak menyentuh database asli.

**GO terbatas untuk review/perencanaan FL-ENR05 sintetis/disposable dan penutupan blocker AAD R2.** Pelaksanaan FL-ENR05 tetap memerlukan task/persetujuan pemilik tersendiri dan evaluasi residual. **NO-GO human enrollment/production/migrasi Product Owner**, termasuk irreversible D-030 deletion yang belum direkonsiliasi.

**STOP setelah laporan FL-SEC01. Tidak memulai FL-ENR05 atau aktivitas biometrik manusia secara otomatis. Tidak ada commit/push/merge.**

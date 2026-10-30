# FL-ENR05 — Full Synthetic Enrollment End-to-End Integration

Tanggal: 28 Oktober 2026. **Hasil: GO — Synthetic Integrated E2E.** Berlaku hanya untuk fixture prosedural, staging lokal, dan database disposable. Bukan izin enrollment manusia, production, migrasi Product Owner, face attendance, atau tahap berikutnya.

## A. Git baseline dan preservation

- Branch `feat/fl-otp01-email-auth`; HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Local `origin/main`: `95d31d8fa03d9bdaaee035da9a0bba6b06b4bc00`; divergence HEAD/main `21/1`. PR #19 telah diverifikasi merged pada pekerjaan sebelumnya; tidak di-merge ulang. Tidak melakukan pull/reset/stash/clean/rebase/switch/commit/push/merge pada tugas ini.
- Baseline dirty berisi pekerjaan FL-CORE/FL-CAM/FL-ENR/FL-SEC/FL-STAB existing; tidak dianggap telah masuk main atau dibuang. Pemeriksaan baseline mencatat 60 tracked modifications dan 198 untracked files sebelum implementasi baru.
- SHA256 APK utama tetap `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`; APK staging tetap `c8cb730c590022d2dc538df3ace7ef50d4b726d393614f11c6d75a48a4ea8134`.
- Kedua MP4 dan berkas pendukung pada `apps/mobile/assets/video` dibandingkan dengan baseline dan tidak berubah. Model YuNet/SFace juga tetap checksum pinned.
- Tidak membaca/migrasi database Product Owner, memakai akun/OTP/kunci production, membaca SecureStore existing, mengubah SMTP/DNS, atau menjalankan ADB/kamera.

## B. Integrated test environment

Command existing `backend/cmd/enrollment-staging-serve` mendapat mode `-integration-fixtures`. Mode ini wajib HTTPS, semua tiga `-cv-*` flag, serta concrete `enrollmentcv.Processor`; menolak `-admin-fixtures`, `-test-tokens`, dan `-insecure-http`. `cmd/server` tetap tanpa enrollment.

Satu proses Go membuat satu SQLite WAL disposable schema8 melalui migration00001–00008 existing; tidak ada migration baru FL-ENR05. KeyProvider test memakai key acak terpisah pending-photo/template, hanya di memori. Fixture seed hanya akun, person, identitas nonbiometrik dan roster; tidak menanam consent/submission/template/ACTIVE. Student, Admin, reviewer2 dan Dosen fixture terisolasi; login melalui request/verify OTP normal.

Worker real dipause agar status HTTP202 sebelum processing dapat diobservasi. Kendali hanya JSONL stdin, bukan endpoint HTTP: `worker.step`, `snapshot`, `reconcile`, forward-only `clock.advance`, dan `otp.wait`. Input bounded dan strict; error/output disanitasi. `otp.wait` hanya membaca committed delivery challenge tertentu, bounded5detik/cancellable, bukan verifier atau bypass OTP. EOF menghentikan server, menunggu worker/child, menutup DB dan menghapus root disposable milik run.

## C. Student submission path

`apps/mobile/tests/enrollmentCombinedE2E.test.ts` memakai `authApi`, enrollment API dan `EnrollmentFlow` aktual. Rangkaian yang dijalankan:

1. OTP Student sah → notice `NOT_GIVEN`/`NOT_ENROLLED`.
2. Consent eksplisit → grant tersimpan melalui API.
3. Upload JPEG fixture R2 existing SHA256 `cf572a3ef381f4b52087c9dac062d0af4da6ca352b1e55742674a1c308e6cabe`, tanpa foto/manusia atau kandidat baru.
4. Backend canonicalize, encrypt private pending file, persist submission `PENDING_APPROVAL` dan audit.
5. Student membaca pending dari backend; belum trusted ACTIVE.

Artefak original fixture tidak ditimpa. Preview dan buffer upload hanya dipakai di memori oleh pengujian, tidak dipersist/log ulang. Canonical SHA berbeda dari source JPEG karena normalisasi yang sah.

## D. Admin approval path

Admin login OTP normal, membaca queue/detail lewat client/controller aktual, melihat account-person-roster/consent eligible dan mengambil protected preview. SHA256 preview sama dengan canonical submission. Attestation memakai UUID referensi identitas fixture yang valid; tidak menganggap email OTP membuktikan identitas kampus.

Approve aktual mengembalikan202 `APPROVED_PROCESSING`, template_readyfalse; snapshot membuktikan template0/activation0/processing_calls0 sebelum `worker.step`. Tidak ada SQL shortcut untuk approval/activation dan tidak ada deterministic processor untuk klaim positif ini.

## E. Real YuNet/SFace execution

Go mendekripsi approved artifact yang sama, lalu concrete processor menjalankan Python melalui HMAC pipe privat existing. Result terikat private request UUID/digest/manifest, bounded dan tervalidasi. Observer staging hanya mencatat hash/scalar metadata dari pemanggilan nyata; tidak mengubah result atau state.

- YuNet `2023mar`: SHA256 `8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4`.
- SFace `2021dec`: SHA256 `0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79`.
- Pipeline `enr04-synthetic-native-v2`; threshold0,75/quality gate/model tetap.
- Fixture512×512: satu deteksi, confidence0,907666, landmark valid, alignment112×112×3, embedding128 float32 finite/unit-normalized. Ini bukan akurasi identitas manusia atau probabilitas identitas.

Pengukuran terpisah memakai script existing `measure_existing_fixture.py`; label task historis FL-ENR04-R2 di output script dipertahankan, bukan run lama yang dipakai sebagai bukti baru.

| Tahap satu pengukuran lokal | Milidetik |
|---|---:|
| Model init |105,333|
| Decode |0,992|
| YuNet |26,303|
| Alignment |0,237|
| SFace |60,083|
| Full pipeline, pemanggilan berikutnya |55,765|
| Authenticated IPC round trip |526,317|

Peak working set proses pengukuran146,547MiB; child IPC exit0/reaped. Angka merupakan satu pengukuran, bukan SLA, hard memory cap, total mesin, atau jaminan zeroization.

## F. Encrypted AAD v2 activation

Go memvalidasi result lalu memakai implementasi FL-SEC01 tanpa melonggarkan binding: domain/purpose/version, immutable template/submission/person/grant, model/version/complete fingerprint dan key version. Snapshot merekonstruksi binding dari joined record terpercaya dan melakukan authenticated Open di server; plaintext segera dibersihkan dan tidak keluar sebagai evidence.

Run membuktikan satu pemanggilan processor, face_count1/dim128/finite, manifest cocok, envelope_version2/binding valid, template1 dan active enrollment1. Canonical SHA, protected-preview SHA, dan processed-input SHA sama. Student dan Admin membaca ACTIVE/template_readytrue dari backend. Setelah activation, explicit reconciliation existing menyelesaikan penghapusan fisik pending photo; bukan klaim bahwa commit logical purge sendiri langsung menghapus file.

Legacyv1/unknown, envelope swap, metadata mismatch dan key-loss fail-closed tetap diuji oleh suite crypto/domain existing. Tidak ada fallback template v1, migration baru, atau perubahan KeyProvider.

## G. Withdrawal/revocation/deletion

Student menarik consent lewat API aktual. Kedua client membaca REVOKED/template_readyfalse; preview ditolak410. Setelah reconciliation, photo rows0/template rows0/active enrollments0/ciphertext files0/deletion incompletefalse. Immutable grant identity/evidence digest tetap sama setelah status withdrawal berubah.

Cleanup yang dibuktikan adalah logical revoke, physical deletion file disposable dan reconciliation/WAL maintenance sesuai kontrak. Bukan forensic erase SSD/paging/backup, bukan otorisasi irreversible key-loss deletion D-030.

## H. Failure dan concurrency

Seluruh coverage berikut dijalankan dalam suite final; lapisan bukti dibedakan. Dependency faults tidak dipakai untuk mengklaim real-model positive PASS.

| # | Kasus | Bukti/lapisan yang dijalankan |
|---|---|---|
|1|Consent belum diberikan|TLS HTTP `TestEnrollmentFailureHTTPSConsentAndDecisionFences/consent-not-given`:422, tidak persist artefak/aktivasi.|
|2|Withdrawal sebelum approval|TLS HTTP `/withdraw-before-approval`:stale approval412, REVOKED, tidak queued/running/ACTIVE.|
|3|Withdrawal saat inference|TLS HTTP `TestEnrollmentFailureHTTPSInFlightResultsAreFenced/consent-withdrawal`:blocked dependency result tidak dapat mengaktifkan setelah withdrawal aktual.|
|4|Tepat72jam|TLS HTTP `TestEnrollmentFailureHTTPSExact72HourRetention`:preview tersedia deadline−1ms, EXPIRED/410 tepat deadline, approval409, cleanup0.|
|5|Retry tidak memperpanjang TTL|TLS HTTP `TestEnrollmentFailureHTTPSRetryKeepsOriginal72HourDeadline`:recoverable key outage, retry202, expiry sama, tepat deadline tidak ACTIVE.|
|6|Roster/binding tidak valid|TLS HTTP `/invalid-roster-binding` dan domain `TestAccountRosterAndLinkRestorationDoNotResurrectWork`:ditolak dan tidak resurrect.|
|7|Attestation tidak sah|TLS HTTP `/attestation-and-etag`:unconfirmed/version/method/reference invalid ditolak.|
|8|Dua Admin bersamaan|Domain `TestDecisionRaceIsSingleImmutableCAS` dan Admin HTTPS TS concurrent-review test:satu keputusan immutable.|
|9|Role reviewer hilang|TLS HTTP `/reviewer-role-revoked-restored`:403 saat demoted; restore tidak menghidupkan authority approval lama.|
|10|ETag/CAS conflict|TLS HTTP wrong strong ETag412 dan Admin HTTPS concurrent/replay cases.|
|11|Lease stale|TLS HTTP `/stale-lease`, domain `TestInferenceLeaseRevocationAndImmutableBindings`:late result tidak activate.|
|12|Python unavailable|Adapter `TestPythonExecutableUnavailableBeforeAndAfterConstruction`:configuration/process launch menolak, sanitized error tanpa output tepercaya.|
|13|IPC timeout/crash|Actual child-process protocol-helper tests `TestProtocolAndProcessFailuresClose/crash`, `TestDeadlineKillsReapsChild`, `TestCancellationKillsReapsAndReleasesSlot`; bukan successful real-model inference.|
|14|NO_FACE|Real pinned-model Python negative `test_real_pinned_pipeline_rejects_generated_nonhuman_jpeg`; adapter/domain propagation terpisah tanpa ACTIVE.|
|15|Model corrupted|Python checksum-before-runtime tests `test_model_failure_precedes_any_runtime_construction[corrupt]`, `test_corrupt_sface_prevents_loading_verified_detector`; corrupt disposable bytes, bukan model asli diubah.|
|16|Invalid embedding|Adapter protocol dimension/norm/zero/nonfinite, domain `TestExtractionErrorsNeverActivateAndConservativelyPurge`, Python `test_invalid_embedding_never_returns_success`.|
|17|Key unavailable|TLS retry/decryption outage dan domain `TestKeyFailurePreservesCiphertextAndWithdrawalNeedsNoKey`, template-encryption/key-loss/restart tests.|
|18|Ciphertext/AAD corrupted|New domain `TestEnrollmentCorruptPendingEnvelopeNeverReachesProcessorOrAutoDeletes`:processor tidak dipanggil, ciphertext retained/quarantined; explicit withdrawal membersihkan. Template tampering/swap tests existing.|
|19|Legacy templatev1|`TestLegacyTemplateUpgradeRemainsUnsupportedAndRetained`:authentic historicalv1 unsupported, tidak auto-convert/delete.|
|20|Client kehilangan response approval|Combined HTTPS case kedua:real Go commit202 lalu transport response-loss diinjeksikan, uncertainty/noACTIVE; exact body/CAS/key replay200, audit+0; cleanup0. Bukan physical network-disconnect/Android.|
|21|Logout access expired|HTTP `TestLogoutExpiredAccessWithRefreshRevokesSession`:access401, logout refresh204, refresh ditolak, revocation durable; auth/client regression final.|
|22|Restart state transisi|`TestEnrollmentRestartReopensSQLiteAndReconcilesTransitions`:SQLite benar-benar close/reopen, service/FileStore baru; pending/queued/running/failed/active/withdrawn. Bukan OS-process kill test.|
|23|Cleanup failure/reconcile|`TestEnrollmentCleanupFailurePersistsAndReconcilesAfterRestart`:owned-directory outage, revoke tetap, FAILED+retry/audit durable, restore/reopen/reconcile selesai tepat sekali.|

Receipt idempotensi24jam tidak disamakan dengan retensi foto72jam: replay sah diuji sebelum24jam; re-upload dengan key lama setelah receipt habis tetapi submission masih live ditolak409, tanpa perpanjangan TTL.

## I. RBAC dan authentication

Backend middleware dan transaksi domain tetap authoritative: Student hanya self-consent/submission/status; Admin review; Dosen tidak mendapat Admin queue/detail/photo/decision. Existing Admin HTTPS role/status denial serta Go HTTP/domain ownership/cross-user tests lulus. Login client baru memakai HTTPS+OTP normal, tanpa startup bearer tokens atau SecureStore existing.

Tidak mengubah OTP/session/role policy. Sinkronisasi `otp.wait` hanya infrastruktur staging agar test tidak menebak delivery readiness; tidak memverifikasi kode, tidak mengubah DB auth, dan tidak diekspos melalui HTTP. Expired-access logout diuji sebagai regresi FL-STAB01, bukan kontrak auth baru.

## J. Cross-component findings dan penyelesaiannya

1. **Harness sebelumnya terpisah:** Student HTTP/token fixture, Admin HTTPS/deterministic, real-model Go test tanpa dua client TS. Mode baru menjalankan satu positive chain nyata dalam satu DB/process; Student E2E existing dimigrasikan ke normalOTP/HTTPS.
2. **Attestation fixture mismatch:** default reference `roster-synthetic-v1` tidak memenuhi UUID campus-check client/HTTP. Integration-only seed memakai UUID valid dan actual person reference; validator tidak dilonggarkan.
3. **P2 Student stale ACTIVE:** load/refresh yang gagal memperoleh token tidak menandai cached ACTIVE stale. Tiga regresi load/manual/background FAIL-before; dua perubahan `stale:true` menutup trust;26 controller tests PASS-after. Combined real-ACTIVE smoke juga menguji credential-unavailable→untrusted→real authenticated reload→trusted.
4. **P2 OTP readiness race:** rename outbox terjadi sebelum delivery commit. Semua tiga helper E2E kini menunggu exact committed challenge melalui bounded stdin barrier, lalu satu verifikasi OTP normal; tidak memakai fixed delay atau retry-verification.
5. **P2 cleanup assertion timing:** activation dan withdrawal memerlukan reconciliation untuk physical cleanup/COMPLETE. Test memanggil existing reconcile pada kedua batas; assertion nol tetap.
6. **Test contract24h/72h:** run Go awal gagal karena mengharapkan replay pada72jam−1ms padahal receipt24jam. Setup/assertion diselaraskan dengan kedua kontrak independen; tidak mengubah retensi runtime.
7. Satu run typecheck gagal karena helper polling ACTIVE `delay` terhapus saat helper OTP diubah; helper dikembalikan. Verifikasi final di bawah lulus. Dua trace FAIL awal dipertahankan, tidak dipromosikan menjadi PASS.

Independent security review dilakukan pada boundary/control/privacy/trace dan state/auth/crypto/withdrawal/cleanup/client freshness. Closure static review mengembalikan findings kosong setelah perbaikan. Reviewer tidak menjalankan checks; bukti runtime berasal dari perintah parent. Tidak ada evidence-backed blocker tersisa dalam cakupan ini; bukan jaminan tidak ada semua vulnerability/race.

## K. Mobile/UI contract review

Client/controllers aktual menjalankan consent, pending, processing, server ACTIVE, withdrawal, background/resume revalidation serta credential-unavailable distrust. Admin queue/detail/protected preview/approve202 dan exact replay berjalan melalui HTTPS nyata. Reject/retry/expiry/role/status/CAS tercakup existing HTTPS client dan Go TLS failure tests. Existing mocked controller tests tetap berlabel lapisan controller, bukan render Android.

**Android UI/device, TalkBack menyeluruh dan APK rebuild: NOT_RUN pada FL-ENR05.** Tidak ada perubahan native/dependency/permission atau kebutuhan build native untuk perubahan controller/test ini; tidak ada izin ADB baru. Tidak mengklaim render, native preview-cache zeroization, perbaikan mirror/crop absolut, interruption ENCODING, atau penutupan F3 confirmation flicker. Main route dan feature flag enrollment tetap default-off.

## L. Regression, commands dan files

| Pemeriksaan final | Hasil |
|---|---|
| Go `go test -count=1 ./...` |18 paket ok;4 tanpa tests|
| Go `go vet ./...` |Exit0, sesudah full tests sukses|
| Python seluruh tests |217PASS,1SKIP; skipped bukan PASS|
| Mobile typecheck/lint |Lulus|
| Mobile dengan seluruh opt-in client E2E aktif |17 test files;230PASS;0failed|
| Client E2E di dalam run tersebut |AdminHTTPS5, StudentHTTPS1, combined2:8PASS|
| Offline native/JS contract suites |36PASS; bukan Android codec/device run|
| Real pinned model smoke/measurement |Satu wajah,128float32finite/unit, authenticated IPC exit0/reaped|
| APK/video/model preservation |Hash baseline tidak berubah|

Perintah dari root, kecuali folder ditandai:

```powershell
services/face-engine/.venv/Scripts/python.exe -m pytest services/face-engine/tests
services/face-engine/.venv/Scripts/python.exe services/face-engine/scripts/measure_existing_fixture.py --fixtures research/fl-enr02a-host/evidence-local/enr04-r1/fixtures-v1-verified --output research/fl-enr02a-host/evidence-local/enr05/model-measurement-20261028.json
node --test research/fl-cam00-proof/nativeCaptureAdapter.test.mjs research/fl-cam00-proof/guidanceModel.test.mjs research/fl-enr02a-host/mannequinTrace.test.mjs
# backend/
go test -count=1 ./...
go vet ./...
# apps/mobile/; NODE_EXTRA_CA_CERTS wajib diset sebelum memulai Node
npm run typecheck
npm run lint
$env:FACELINK_ADMIN_ENR_E2E="1"
$env:FACELINK_ENR_E2E="1"
$env:FACELINK_ENR05_E2E="1"
$env:NODE_EXTRA_CA_CERTS=(Resolve-Path ../../backend/dev/certs/dev-ca.pem).Path
npm test
```

Script measurement menolak overwrite evidence existing; gunakan nama output baru jika reproduksi. Python/model fixture dan sertifikat CA harus tersedia; tidak ada download/fallback otomatis. Toolchain aktual: Go1.27.0 windows/amd64, Node24.21.0/npm11.19.0, Python3.12.14/OpenCV4.12.0/NumPy2.2.6. Node contract suites menghasilkan warning existing MODULE_TYPELESS_PACKAGE_JSON; tidak disembunyikan atau mengubah package unrelated.

Berkas source/test yang diubah/ditambah pada tugas ini:

- `backend/cmd/enrollment-staging-serve/{main.go,main_test.go,integration_fixtures.go,integration_fixtures_test.go,admin_fixtures.go,otp_controls.go,otp_controls_test.go}`.
- `backend/internal/httpapi/enrollment_failure_integration_test.go`.
- `backend/internal/enrollment/failure_integration_test.go`.
- `backend/internal/enrollmentcv/failure_integration_test.go`.
- `apps/mobile/src/features/enrollment/flow.ts` dan `apps/mobile/tests/enrollmentFlow.test.ts`.
- `apps/mobile/tests/{enrollmentCombinedE2E.test.ts,enrollmentE2E.test.ts,adminEnrollmentE2E.test.ts}`.
- Dokumentasi laporan ini, README root/backend/mobile, PROJECT_STATE, BACKLOG, TEST_PLAN, API_CONTRACT, ARCHITECTURE, SECURITY_PRIVACY dan DECISIONS.

Evidence JSON tersimpan pada `research/fl-enr02a-host/evidence-local/enr05/`, diabaikan Git sesuai aturan existing; tidak menambah JPEG/model/vector/key/OTP ke repository. Tidak ada layanan/watchers/temporary scripts baru yang ditinggalkan oleh run; root Go disposable dibersihkan melalui EOF dan diverifikasi oleh tests.

## M. Remaining risks

D-030 recovery timer/irreversible deletion, retensi backup/insiden, human consent notice final, preview bitmap cache, PAD/liveness, threshold calibration/FAR/FRR/accuracy, mirror/crop visual absolut, interruption ENCODING, TalkBack menyeluruh dan F3 Admin flicker tetap terbuka. AES-GCM AADv2 tidak mencegah full coherent DB rollback/freshness replay atau key/same-user compromise. Tidak ada sandbox OS/hard memory cap/zero memory leak/zero SDK telemetry/forensic erase claim.

Response-loss test menginjeksi kehilangan response setelah real commit, bukan memutus Wi-Fi/perangkat. Restart test menutup/membuka ulang SQLite dan service, bukan kill/restart OS process. Cleanup outage memakai direktori milik fixture, bukan disk exhaustion. Keterbatasan lapisan ini tidak disamarkan sebagai uji Android/production. Synthetic positive tidak membuktikan human recognition atau hak menyalakan production.

## N. Keputusan GO/PARTIAL/NO-GO dan satu correlation trace

**GO — Synthetic Integrated E2E**: satu continuous chain TypeScript→Go→real Python→Go→TypeScript terbukti, approved artifact sama, encrypted template AADv2 valid, server ACTIVE, withdrawal/revocation/deletion berjalan; failure/RBAC/security regressions relevan lulus. Tidak ada keputusan tambahan untuk menutup tugas sintetis ini. **NO-GO untuk manusia/production/migrasi Product Owner/face attendance** tanpa otorisasi dan security gates terpisah.

Trace satu run: `ab77bc28-821c-458a-a1a0-cfd10708d918`.
Positive submission UUID sintetis: `c448ea0c-4a8d-4400-b2e6-8af54a64a0a2`.
Canonical/preview/processed SHA256: `00eaa7792b75bd3f7f2ad10bfcbbb5fd3afe64fec4b626761a0e42b239cace76`.
Artefak: `research/fl-enr02a-host/evidence-local/enr05/combined-ab77bc28-821c-458a-a1a0-cfd10708d918.json`.

| Tahap positive chain | State backend | Foto/template/active/file ciphertext |
|---|---|---|
|Student submit|PENDING_APPROVAL|1/0/0/1|
|Admin approve202, worker belum jalan|APPROVED_PROCESSING|1/0/0/1|
|Actual Python satu wajah,128D; templatev2 valid; reconciliation|ACTIVE|0/1/1/0|
|Student withdraw; kedua client membaca revoke; reconciliation|REVOKED|0/0/0/0|

Trace juga memuat kasus kedua response-loss/replay terpisah pada DB/run yang sama: server202, client uncertain/noACTIVE, identical body/CAS/key replay200, approval audit+1 lalu replay+0, withdrawal cleanup0; tidak memanggil processor untuk kasus kedua. Keseluruhan run48verifiedHTTPS requests, kedua kasus PASS; child EOF/root_removedtrue. Hasil positive tidak disusun dari run berbeda.

Korelasi jujur: HTTP audit memakai `X-Request-ID` trace di atas. Worker audit existing memang request_id kosong; observer menghubungkan actual Process input SHA/result tervalidasi/manifest ke submission yang RUNNING dan trace yang sama. Python mempertahankan nonce/request UUID privat sendiri; tidak mengarang bahwa trace ID telah ditambahkan ke protokol Python atau worker audit. Hanya metadata aman yang disimpan.

**Rekomendasi berikutnya:** tinjau laporan/residual dan tetapkan tugas terpisah bila Product Owner menginginkan kelanjutan. **STOP setelah FL-ENR05; tidak memulai human pilot, migrasi production, absensi wajah, UI overhaul, atau commit/push/merge.**

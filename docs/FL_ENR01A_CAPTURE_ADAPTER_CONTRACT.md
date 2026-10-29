# FL-ENR01A — Kontrak Native Capture Adapter

Status: kontrak implementasi **prototype terisolasi**, bukan API enrollment backend atau persetujuan integrasi aplikasi utama. Implementasi: `research/fl-cam00-proof/`. Android saja; Expo Go dan iOS tidak didukung. Hasil verifikasi aktual dan gate ada di `FL_ENR01A_TECHNICAL_REPORT.md`.

## 1. Boundary dan urutan pemakaian

Host tepercaya memiliki permission, foreground, pemasangan kamera, guidance, dan consumer native. Adapter memiliki transaksi capture serta buffer JPEG/scratch yang dialokasikannya. Tidak ada shutter manual pada UI: callback live terbaru yang memenuhi guidance memicu approval otomatis.

1. Aktifkan proteksi layar lokal `FLAG_SECURE`; periksa/request `CAMERA` secara eksplisit saat pengguna memulai scanning. Adapter tidak meminta izin global, tidak membuka kamera sendiri, dan tidak mengaktifkan kamera saat resume.
2. `CaptureAdapter.start(generation)` meng-arm native. Host memasang preview depan portrait dan detector output. `start` bukan jaminan preview sudah READY.
3. Event `CaptureFrame` membawa generation, frame ID, waktu monotonic native, bounds/pose/jumlah wajah. Event tidak boleh dicatat sebagai data wajah mentah. `guidanceModel` existing menguji tepat satu wajah, seluruh bounds dalam oval, pose, drift, dwell ≥1.500 ms, ≥6 sampel, gap ≤400 ms, lalu satu observasi valid berikutnya. Fixture waktu virtual bukan input kamera live.
4. Untuk setiap kandidat, host memanggil `decide(generation, frameId, approved, sensorCornersOrNull)`. Approval hanya dari terminal guidance live; false selalu mengirim argumen keempat `null`. Empat sudut preview dikonversi kembali ke sensor melalui `CameraRef`, bukan rasio dimensi buatan.
5. Approval mengikat **ImageProxy yang sama** setelah ML Kit complete. Native menerapkan sensor-to-buffer transform, contained/even crop, orientation dan mirror melalui `Cam02Pixels`. Tidak ada pengambilan foto kamera kedua.
6. Hasil `CaptureLease` memiliki metadata READY dan `consumeOnce()`. Handle opaque privat facade tidak ikut `JSON.stringify(lease)`.
7. `consumeOnce()` memanggil consumer native sinkron. Prototype consumer memeriksa marker JPEG dan decode bounds langsung dari view terbatas; tidak membuat preview bitmap, file, URL, upload, atau template. Receipt `CONSUMED` hanya setelah consumer kembali dan cleanup buffer fisik selesai.
8. Host menutup/unmount kamera **setelah konsumsi selesai**, atau membatalkan jika ada error/background. Source close mencabut handle yang belum dikonsumsi. `cancel` dipakai untuk siklus biasa; `dispose` terminal untuk lifetime instance.

Guidance adalah kewajiban host tepercaya, bukan bukti approval kriptografis dari JS. Native menegakkan identitas/freshness/ownership/resource/lifecycle; boolean approval tidak membuktikan consent, identitas, PAD, atau enrollment.

## 2. API JavaScript dan React Native

Facade: `nativeCaptureAdapter.ts`. Bridge: `NativeModules.FaceLinkCapture`; event: `CaptureFrame`. Tidak ada alias bridge lama.

| Operasi | Hasil / kewajiban |
|---|---|
| `start(generation: number)` | Promise void; generation integer aman JS, positif, meningkat ketat; hanya saat transaksi lama sudah drain |
| `CaptureFrame` | Metadata detector untuk guidance host; frame ID string decimal Long, generation dan waktu native; bukan JPEG |
| `decide(generation, frameId, capture, cornersOrNull)` | False → null; true → lease READY untuk frame yang disebut, atau safe error |
| `lease.metadata` | Whitelist generation/frameId/dimensi/jumlah byte/EXIF=false, encodeMs, selectedAgeMs, peakOwnedBytes, expiresAtMs; tidak ada handle |
| `lease.consumeOnce()` | Promise receipt `CONSUMED`, `consumed:true`, identitas/dimensi/byte count/EXIF=false; tidak ada byte gambar |
| `cancel()` | Mencabut epoch/handle secara lokal segera; native acknowledgement menunggu transaksi capture dan callback notifikasinya drain |
| `dispose()` | Mencabut lease dan menutup instance secara terminal; gunakan instance native baru, bukan menghidupkan kembali instance disposed |
| `getStatus()` | Diagnostik metadata native; tidak mengeluarkan handle atau isi file/gambar |
| `runSyntheticVerification()` | API diagnostik prototype eksplisit, fixture YUV sintetis + codec Android nyata, bukan scanning/permission kamera/live proof |

Suite diagnostik memakai instance adapter terpisah dan reservasi global agar tidak bersaing dengan scanning. Promise `cancel()` untuk scanner tidak boleh dipakai sebagai bukti suite diagnostik sudah selesai; tunggu Promise suite/status reservasinya. Reservasi tetap menghalangi capture baru bila codec fixture belum drain.

`consume(generation, handle)` pada bridge hanya memilih consumer validasi native yang tetap. Tidak tersedia callback JS yang menerima byte, base64, path, atau URI. Integrasi consumer bisnis kelak dilakukan di native dengan review terpisah; task ini tidak menyediakan submission/upload.

Handle native adalah capability acak 256-bit (64 karakter hex), terikat generation dan transaksi, sekali pakai, dicabut saat cancel/timeout/dispose/source close. Jangan mencetak, menyimpan, mengirim, atau menyebarkannya melalui state diagnostik. String handle di JS bukan buffer gambar dan tidak dijanjikan dapat di-zeroize.

## 3. API Kotlin dan ownership

`NativeCaptureAdapter` tidak bergantung pada Android, storage, React Native, atau transport. Host menyediakan clock monotonic, worker `Executor`, deadline scheduler, dan `AdapterEncoder`.

- `start(generation: Long)`; satu capture yang diterima menghabiskan generation tersebut, termasuk bila kemudian gagal.
- `capture(frame: AdapterFrame, done: (ReadyCapture?, String?) -> Unit): Boolean`.
  - **true:** ownership input pindah ke adapter; callback selesai tepat sekali.
  - **false:** tidak ada callback dan tidak ada close oleh adapter. Caller menutup input baru yang ditolak. Jangan menutup ulang objek yang ownership-nya sudah ditransfer ketika objek aktif yang sama disubmit lagi.
- `AdapterFrame`: generation, frameId, acquiredAtMs, width/height, `close()`. Producer Android meminjam plane views dari ImageProxy; adapter menutup input tepat sekali setelah encode/abort aman.
- `AdapterEncoder.encode(frame, output: OutputStream, scope: EncodingScope): ImageInfo`: sinkron; hanya input yang disetujui. Gunakan `scope.allocateScratch()` untuk scratch milik adapter dan `scope.checkActive()` antar langkah/row. Jangan memakai scope/output/frame dari thread atau operasi asynchronous lain.
- `consume(generation, handle, consumer: (ByteBuffer, ImageInfo) -> Unit): ConsumeReceipt`: klaim atomik satu pemenang. ByteBuffer read-only, capacity/limit hanya panjang JPEG sebenarnya. Consumer sinkron, di luar lock adapter, tidak boleh retain/copy/persist/upload buffer. Kontrak ini bukan perlindungan dari native consumer berbahaya yang sengaja melanggar lifetime.
- `cancel(reason, drained)` / `dispose(drained)`: invalidasi logis segera, acknowledgement setelah physical cleanup dan callback notifikasi selesai. Jangan memblokir callback encoder/consumer/notifikasi sambil menunggu drain miliknya sendiri.
- Executor/scheduler milik host. Adapter tidak mematikan executor yang diinjeksikan. Prototype memiliki worker native dan scheduler sendiri; fixture test menutup executor miliknya.

Scratch diisi nol setelah encoder keluar. JPEG backing diisi nol setelah consume selesai atau saat cleanup cancel/error/timeout. Input kamera dapat dilepas setelah encoding; handle tetap valid hanya selama source/generation masih valid. Input CameraX/ML Kit, salinan internal codec/decoder/GPU/OS dan runtime bukan buffer milik adapter.

## 4. Batas resource dan waktu

| Batas produksi | Nilai |
|---|---:|
| Piksel input dan output | 2.073.600 maksimum |
| Kapasitas JPEG backing | 4.194.304 byte |
| Total explicit image buffers milik adapter | 16.777.216 byte |
| Freshness input sebelum encoding | 400 ms |
| Timeout encoding logis | 5.000 ms |
| TTL handle READY | 5.000 ms |
| Timeout borrow consumer logis | 5.000 ms |
| Capture aktif | 1, termasuk READY/CONSUMING/draining callback |

JPEG backing dicadangkan dengan kapasitas tetap; NV21 scratch sebesar `width × height × 3 / 2` untuk crop output, bukan salinan penuh input tambahan. JPEG tidak disalin ke array ukuran tepat atau JS. JNI codec dan bounds decoder boleh mempunyai alokasi internal yang tidak dihitung dalam cap adapter; **cap ini bukan RSS/process cap**.

Deadline berlaku per fase, bukan satu total 5 detik untuk seluruh pipeline. Secara logis encode + READY + consumer dapat mencapai sekitar 15 detik; freshness queued frame tetap dibatasi 400 ms. Native codec/consumer yang belum kembali tidak boleh dibebaskan/wipe dari bawah pemakai aktif. Timeout mencabut kelayakan hasil segera, tetapi release fisik bersifat cooperative dan tidak mempunyai jaminan hard real-time bila runtime/platform macet.

`expiresAtMs` memakai clock Android monotonic. Jangan bandingkan dengan UTC, `Date.now()`, atau origin `performance.now()` JS. Native memeriksa expiry saat consume, bukan hanya mengandalkan timer atau UI.

## 5. Error dan konkurensi

Safe errors mencakup generation/identity/stale input atau frame, handle, busy/disposed, permission/foreground/privacy, timeout encoding/READY/consume, resource cap, codec/consumer/source/callback/scheduler. Pixel validation existing memakai safe `E_CAM02_*`; adapter memakai `E_CAPTURE_*`, suite `E_ENR01A_*`. Facade hanya meneruskan code dengan pola aman, bukan pesan/stack native.

Error tidak menghasilkan receipt sukses atau lease yang dapat dicoba ulang. Cancel/background/exception tidak memulai ulang kamera. Double consume dan konsumsi generation lama ditolak. Sesudah kegagalan, lakukan cancel/drain lalu mulai generation baru secara eksplisit.

Callback READY yang sudah di-reserve dapat berlomba dengan dimulainya cancel: handle dapat sudah dicabut ketika callback menerima metadata. **Consume native tetap otoritatif.** Drain tidak diakui dan generation baru tidak diterima sampai callback tersebut kembali. Cancel tidak diklaim dapat menarik kembali callback yang sudah di-reserve.

Observer sandbox memiliki identitas sesi di bawah lifecycle lock. Cleanup task consume lama hanya menghentikan observer sesi/generation asal, bukan observer capture/suite baru atau capture yang masih busy. Observer dan counter UID bukan forensic tracing atau bukti zero-telemetry.

## 6. Permission, privacy dan dependency integrasi

- Native start/decide/consume memeriksa foreground, Activity hidup, `FLAG_SECURE`, dan CAMERA permission. Host tetap bertanggung jawab atas UI penjelasan/permission lifecycle. Suite sintetis tidak membutuhkan permission kamera.
- Activity prototype portrait-only; tidak mengubah orientasi, brightness, sleep timeout, developer/USB/DNS/SMTP atau permission global perangkat.
- `onHostPause` membatalkan transaksi; resume tidak otomatis start. Source close dan module invalidation/dispose membersihkan ownership sesuai kontrak. Force-stop membunuh proses; tidak membuktikan zeroization fisik RAM Android.
- Tidak ada gallery/file writer/image loader/upload pada jalur adapter. Cache SDK historis harus dibedakan dari output capture baru; jangan menghapus cache untuk menyembunyikan bukti.
- Tombol uji file mannequin historis masih terpisah di prototype. Jalur image-loader/file fixture tersebut bukan pipeline adapter dan tidak termasuk klaim memory-only capture; tidak dijalankan pada FL-ENR01A.
- ML Kit/DataTransport masih dependency. Release prototype menghapus INTERNET/storage/media permissions, tetapi metadata/cache SDK/IPC tidak otomatis hilang. Main app membutuhkan jaringan untuk fitur lain; boundary prototype **tidak boleh disalin sebagai klaim zero-telemetry main app**.
- Expo Development Build Android diperlukan untuk custom native modules. Expo58/RN0.88/React19.3/Nitro0.37.1 cocok pada metadata stack existing; paket native tambahan adalah VisionCamera5.2.3, detector2.1.0, Nitro Image0.15.2, source-set/patch detector yang dipin, dan registrasi capture/privacy module. `expo-dev-client` saja tidak memasukkan semua dependency itu.
- Aplikasi utama belum menerima dependency, plugin, permission/camera UI, native consumer bisnis, atau perubahan source apa pun dari task ini. Packaging reusable/autolinking dan runtime Expo Development Build utama tetap memerlukan tugas integrasi yang disetujui; tidak dijalankan otomatis.

Tidak ada consent/enrollment API, database migration, template SFace, approval P01–P12, real-human capture, PAD/liveness atau recognition accuracy di kontrak ini.

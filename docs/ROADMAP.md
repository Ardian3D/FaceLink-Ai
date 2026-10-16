# Roadmap Pengembangan FaceLink AI

Roadmap adalah **urutan pekerjaan beserta gerbang penerimaan**, bukan janji tanggal selesai. Satu tugas/branch yang dapat ditinjau dalam satu waktu. Jangan lanjut fase bila syarat fase saat ini belum teruji.

| Fase | ID tugas | Hasil yang ditargetkan | Syarat dapat melanjutkan |
|---|---|---|---|
| 0 — Fondasi | FL-000–009 | PRD, keputusan, aturan agent, aturan Git, kontrak awal | Pemilik menyetujui kebutuhan; repo bersih dan bebas data rahasia |
| 1 — Backend | FL-010–019 | Backend modular, auth, database/migrasi, orang, kelas, sesi, absensi manual, tes | Otorisasi lulus, transaksi mencegah duplikasi, check-in manual bekerja |
| 2 — Mesin AI lokal | FL-020–029 | Pemuatan model, kualitas gambar, enrollment, matching, penarikan consent, evaluasi | Wajah tidak diketahui/ambigu tidak pernah otomatis dicatat; sampel izin aktif dapat diuji |
| 3 — React Native | FL-030–039 | Navigasi, **login Google & tiga role** (`admin`, `dosen`, `mahasiswa`), sesi, roster, enrollment dan laporan | Android dapat membuka layar dan menampilkan data mock/API secara benar |
| 4 — Integrasi software | FL-040–049 | Webcam → backend → AI → database → mobile, pembaruan status, uji gagal | Alur dari persetujuan hingga rekap berjalan tanpa hardware ESP32 |
| 5 — Hardware IoT | FL-050–059 | Pilih board tepat, beli komponen sesuai anggaran, firmware, LED/buzzer | Alur yang sama bekerja lewat ESP32-S3 CAM, pemutusan koneksi tertangani |
| 6 — Validasi dan rilis demo | FL-060–069 | Pengukuran akurasi/latensi, uji berkacamata, keamanan, APK, video presentasi | APK terpasang dan demo berulang berhasil dengan laporan keterbatasan |
| 7 — Cloud opsional | FL-070+ | Tinjauan biaya dan ancaman, rancangan migrasi, eksperimen deployment | **Hanya** jika diminta setelah kelulusan seluruh uji lokal |

## Catatan FL-010U (16 Oktober 2026)

Revisi ini berasal dari **FL-010U (16 Oktober 2026)**, cabang `docs/fl-010u-google-three-roles` dari `origin/main` (`804c2f7`), dan **dokumentasi saja** — tidak ada kode maupun migrasi yang ditulis.

- **Fase 3** berubah dari "login operator" menjadi **"login Google & tiga role"**: aplikasi React Native memakai **Google Sign-In** dan menampilkan beranda sesuai `role` dari backend (`admin`, `dosen`, `mahasiswa`). Menyembunyikan menu **bukan** pengendalian akses.
- **Fase 1** adalah fase yang memuat pelaksanaan rancangan FL-010U: **FL-013** kini mencakup verifikasi ID token Google, sesi `session_tokens`, otorisasi tiga role, dan **migrasi 00002+**; **FL-011** memuat perintah bootstrap admin lokal (`-bootstrap-admin` pada `cmd/server`); **FL-012** memuat perubahan skema dan migrasi (tabel `user_identities`, `session_tokens` pengganti `operator_tokens`, `class_lecturers`, kolom `people.account_user_id`, penyesuaian `users`/`roles`) tanpa mengubah `00001_init_core_schema.sql`. Rancangan acuan matriks peran/izin: `docs/AUTHORIZATION.md`.
- **Peran `device` tidak berubah posisinya:** perangkat IoT tetap bukan role pengguna manusia dan memakai identitas serta autentikasi mesin terpisah, sesuai **Fase 5**.
- **Ketergantungan jaringan:** selama kode FL-005 masih menolak alamat non-loopback, HP tidak dapat menjangkau backend lewat Wi-Fi kelas; pilihan penyelesaiannya dicatat sebagai pertanyaan terbuka **Q-19** di `docs/OPEN_QUESTIONS.md` dan belum diputuskan pemilik.
- Perubahan status peran ini **bukan** bukti fitur berjalan. Sampai tugas implementasi dijalankan dan hasil tesnya nyata, seluruh skenario runtime tiga role dan Google Sign-In berstatus **BELUM TERVERIFIKASI**.

## Catatan FL-010V (16 Oktober 2026)

Revisi ini berasal dari **FL-010V (16 Oktober 2026)**, cabang `docs/fl-010v-final-auth-decisions`, dan **dokumentasi saja** — **tabel fase di atas tidak diubah** dan tidak ada kode maupun migrasi yang ditulis.

- **Keputusan D-031–D-037 kini berstatus disetujui** (D-031 tidak berubah; **D-032** lifecycle akun, **D-033** session management, **D-034** hak akses `dosen`, **D-035** akses jaringan lokal **pada tingkat kebijakan**, **D-036** kebijakan audit keamanan, **D-037** satu role utama per akun). Status resmi tiap keputusan ada di `docs/DECISIONS.md`, dan status pertanyaan terkait ada di `docs/OPEN_QUESTIONS.md`.
- **FL-011 siap dimulai** sebagai handoff eksplisit implementasi Google Authentication & Authorization backend dengan **12 butir lingkup wajib**: migrasi **00002+** tanpa mengubah `00001_init_core_schema.sql`, verifikasi Google OIDC (JWKS + nonce sekali pakai), bootstrap admin pertama, pengelolaan `users` + identitas Google (penautan atomik, satu role), lifecycle sesi + rotasi + batas absolut 30 hari, RBAC tiga role + middleware, pembatasan kelas `dosen`/data `mahasiswa`, audit sesuai D-036, pengujian negatif TA-01–TA-22 dan GA-01–GA-12, pengikatan LAN eksplisit + TLS, konfigurasi Google OAuth, serta CI Go (`.github/workflows/backend-go.yml`) yang **wajib selalu hijau**. Rincian, prasyarat, dan daftar yang **boleh ditunda** ada di `docs/BACKLOG.md`.
- **FL-005S adalah penguatan wajib**: access log Go dapat melaporkan `200` untuk respons `503` batas waktu dan tidak mencatat respons `500` hasil pemulihan panic; tugas penguatan log respons & pemulihan panic beserta uji regresinya **harus selesai lebih dahulu** agar logging keamanan D-036 dianggap andal.
- **Status pertanyaan berubah sebagian saja:** **Q-19** naik dari pertanyaan terbuka menjadi **kebijakan yang diputuskan (D-035)**, sementara **rincian penyediaan sertifikat lokal dan trust chain Android belum diuji** dan dicatat sebagai **Q-25**; **Q-20 → D-034** dan **Q-21 → D-037**. **Q-22, Q-23, dan Q-24 tetap terbuka** dan tidak otomatis disetujui.
- Perubahan status keputusan ini **bukan** bukti fitur berjalan. Sampai tugas implementasi dijalankan dan hasil tesnya nyata, seluruh skenario runtime tiga role, Google Sign-In, dan TLS LAN berstatus **BELUM TERVERIFIKASI**.

## Tugas pertama yang disarankan

**FL-001 — Fondasi Git dan pemeriksaan dokumentasi:** periksa repo sebenarnya, tinjau README dan aturan agent, putuskan cara kolaborasi, siapkan `.gitignore` untuk Go/Python/React Native **sebelum source atau data dibuat**, lalu jalankan pemeriksaan agar tidak ada data pribadi/rahasia. **Belum membuat aplikasi pada FL-001**.

## Cara serah-terima tiap tugas

Catat: tujuan → kriteria penerimaan → ruang lingkup berkas → perintah pengujian → hasil aktual → perubahan keputusan/dokumen → tugas berikutnya. Status selesai harus didasarkan pada hasil yang dapat diulang, bukan deskripsi agent.

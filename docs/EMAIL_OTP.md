# FL-OTP01 — Email OTP (kontrak implementasi)

Keputusan pemilik FL-OTP01 menggantikan Google sebagai login utama MVP: Continue with FaceLink aktif, Google tetap terlihat tetapi disabled dengan label Segera hadir. Implementasi Google, identitas lama, RBAC, dan sesi D-033 tetap dipertahankan. Tidak ada pendaftaran mandiri atau role dari suffix domain.

## API dan transport

Semua endpoint di bawah /api/v1, respons {data, request_id} / {error:{code,message,request_id}}, Cache-Control: no-store. HTTPS wajib di klien autentikasi DAN handler OTP (r.TLS != nil; tidak percaya header forwarded). Server tetap loopback-only. HTTP Metro bukan HTTP autentikasi.

- POST /auth/email/request: {email:string}. Email dinormalisasi lowercase+trim, maksimal 254 karakter, alamat tunggal tanpa display name/CRLF. 202 {data:{challenge_id:UUID,expires_in:300,retry_after:60,message:"Jika akun memenuhi syarat, kode akan dikirim. Periksa kotak masuk dan spam."},request_id}. Respons sama untuk invited/active/suspended/tidak terdaftar, termasuk kegagalan SMTP. Challenge opaque baru untuk setiap permintaan yang diterima, termasuk decoy; jangan ungkap status delivery. 400 VALIDATION_FAILED untuk format; 429 RATE_LIMITED + Retry-After untuk limit IP/global (sama untuk semua akun); 503 AUTH_UNAVAILABLE bila komponen belum dikonfigurasi/queue penuh; 403 HTTPS_REQUIRED pada HTTP.
- POST /auth/email/verify: {challenge_id:UUID,code:string 8 digit}. 200 memakai bentuk sesi login Google yang ada (access_token,refresh_token,expires_in,session,user). Role/status dibaca dari DB. 401 OTP_INVALID untuk salah/expired/used/superseded/delivery pending/failed/unknown/ineligible (pesan generik, tidak membedakan keberadaan akun). 429 RATE_LIMITED + Retry-After untuk limit IP/account/global. 400 VALIDATION_FAILED untuk struktur/format; 503 AUTH_UNAVAILABLE untuk layanan belum siap; 403 HTTPS_REQUIRED untuk HTTP. Limit percobaan juga berlaku pada input kode salah format yang merujuk challenge sah agar brute force tidak lolos counter.

## Challenge dan pembatasan

8 digit desimal (termasuk nol awal), crypto/rand tanpa modulo bias; peluang tebakan per challenge maksimal 5/100.000.000 sebelum limit tambahan. TTL 5 menit menurut clock server UTC, expired tepat pada batas. Verifier HMAC-SHA256 menggunakan key 32 byte acak dari berkas di luar repository, bukan hash polos dari ruang kode kecil. HMAC mengikat domain purpose, challenge ID, user ID/email binding, dan kode; constant-time comparison. Tidak menyimpan kode mentah, tidak memasukkan kode ke log/audit/error. Key stabil antar restart; ganti key membatalkan challenge outstanding, tidak mencabut sesi yang ada.

Migrasi SQLite additive baru: email challenge, identitas email terverifikasi, dan rate buckets persisten. Challenge menyimpan user ID/email binding saat request; verify mencocokkan kembali user/email/status, sehingga pergantian akun tidak mewarisi challenge. Tidak mengubah atau menghapus user_identities Google. Eligibility hanya users invited/active yang telah diundang lewat mekanisme sah. Verify pertama mengaktifkan invited. Tidak membuat user/role saat request atau verify.

Request: cooldown per email 60 detik, maksimal 5/jam per email, 30/jam per IP, 200/jam global. Request email yang melewati budget email tetap 202 dengan decoy dan tidak mengirim/membatalkan challenge valid, agar tidak menjadi oracle akun. Rate limit diterapkan juga pada alamat tidak dikenal. Permintaan resend yang diterima menggantikan challenge sebelumnya untuk email itu, termasuk bila SMTP kemudian gagal; UI selalu menjelaskan hanya kode terbaru berlaku. IP berasal RemoteAddr, bukan X-Forwarded-For. Bucket email/IP memakai HMAC, bukan PII mentah di tabel limiter. Window tetap dengan expiry; cleanup opportunistic membatasi pertumbuhan.

Verify: maksimal 5 tebakan per challenge (persisten, transaksi), 20/10 menit per email, 60/10 menit per IP, 600/10 menit global. Limit IP/global tetap berlaku untuk UUID acak/tidak dikenal. Konsumsi challenge dan pengecekan eligibility harus atomik di BEGIN IMMEDIATE; hanya satu verify konkuren boleh menang. Sesi memakai session.Store existing; bila penerbitan sesi gagal, kode tidak dihidupkan kembali (fail closed), pengguna meminta kode baru. Tidak mengubah TTL, rotation/reuse detection, revoke, last-admin/self protections.

## Pengiriman SMTP

Sender dengan worker queue terbatas menghindari request latency bergantung status akun; enqueue decoy untuk akun tidak eligible. OTP mentah hanya di memori sampai send selesai; tidak ada outbox plaintext atau dev endpoint untuk mengambil kode. SMTP error ditelan menjadi status challenge failed, tidak pernah membuat sesi atau mengirim error upstream yang membocorkan akun. Pending/failed challenge tidak dapat diverifikasi. Queue penuh menolak semua eligibility secara sama. Delivery tidak dijamin meski server SMTP menerima; ulangi hanya setelah cooldown, kode terlambat dari challenge lama tetap tidak berlaku. Worker ber-timeout, shutdown tidak mencetak payload.

SMTP_HOST, SMTP_PORT (465 implicit TLS atau 587 STARTTLS wajib), SMTP_USERNAME, SMTP_AUTH_SECRET, SMTP_FROM, SMTP_REPLY_TO, dan `SMTP_ALLOWED_FROM_DOMAINS`. Verifikasi chain+hostname TLS wajib, TLS >=1.2, tidak ada insecure flag/cleartext downgrade. AUTH hanya setelah TLS. Gmail App Password disiapkan sendiri oleh pemilik melalui 2-Step Verification; jangan meminta password utama atau secret di chat. `SMTP_FROM` boleh berbeda dari `SMTP_USERNAME` **hanya** jika domainnya dicantumkan eksplisit di `SMTP_ALLOWED_FROM_DOMAINS` sebagai pernyataan operator bahwa alias sudah terverifikasi di penyedia SMTP. Allowlist bukan bukti kriptografis: penyedia tetap yang menolak atau mengganti From, dan backend tidak boleh menulis ulang From secara diam-diam. Reply-To opsional, alamat tunggal tervalidasi. Config parsial gagal tertutup, nilai rahasia tidak ada di `Redacted()`.

Cloudflare Email Routing hanya menerima/meneruskan email, bukan mengirim OTP. Bukti E2E harus mencatat penerimaan kode dan login per alamat, termasuk delay/resend, tanpa merekam kode/tujuan routing privat.

## FL-OTP02 — Pengirim support@facelink.online (23 Oktober 2026)

**Keputusan pemilik:** OTP dikirim sebagai `support@facelink.online` melalui `smtp.gmail.com`, memakai akun Gmail asli yang sudah mengotorisasi alias tersebut sebagai **Send mail as**. Penerima: `admin@`, `dosen@`, `mahasiswa@facelink.online` (Cloudflare Email Routing).

**Konfigurasi yang berlaku:** `SMTP_FROM=support@facelink.online`, `SMTP_REPLY_TO=support@facelink.online`, `SMTP_ALLOWED_FROM_DOMAINS=facelink.online`, `SMTP_USERNAME=<akun Gmail>`, App Password sebagai `SMTP_AUTH_SECRET`. Backend **tidak** menulis ulang From. Jika Gmail menolak atau mengganti From, hasilnya dilaporkan; tidak ada pemalsuan.

**Audit DNS pada 23 Oktober 2026 (fakta, bukan hasil pengiriman):**

| Catatan | Nilai terlihat |
|---|---|
| MX | `route1/2/3.mx.cloudflare.net` (Email Routing) |
| SPF | `v=spf1 include:_spf.mx.cloudflare.net ~all` — tidak menyebut `_spf.google.com` |
| DMARC | `v=DMARC1; p=none;` (monitoring; record ditambahkan 23 Oktober 2026) |
| DKIM | hanya selector `cf2024-1` (Cloudflare); selector Google (`google`) tidak ada |

Konsekuensi yang harus diverifikasi pada header nyata: Gmail umumnya memakai `Return-Path` akun Gmail dan menandatangani DKIM dengan `d=gmail.com`, sehingga SPF/DKIM mungkin **tidak selaras** dengan domain From `facelink.online`. Karena DMARC belum ada, belum tentu ada penolakan; klaim “DMARC lulus” tidak boleh dibuat sebelum header diterima diperiksa. Menerbitkan DMARC/SPF Google adalah perubahan DNS dan memerlukan persetujuan pemilik.

**Laporan awal dari pemilik (belum diverifikasi alat):** email nyata menunjukkan `SPF=pass` melalui IP Cloudflare `104.30.8.131`, `DKIM=pass`, tetapi `DMARC=fail`. Pola ini **konsisten dengan pesan yang diteruskan Cloudflare Email Routing**, bukan dengan jalur Gmail SMTP langsung:

| Kemungkinan jalur | Ciri header |
|---|---|
| Cloudflare forwarding | `Received` memuat `cloudflare` dan `google`; `Return-Path` dapat berupa alamat bounce Cloudflare; SPF memvalidasi envelope Cloudflare, bukan Gmail |
| Gmail SMTP langsung | `Received` hanya memuat Google; `Return-Path` akun Gmail; SPF memvalidasi `gmail.com` |

DMARC menilai **alignment** antara domain `From` dan domain yang lolos SPF/DKIM. SPF PASS pada domain Cloudflare/Gmail tidak menolong bila domainnya bukan `facelink.online`; DKIM PASS juga tidak menolong bila `d=` bukan `facelink.online`. Karena itu, tambahkan `_dmarc` `p=none` **tidak memperbaiki alignment**—record itu hanya mengubah kebijakan/monitoring. Pemeriksaan pasti memerlukan `DKIM-Signature: d=`, `Authentication-Results: dkim=pass header.d=`, `smtp.mailfrom=`, dan `header.from=`. Gunakan `scripts/check-email-headers.ps1` pada `.eml`; script ini juga membedakan jalur Cloudflare-forwarded dari Google langsung.

**Penting untuk DMARC PASS yang dilaporkan:** jika `d=facelink.online` disertai selector `s=cf2024-1`, tanda tangan itu berasal dari **Cloudflare Email Routing**, bukan dari Gmail. DMARC final bisa PASS karena forwarder menandatangani ulang dengan domain sendiri. Itu tetap autentikasi yang valid untuk pesan yang diterima, tetapi bukan bukti bahwa Gmail `Send mail as` menandatangani dengan `facelink.online`. Periksa `s=` dan rantai `Received` sebelum menyimpulkan.

**Alat lokal (secret tidak masuk Git):**

- `scripts/setup-smtp.ps1` — meminta App Password dengan input tersembunyi, menyimpannya sebagai ciphertext DPAPI CurrentUser di `%LOCALAPPDATA%\FaceLink\secrets\smtp-auth.secret`, membuat OTP HMAC key jika belum ada, dan menulis konfigurasi non-rahasia `smtp.config.json`.
- `scripts/start-server.ps1` — mendekripsi secret hanya ke environment proses, lalu menjalankan backend; `-Check` memvalidasi konfigurasi tanpa membuka port.
- `scripts/send-smtp-test.ps1` — mengirim **satu** email uji tanpa OTP dan menampilkan ID uji; bukan bukti inbox.
- `scripts/check-email-headers.ps1` — membaca `.eml`, mencetak hanya From/Return-Path/SPF/DKIM/DMARC/Authentication-Results; body dan OTP tidak dicetak.

**Status jujur:** konfigurasi, validasi allowlist, unit test, dan launcher sudah diuji dengan secret dummy DPAPI. **Pengiriman nyata BELUM** karena App Password asli belum dimasukkan. Setelah pemilik menjalankan setup, bukti yang diperlukan adalah email uji beserta output pemeriksa header; baru setelah itu OTP nyata dan login Android diuji.

**Alternatif jika Gmail menolak/mengganti From (tanpa memalsukan identitas):** gunakan penyedia pengiriman yang mendukung DKIM domain sendiri (memerlukan persetujuan karena data/penerima keluar ke pihak ketiga), Google Workspace dengan domain (berbayar, tidak sesuai batasan MVP), atau terima From bertanda “via gmail.com” dengan kebijakan DMARC `p=none` setelah keputusan pemilik. Jangan menambahkan record SPF/DKIM/DMARC secara otomatis.

### FL-OTP02-FIX01 — ACL secret non-elevated

**Gejala:** `Set-Acl` gagal dengan `SeSecurityPrivilege` saat setup dijalankan akun Windows biasa.

**Akar masalah:** `new-otp-key.ps1` membuat `DirectorySecurity` baru, memanggil `SetOwner`, lalu `Set-Acl`. `Set-Acl` menulis seluruh security descriptor—termasuk bagian owner/SACL—sehingga akun non-admin dapat diminta `SeSecurityPrivilege`.

**Perbaikan:** `scripts/lib/secure-local.ps1` hanya membaca dan menulis **DACL** lewat `DirectoryInfo/FileInfo.GetAccessControl(Access)` dan `SetAccessControl`. Tidak ada `Set-Acl`, `SetOwner`, atau penulisan SACL. Owner tidak diubah; `Everyone` dan Administrators tidak ditambahkan; ACE eksplisit tidak dihapus buta. DACL akhir: **pengguna aktif FullControl + SYSTEM Read** (SYSTEM boleh membaca untuk backup/scanning, tidak menulis). Verifikasi menolak setiap ACE Allow untuk SID lain (grup/Administrators/Everyone); kekurangan SYSTEM Read juga menggagalkan proses. Auditing/SACL tidak disentuh. DPAPI CurrentUser tetap dipakai. Penulisan secret bersifat atomik: file sementara + ACL + move/replace; rotasi mempertahankan backup sampai secret baru lolos, dan tidak menghapus key/secret lama saat gagal.

**Catatan error lama:** teks `new-otp-key.ps1:22 — Set-Acl` berasal dari versi **pra-perbaikan**. Pada file saat ini, baris 22 adalah `$rng.GetBytes($key)` dan tidak ada `Set-Acl` di seluruh script. Jika error itu masih muncul, periksa bahwa yang dijalankan adalah file di branch/commit saat ini: `Select-String -Path scripts/new-otp-key.ps1 -Pattern "Set-Acl"` harus kosong, dan commit `new-otp-key.ps1` minimal `81d2e9e`. Jangan jalankan salinan lama dari folder lain.

**Bukti:** `scripts/test-secure-local.ps1` dijalankan pada PowerShell **non-elevated**: DACL-only pada folder/berkas (user FullControl + SYSTEM Read), owner tidak berubah, ACE identitas lain ditolak, DPAPI 32 byte, secret writer roundtrip, penolakan overwrite tanpa `-Force`, rotasi aman, key kedua ditolak. Setup penuh dengan secret dummy juga lulus non-elevated dan `start-server.ps1 -Check` menghasilkan `email_otp:true` tanpa mencetak secret. Berkas secret nyata dipertahankan; tidak ada `*.tmp.*`/`*.bak.*`.

### FL-OTP01T-R — RCA autentikasi pengirim (23 Oktober 2026)

**Bukti yang dilaporkan Product Owner:** envelope asli `gmail.com`; header From `support@facelink.online`; Cloudflare `SPF=pass`, `DMARC=fail (not aligned)`; Gmail tujuan setelah forwarding `SPF=pass`, `DKIM=pass`, `DMARC=pass`; DKIM selector `cf2024-1`; SRS dan ARC terdeteksi; DMARC `p=none`.

**Root cause:** Gmail personal “Send as” memakai envelope `gmail.com` dan menandatangani DKIM dengan `d=gmail.com`. Keduanya tidak aligned dengan From `facelink.online`, sehingga DMARC gagal pada hop asli. Cloudflare Email Routing meneruskan dengan SRS dan menambahkan DKIM selector `cf2024-1` di domain `facelink.online`; hop Gmail terakhir melihat SPF/DKIM yang aligned sehingga DMARC PASS. **DMARC PASS itu berasal dari forwarding Cloudflare, bukan dari Gmail menandatangani domain Facelink.** `p=none` hanya monitoring, bukan penyebab PASS.

**Parser:** `scripts/check-email-headers.ps1` sebelumnya dapat mencampur beberapa header `Authentication-Results` karena memakai `-match` pada gabungan blok. Sekarang blok dipisah per hop, authserv-id ditampilkan, SPF/DKIM/DMARC dan domain per blok dilaporkan, ARC dianalisis terpisah, dan `Received-SPF`/IP mentah tidak dicetak. Regression test `scripts/test-email-headers.ps1` memakai fixture sintetis multi-hop + direct Gmail dan memastikan body/OTP/client-ip tidak bocor.

**DNS read-only:** MX Cloudflare; SPF hanya `include:_spf.mx.cloudflare.net` (Cloudflare IP `104.30.0.0/19`); DMARC `p=none`; DKIM hanya `cf2024-1`; selector Google tidak ada. Tidak ada bukti Gmail personal dapat menghasilkan SPF/DKIM aligned untuk `facelink.online` sebelum forwarding. Custom MAIL FROM/DKIM domain tidak tersedia pada jalur ini.

**Direct control:** satu email non-OTP dikirim langsung ke Gmail `<smtp-account-email>` (ID pengiriman disamarkan), SMTP accepted. Verifikasi header langsung menunggu `.eml`; jangan menyimpulkan hasilnya sebelum file diperiksa.

**Remediation (butuh keputusan, tidak dieksekusi):** (a) terima PASS hasil forwarding Cloudflare + `p=none` sebagai monitoring, dengan risiko hop asli/direct delivery gagal DMARC; (b) gunakan outbound provider yang mendukung DKIM domain sendiri dan/atau MAIL FROM `facelink.online` (free tier memerlukan approval karena data keluar ke pihak ketiga dan perubahan DNS); (c) Google Workspace berbayar (di luar batas MVP); (d) relay mandiri dengan IP bereputasi (tidak realistis untuk MVP). Kebijakan Gmail Send As third-party berakhir Januari 2027; dampak tepat terhadap SMTP submission tidak dinyatakan eksplisit, sehingga migrasi provider harus direncanakan tanpa mengklaim SMTP pasti aman. Abstraksi `Sender` backend memungkinkan penggantian provider tanpa mengubah OTP/UI/session.





## Admin dan alamat resmi

admin@facelink.online = target Administrator permanen; dosen@facelink.online dan mahasiswa@facelink.online = target akun testing. Tidak ada hardcoded role mapping di runtime/migrasi. Sebelum mengundang: verifikasi DB path dan Admin lama, backup SQLite yang konsisten, konfirmasi penguasaan inbox/routing. Admin lama menggunakan endpoint sah POST /users untuk mengundang alamat baru dengan role eksplisit; termasuk Admin baru (bukan bootstrap kedua). Jangan UPDATE email, menggabungkan UUID, memindahkan Google subject, atau membuat user otomatis saat login.

**Batas bukti 23 Oktober 2026:** delivery E2E belum terbukti pada snapshot ini; rincian provisioning/pengiriman per akun tidak dipublikasikan. Permintaan OTP ke alamat yang belum diundang menghasilkan decoy 202 tanpa email. Prosedur sah tanpa SQL/bootstrap kedua: autentikasi Admin lama, lalu `POST /api/v1/users` dengan `{email, display_name, role}` dan Bearer token. Alat lokal `scripts/invite-admin.mjs` meminta OTP Admin lama secara **tersembunyi**, memverifikasi, mengundang, dan logout; OTP/token tidak dicetak. Alat memerlukan backend HTTPS dan `--yes`; gunakan `--dry-run` untuk memeriksa rencana. Aturan kontrak tetap: setelah undangan, akun `invited`, lalu menjadi `active` pada OTP pertama. Tidak ada UI undangan; jangan membuat akun melalui SQL langsung.

Uji pengiriman dan login Admin resmi hingga /me menunjukkan admin active, restore/logout/refresh terbukti. Pertahankan akun/identitas/sesi lama selama transisi; dekomisioning hanya tugas terpisah dengan persetujuan eksplisit dan last-admin protection. Dengan UI Google disabled, endpoint Google dan kredensial existing tetap utuh; OTP pada email lama juga tetap tersedia jika eligible. Rencana transisi tidak memberi izin menyunting database langsung. Belum ada undangan/data nyata yang diubah pada penulisan kontrak ini.

Email OTP bukan phishing-resistant MFA. Sebelum produksi/data sensitif: tambah passkey (disarankan) atau TOTP dengan recovery aman; lindungi akun pengirim Gmail dan inbox/routing Admin dengan MFA, pisahkan mailbox role bila memungkinkan, dan uji pencabutan akses. Satu inbox forwarding yang sama dapat mengendalikan ketiga role: ini risiko pengujian, bukan isolasi akun produksi.

## Mobile

Layout/branding/video/animasi existing dipertahankan. Dua tombol: FaceLink aktif membuka input email/OTP; Google disabled native/accessibility, tidak memiliki handler yang memanggil OAuth. Layar email/OTP memakai design system existing, keyboard email/number-pad, autofill one-time-code, label aksesibel, keyboard avoidance, safe area, tombol back. Tidak menyimpan kode di SecureStore/AsyncStorage atau parameter route/log. Countdown berbasis deadline, dihitung ulang saat foreground; resend hanya setelah retry_after dari server. 429 memakai Retry-After; error jaringan tidak menghapus sesi atau mengirim ulang otomatis. Kode dipertahankan hanya dalam memori layar dan dikosongkan setelah penggunaan/keluar; hasil verifikasi masuk melalui AuthFlow existing untuk SecureStore/role routing/refresh/logout.

## Prasyarat live dan bukti

SMTP/HMAC belum tersedia pada environment proses saat audit FL-OTP01; hanya keberadaan variabel diperiksa, nilainya tidak ditampilkan. Implementasi dan automated tests boleh memakai sender uji terisolasi, tetapi itu bukan bukti Gmail/Cloudflare/native OTP E2E. Pengujian nyata menunggu konfigurasi owner yang aman; jangan pernah menyediakan fallback OTP tetap, print-code, bypass login, atau test identities dalam database pengguna. Hasil tes dan checkpoint dicatat setelah dieksekusi.

## Status implementasi FL-OTP01 (23 Oktober 2026)

**SELESAI pada tingkat kode dan pengujian terisolasi; E2E Gmail/Cloudflare dan Android belum terbukti.** Branch `feat/fl-otp01-email-auth`, bertumpu pada checkpoint FL-ANDROID02F (`fd3785f`).

| Bukti | Hasil |
|---|---|
| `go test -count=1 ./...` | 14 paket uji lulus, termasuk `emailotp`, `smtpemail`, `otpkey`, `migrations` v4 |
| Vitest | **106/106**, 8 berkas |
| `tsc --noEmit`, `expo lint`, `gofmt -l`, `go vet` | Bersih |
| HTTP OTP | HTTPS wajib; 202 identik akun dikenal/tidak/ditangguhkan; role dari DB; expiry/reuse/brute-force; 1 pemenang konsumsi konkuren; 429 + `Retry-After`; SMTP gagal tidak mengaktifkan akun; sesi/refresh/logout/revoke; 0 kode/email/token pada log & audit |
| SMTP TLS | Server SMTP loopback sungguhan: TLS tervalidasi + AUTH setelah TLS; CA asing, hostname salah, STARTTLS hilang, RCPT ditolak semuanya gagal |
| Key | Script PowerShell menghasilkan ciphertext DPAPI CurrentUser 262 B → plaintext 32 B; direktori ACL owner-only; key kedua ditolak. Loader Go menolak key plaintext dan key di dalam repository |
| Runtime smoke | Binary nyata + TLS + DB sementara: migrasi v4; `email_otp:false` → 503; `email_otp:true` → 202 tanpa membocorkan akun; worker menandai `failed` saat port SMTP tertutup; verifier 32 B; log tanpa email/kode/secret |
| Android | **BELUM**: emulator ANR saat startup dengan RAM host 0,26 GB. Layar dan tombol hanya dapat diperiksa di kode/tes, bukan rendering nyata |
| Gmail/Cloudflare | **BELUM**: tidak ada SMTP/App Password pada environment; tidak ada email nyata yang dikirim atau diterima |
| Login OTP nyata | **BELUM**: memerlukan SMTP terotorisasi; tidak ada bypass/kode tetap |

**Masalah terbuka:** batas rate berbasis IP memperlakukan semua pengguna di balik satu NAT sebagai satu bucket (dapat memblokir kelas); jika LAN/proxy ditambahkan, perlu kebijakan IP tepercaya eksplisit, bukan mempercayai `X-Forwarded-For` tanpa konfigurasi. Email OTP bukan MFA tahan phishing. Rotasi key membatalkan challenge yang belum dipakai. Antrean worker in-process hilang saat restart; challenge `pending` tidak di-retry otomatis dan harus diminta ulang. Belum ada pembersihan challenge lama selain opportunistic saat request/verify.

## FL-OTP01E-R2 — Provisioning disetujui, E2E terhambat (24 Oktober 2026)

Baseline `feat/fl-otp01-email-auth`, HEAD `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`, worktree awal bersih. Product Owner menyetujui dua undangan testing melalui Admin resmi `admin@facelink.online`, dan menerima shared inbox untuk pengujian principal backend. Tidak ada izin suspend/hapus, perubahan Admin, autentikasi/UI, SMTP/DNS, push, atau merge.

Rincian baris akun provisioning (alamat/nama/status) tidak dipublikasikan. Hasil provisioning dan batas E2E di bawah tetap merupakan kesimpulan historis, bukan ilustrasi akun sintetis.

Provisioning dan koreksi data testing dilakukan melalui API sah setelah persetujuan Product Owner; tidak memakai SQL atau bootstrap kedua. Koreksi `display_name` tidak mengganti UUID/email/role/status atau membuat akun ulang. Nilai profil dan riwayat autentikasi per akun tidak dipublikasikan. **Gap audit:** handler existing tidak menulis event khusus perubahan nama saja. Helper koreksi sementara dihapus; implementasi aplikasi tidak diubah.

Rincian nama, status, jumlah identitas dan sesi akun Admin tidak dipublikasikan. Provisioning belum membuktikan account linking maupun aktivasi target.

Startup backend existing selalu memanggil migration runner. Product Owner memberi pengecualian startup terjaga: empat versi/checksum diperiksa read-only dan cocok sebelum startup; log menunjukkan `current_version=4`, `applied=0`, `available=4`. Tidak ada migrasi skema baru atau perubahan runner. Konfigurasi SMTP cocok baseline; `start-server.ps1 -Check` lulus. Listener nyata hanya `127.0.0.1:8080`; smoke health HTTPS dengan CA dev dan verifikasi TLS default menghasilkan HTTP 200 dan `TLS_AUTHORIZED=true`.

| Verifikasi | Hasil |
|---|---|
| `go test -count=1 ./internal/auth ./internal/authz ./internal/emailotp ./internal/session ./internal/httpapi` | PASSED, lima paket |
| `go vet` untuk kelima paket tersebut | PASSED |
| Vitest `authFlow`, `emailOtpFlow`, `authApi` | PASSED, 65 tes |
| Vitest `tokenStore`, `authSecurity`, `roleRoutes` | PASSED, 20 tes |
| `npm run typecheck`, `npm run lint` | PASSED |
| Provisioning dan audit undangan | PASSED setelah koreksi nama yang disetujui |
| Android Dosen dan Mahasiswa | BLOCKED: layar kosong; capture ADB biasa timeout 45 detik, capture langsung timeout 20 detik; RAM host 0,17–0,24 GiB |
| Live OTP target, aktivasi, dashboard, SecureStore restore, refresh, logout, RBAC 403 | BLOCKED; tidak digantikan hasil unit test |
| Kebocoran secret runtime target | Belum dapat dinilai: tidak ada sesi/login target; regresi keamanan terisolasi lulus |

Metro dihentikan setelah bundle Android selesai untuk membebaskan memori, tetapi pengambilan surface tetap tidak selesai. Tidak ada OTP Dosen/Mahasiswa diminta atau dimasukkan lewat CLI sebagai pengganti Android. Tidak ada OTP/token/secret ditampilkan atau diambil dari SecureStore. Tidak ada brute force, Face Enrollment, atau bypass TLS.

Risiko tersisa: shared inbox Dosen dengan Admin legacy berarti kontrol inbox bukan independen meskipun principal berbeda; original-hop DMARC tetap tidak aligned; sebagian dokumentasi invitation masih menyebut Google-only; validasi sintaks email pada endpoint invitation lebih longgar daripada request OTP. **Functional three-role NO-GO; sender security NO-GO; merge/release NO-GO.** Lanjutan memerlukan resource emulator memadai, lalu login OTP manual kedua role dan bukti server-side 403 menggunakan sesi role yang sah; jangan mengulang undangan akun existing.

## FL-OTP01E-R4 — HP USB siap untuk testing manual (24 Oktober 2026)

**Status: READY untuk memulai testing manual; Android OTP E2E belum dinyatakan PASS.** Branch/HEAD tetap `feat/fl-otp01-email-auth` / `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`. Perubahan dokumentasi sebelumnya dipertahankan.

- HP TECNO LJ7, Android 16, API 36, ABI arm64-v8a; awalnya unauthorized, lalu terotorisasi oleh pemilik. Tidak memakai emulator dan tidak mencatat serial lengkap dalam laporan.
- APK lokal existing hanya x86_64, sedangkan APK ARM64 lama pada HP belum memiliki CA dev. Karena itu build ulang debug ARM64 diperlukan: `gradlew.bat :app:assembleDebug -PreactNativeArchitectures=arm64-v8a --no-daemon --max-workers=2`, tanpa Expo prebuild atau perubahan source. Build sukses; minSdk 24, targetSdk 36. Signature SHA-256 cocok dengan aplikasi terpasang: `fac61745dc0903786fb9ede62a962b399f7348f0bb6f899b8332667591033b9c`. Pembaruan `adb install -r` sukses, tanpa uninstall atau wipe.
- APK hasil build pada path debug existing memiliki SHA-256 `f4cc2eae08fc95681960e9fbf33808e382125961d6a4304c7b1d2cc60967bf8b`. CA di dalam APK cocok dengan `backend/dev/certs/dev-ca.pem`; manifest/network-security-config debug memercayai CA bundel. Main/release tidak ditambahi CA. Tidak ada pemasangan CA global.
- Empat versi/checksum SQLite diperiksa read-only dan cocok sebelum startup; runner existing melaporkan current_version 4, applied 0, available 4. Backend dan Metro hanya mendengarkan `127.0.0.1`, masing-masing port 8080 dan 8082. Mapping ADB reverse kedua port terverifikasi pada HP fisik yang dipilih eksplisit.
- Metro memakai development client, localhost port 8082, cache dibangun ulang, dan override proses `EXPO_PUBLIC_FACELINK_API_BASE_URL=https://127.0.0.1:8080`. `.env` tidak disunting. Manifest serta JS bundle HTTP 200, host 127.0.0.1:8082, API localhost ter-inline, tidak ada URL HTTPS emulator lama.
- **Bukti Android HTTPS:** halaman diagnostik existing pada HP menampilkan `https://127.0.0.1:8080`, `healthz: ok`, `readyz: ready`, database ok, WAL wal, dan signedOut. Ini request nyata aplikasi lewat USB dengan trust native existing, bukan hasil browser desktop atau bypass TLS.

### Pengecualian latar sementara yang disetujui

Jalur login dengan video sempat menampilkan layar kosong; log app menunjukkan timeout media, sementara diagnostik dapat dirender dan menjangkau backend. Akar masalah video belum diperbaiki. Product Owner secara eksplisit mengizinkan fallback sementara untuk testing: dua MP4 dipindahkan tanpa dihapus ke:

```text
%TEMP%\FaceLink-R4-video-preserved-1792813932286\video1.mp4
%TEMP%\FaceLink-R4-video-preserved-1792813932286\video2.mp4
```

Resolver Metro existing kemudian memilih daftar klip kosong dan ScanBackdrop; tidak ada perubahan komponen UI, source, autentikasi, secret, atau DNS. Setelah Metro direstart, layar login tampil di HP: FaceLink aktif, Google abu-abu/disabled. Penekanan FaceLink membuka form email kosong dengan keyboard; tombol kirim kode belum aktif karena email belum diisi. Agent tidak mengisi email, mengirim OTP, atau login. Cold start dan rendering diperiksa; animasi splash tidak direkam terpisah.

Jangan menghapus folder backup tersebut. Setelah testing manual selesai, pulihkan kedua berkas ke `apps/mobile/assets/video/` dan restart Metro bila ingin kembali memakai video. Pemulihan ini akan mengembalikan jalur video yang masih memiliki temuan rendering; jangan mengklaim bug videonya sudah diperbaiki.

### Handoff

Backend `OtpR4Backend` dan Metro `OtpR4MetroFallback` dibiarkan berjalan untuk pemilik. Dosen dan Mahasiswa READY untuk mulai testing manual, bukan untuk acceptance E2E. Agent berhenti pada form email kosong. Product Owner meminta dan memasukkan OTP sendiri, lalu memberikan hasil dashboard/restore/logout. Verifikasi status akun, audit, rotasi, revocation, dan 403 dilakukan hanya setelah hasil manual diberikan dan dengan mekanisme sah yang tidak mengambil token dari SecureStore/log. Sender security dan merge/release tetap NO-GO; tidak ada perubahan SMTP/DNS, invitation ulang, enrollment, commit, push, atau merge.

## FL-OTP01E-R5 — Verifikasi pascates dan batas closure (24 Oktober 2026)

**Status keseluruhan: PARTIAL.** Bukti login OTP dasar tiga role terverifikasi;
refresh/restore HP Dosen–Mahasiswa dan live RBAC 403 belum terbukti. Bagian R2/R4
di atas adalah snapshot historis sebelum testing manual, bukan status akun kini.

### A. Baseline dan batas perubahan

- Branch `feat/fl-otp01-email-auth`; HEAD persis
  `87bac5e96602e9ff5568069d1c6b1bd1d96913b7`.
- Worktree awal hanya memuat dokumentasi `PROJECT_STATE.md`,
  `docs/EMAIL_OTP.md`, dan `apps/mobile/assets/video/BACA-SAYA.md`; semuanya
  dipertahankan. Tidak ada perubahan staged atau source/auth/backend.
- R5 hanya membaca database existing dan buffer log aplikasi. Tidak ada OTP
  diminta, token diambil dari HP/SecureStore, akun dibuat ulang, bootstrap,
  perubahan role/identitas, startup/migrasi baru, atau perubahan SMTP/DNS.
- Kedua MP4 aktif dan backup `%TEMP%\FaceLink-R4-video-preserved-1792813932286`
  masih ada, identik SHA-256, dan MP4 aktif tetap ignored Git. Ukuran
  `video1.mp4` 2080600 byte, `video2.mp4` 1819238 byte.
  Normalnya video dikonfirmasi pemilik; RCA blank intermiten tetap PARTIAL.

### B. Akun dan audit autentikasi — PASSED

Audit read-only database dan autentikasi menghasilkan kesimpulan **PASSED** untuk transisi undangan → aktivasi dan pemisahan principal. Baris akun, UUID, status per akun, waktu autentikasi, rincian hubungan identitas dan histori profil tidak dipublikasikan. Tidak ada akun pengganti sintetis yang diklaim sebagai hasil observasi.

Kesimpulan audit tidak menunjukkan account-link, penggabungan principal, atau bootstrap kedua. Shared inbox tetap risiko kontrol inbox, bukan principal yang sama. Dashboard berbeda di HP TECNO LJ7/Android 16 adalah konfirmasi manual Product Owner, bukan pengujian ulang oleh agent.

### C. Session security — klasifikasi bukti

| Aspek | Database/audit existing | Manual Android | Regresi terisolasi / batas |
|---|---|---|---|
| Issuance | PASSED: issuance cocok audit sukses; rincian jumlah sesi per akun tidak dipublikasikan | PO mengonfirmasi dashboard kedua target | PASSED |
| Logout/revocation | PASSED: pencabutan sesi dan audit logout cocok; rincian status sesi per akun tidak dipublikasikan | Interaksi tombol/layar pascalogout target tidak dikonfirmasi terpisah | PASSED untuk penolakan akses/refresh sesudah logout pada tes |
| Refresh rotation | PASSED historis untuk rotasi; target live belum memiliki bukti refresh (rincian history akun dan timestamp tidak dipublikasikan) | Belum terbukti pada HP Dosen/Mahasiswa | PASSED pada tes, target live PARTIAL |
| Restore | Audit/database tidak mengidentifikasi force-stop/relaunch atau lokasi penyimpanan token | Belum ada bukti restore HP target | PASSED pada tes mobile dengan storage tiruan; live PARTIAL |
| Token plaintext di log | Nol key metadata audit terlarang pada scan struktur | Buffer logcat retained khusus UID FaceLink: 194528 byte, nol pola JWT, field token bernilai panjang, atau Bearer credential | PASSED pada cakupan scan/regresi; bukan jaminan seluruh log historis bersih |

`revoked_at` + audit membuktikan logout server terjadi; R5 tidak mencoba token
live yang dicabut dan tidak mengklaim HTTP rejection live target sudah diuji.
Audit refresh saja tidak membuktikan Android SecureStore restore. TTL akses yang dicatat dalam pemeriksaan adalah 60 menit.

### D. Server-side RBAC — PASSED terisolasi, PARTIAL live

Matrix acuan `docs/AUTHORIZATION.md`; middleware
`backend/internal/httpapi/auth_middleware.go` memuat user terbaru dari database
dan menolak status non-active. Principal mengambil role database, bukan email,
role yang dikirim saat login, atau klaim token. Admin boleh menetapkan role
target lewat API invitation terotorisasi; ini bukan self-asserted role.

- `TestDosenDitolakPadaEndpointAdministratif` dan
  `TestMahasiswaDitolakMembacaDaftarAkun` membuktikan HTTP 403 memakai sesi
  principal terpisah pada database sementara.
- `TestKebijakanCakupanKelas` menolak Mahasiswa pada operasi kelas Dosen.
  Belum ada tes HTTP khusus Mahasiswa → `GET /classes/{class_id}/dosen`.
- `TestEmailOTPRoleSelaluDariDatabase` membalik role email bernama admin/dosen
  dan membuktikan hasil tetap mengikuti DB. Tes perubahan role/status, lifecycle
  OTP, refresh/reuse/revocation juga lulus.
- HTTP live 403 ketiga arah dengan akun target belum dilakukan. Tidak memakai
  dashboard redirect sebagai bukti RBAC server; endpoint dalam matrix yang
  belum diimplementasikan tidak diklaim teruji.

### E. Fixture biometrik dan backlog

**FAILED bila dibaca sebagai status live:** `app/(app)/mahasiswa.tsx:50-55`
menampilkan literal `Persetujuan / Aktif / biometrik`; bukan hasil consent API.
Duplikat fixture ada di `src/features/dashboard/mockData.ts:108-112`; tombol
consent hanya memberi alert bahwa integrasi backend belum tersedia. Database
memiliki **0 people, 0 consent_records, 0 face_enrollments**. Login OTP bukan
persetujuan biometrik dan tidak mengizinkan pengambilan wajah. UI tidak diubah
sesuai batas R5; gap wajib masuk readiness FL-ENR00 sebelum pemakaian live.

Enam residual tetap terbuka di `docs/BACKLOG.md` bagian R5: original-hop Gmail
DMARC tidak aligned; PATCH nama tanpa audit khusus; referensi invitation
Google-only; validasi invitation lebih longgar daripada OTP; distribusi MP4
tidak reproducible pada fresh clone; RCA blank intermiten belum definitif.
Fresh clone tanpa kedua MP4 memilih `video-clips.none.js`/ScanBackdrop menurut
resolver existing: aplikasi punya fallback, tetapi tampilan video tidak identik.
Fresh clone tidak dijalankan pada R5. Tidak mengubah ignore atau commit binary.

### F. Regresi yang dijalankan dan gate

| Perintah | Hasil nyata R5 |
|---|---|
| `go test -count=1 ./internal/auth ./internal/authz ./internal/emailotp ./internal/session ./internal/httpapi` (dari `backend`) | PASSED, lima paket |
| `npx vitest run tests/authFlow.test.ts tests/emailOtpFlow.test.ts tests/authApi.test.ts tests/tokenStore.test.ts tests/authSecurity.test.ts tests/roleRoutes.test.ts` (dari `apps/mobile`) | PASSED, enam berkas / 85 tes |

Tes menggunakan DB/principal/clock/storage terisolasi; tidak mengirim email
nyata. Tidak menjalankan ulang build APK, lint/typecheck, seluruh suite atau
pengujian SMTP karena source tidak diubah.

- **GO menutup bukti OTP login dasar tiga role:** verifikasi email, aktivasi,
  role dari DB, sesi terbit, dashboard target berdasarkan PO, logout server.
- **NO-GO closure autentikasi lengkap:** restore/refresh HP target serta live
  server-side 403 masih PARTIAL. Tidak menaikkan temuan itu menjadi PASS.
- **GO menuju FL-ENR00 hanya untuk review kesiapan/kontrak. NO-GO implementasi
  atau enrollment biometrik nyata** sebelum scope, consent eksplisit, integrasi
  status live, serta gate otorisasi disetujui/dibuktikan.
- Sender security sign-off dan merge/release tetap NO-GO. Tidak ada commit,
  push/merge, perubahan akun/source/UI/secret/SMTP/DNS pada R5.

### Uji manual minimum berikutnya — tidak dijalankan otomatis

1. Pada sesi berikutnya yang PO mulai sendiri untuk masing-masing target,
   force-stop lalu buka ulang sebelum TTL akses: dashboard harus kembali dengan
   role sama tanpa OTP baru. Catat hasil UI dan korelasikan sesi/audit tanpa
   membaca SecureStore; ini bukti restore native yang masih kurang.
2. Setelah TTL akses 60 menit lewat secara normal (jangan ubah clock/TTL),
   buka ulang untuk memicu pemulihan/refresh. Periksa audit refresh serta history
   rotasi bertambah, identitas sesi tetap, role benar; lalu logout dan catat
   layar masuk serta revocation server. Jangan meminta/mengirim OTP di chat.
3. Uji API terotorisasi memakai client uji sah dengan sesi terpisah di memori,
   bukan token hasil ekstraksi HP: Dosen → Admin 403, Mahasiswa → Admin 403,
   Mahasiswa → endpoint Dosen implemented 403; sertakan kontrol positif. Jangan
   memakai endpoint yang belum ada atau menganggap 404 sebagai bukti 403.
   Siapkan sesi tersebut hanya pada putaran pengujian yang disetujui PO.

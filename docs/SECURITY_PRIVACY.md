# Keamanan, Privasi, dan Persetujuan Biometrik

**Status:** Persyaratan minimal untuk MVP akademik. Dokumen ini adalah panduan engineering, **bukan nasihat hukum**. Untuk penggunaan institusional, konsultasikan pengelolaan data dan kebijakan yang berlaku.

## 1. Mengapa penting

Data wajah dan template biometrik bersifat sangat sensitif. Di Indonesia, **UU Nomor 27 Tahun 2022 tentang Pelindungan Data Pribadi** menggolongkan data biometrik sebagai data pribadi bersifat spesifik. Walaupun project hanya tugas kampus, peserta tetap perlu mendapat penjelasan, pilihan yang adil, dan perlindungan data.

## 2. Klasifikasi data

| Data | Risiko | Aturan awal |
|---|---|---|
| Gambar wajah mentah | Tinggi | Diproses sementara; tidak disimpan otomatis |
| Embedding/template wajah | Tinggi | Lokal, dienkripsi, akses terbatas, bisa dihapus |
| Nama/kode peserta | Pribadi | Minimalkan; gunakan kode fiktif untuk konten publik |
| Kehadiran dan jadwal | Pribadi | Operator yang sah saja; retensi dibatasi |
| Login, token perangkat, kunci enkripsi | Rahasia | Jangan log/commit; simpan terproteksi |
| Log kesehatan perangkat | Operasional | Tanpa JPEG/embedding dan tanpa identitas yang tidak perlu |

## 3. Persetujuan dan alternatif

- Sebelum enrollment, jelaskan: tujuan absensi, data yang diambil, lokasi pemrosesan, siapa yang bisa mengakses, masa simpan, cara pencabutan izin, dan alternatif manual.
- Peroleh tindakan persetujuan yang **jelas, sukarela, dan terdokumentasi**. Jangan menganggap kehadiran di ruang kelas sebagai persetujuan biometrik.
- Peserta yang menolak harus tetap dapat hadir melalui absensi manual tanpa perlakuan merugikan.
- Setelah izin dicabut, hentikan proses biometrik orang tersebut dan hapus template dari disk, database, indeks, cache, serta backup bila kebijakan dan mekanismenya mengharuskan; catat penyelesaiannya.
- Pemrosesan dan penyimpanan data untuk jurnal atau publikasi akademik memerlukan kajian dan izin tersendiri; jangan mengambil otomatis dari sistem produk.

## 4. Ancaman utama dan mitigasi

| Ancaman | Mitigasi awal |
|---|---|
| Akses operator tidak sah | Login, hash kata sandi aman, otorisasi tiap endpoint, pembatasan percobaan |
| Perangkat palsu mengirim absensi | Kredensial khusus device, daftar perangkat diizinkan, rotasi/revokasi |
| Request JPEG berbahaya | Validasi MIME dan decode, batas ukuran/laju, timeout, perlindungan memori |
| Foto/video untuk titip absen | Deteksi presentasi dasar yang diuji, pengawasan operator, fallback manual; jangan klaim kebal |
| Bocor embedding/foto | Penyimpanan lokal terlindungi, enkripsi template AES-256-GCM per baris (D-022), tidak ada telemetri eksternal |
| Pengiriman melalui Wi-Fi bersama | Jaringan lab terisolasi; untuk pemakaian riil memerlukan transport terenkripsi |
| Data pribadi ter-commit | `.gitignore`, pemeriksaan sebelum commit, fixtures sintetis, pembatalan dan penanganan kebocoran |
| Duplikasi request atau perubahan sesi | Constraint database, transaksi, idempotensi, validasi ulang status |
| Pemrosesan setelah izin dicabut | Cabut akses template, bersihkan cache, uji penghapusan |

## 5. Penyimpanan, enkripsi, dan retensi

- Tentukan umur simpan **sebelum** pengambilan data asli. Prinsip awal: simpan seminimal mungkin, hapus sesegera mungkin setelah presentasi dan evaluasi selesai.
- Template biometrik disimpan **terenkripsi AES-256-GCM per baris** (D-022): nonce acak kriptografis per baris, tag autentikasi diverifikasi sebelum dekripsi, dan dekripsi yang gagal **tidak** diganti dengan tebakan. Satu-satunya lokasi penyimpanan biometrik adalah tabel `face_templates` (`docs/DATA_MODEL.md` §4.12).
- Kunci enkripsi **tidak** boleh berada di dalam database, di dalam commit, atau di berkas konfigurasi yang ikut ter-commit. Bentuk penyimpanannya sudah final, tetapi **lokasi, rotasi, dan pemulihan kunci belum ditetapkan —** Q-16 **masih terbuka dan menghambat implementasi penyimpanan biometrik.** Jangan menuliskan atau mengasumsikan kebijakan kunci sebelum pemilik project menetapkannya.
- Enkripsi template bukan enkripsi seluruh berkas database: rekap kehadiran dan nama peserta tetap terbaca bagi siapa pun yang memegang berkas SQLite. Jangan mengklaim sebaliknya.
- Cadangan data, jika benar-benar dibutuhkan, harus tunduk pada retensi/penghapusan yang sama; jangan menyisakan template yang sudah dicabut di backup tanpa kebijakan.
- Foto JPEG pemindaian rutin tidak disimpan secara default. Cukup simpan hasil status dan metadata seperlunya.
- Catatan audit menunjukkan tindakan tanpa merekam informasi wajah atau nilai embedding.

## 6. Pengendalian akses dan transportasi

- Operator hanya mendapat hak sesuai kebutuhannya; token perangkat tidak boleh digunakan untuk endpoint admin.
- Endpoint AI Engine hanya untuk backend lokal; batasi ke loopback `127.0.0.1`/IPC dengan kontrak internal `docs/API_CONTRACT.md` §6, dan jangan pernah membukanya pada jaringan kelas.
- Hanya backend Go yang boleh menulis database dan menetapkan kehadiran (D-027); AI Engine tidak memegang kredensial database, tidak membaca tabel biometrik, dan tidak boleh menyimpan embedding di disk.
- Hindari hardcode IP, password, kunci, atau token dalam firmware dan aplikasi. Rahasia perangkat bisa diprovisikan saat demo dan diganti bila bocor.
- Semua penyimpanan, koneksi, dan alur rilis yang terkait biometrik harus diuji sebelum penggunaan orang sungguhan.

## 7. Tindakan ketika gagal atau terjadi insiden

- Jika AI ragu, hentikan check-in otomatis; operator membantu verifikasi manual.
- Jika server atau jaringan mati, jangan membuat catatan kehadiran sementara yang seolah-olah sudah sah.
- Jika data berpotensi bocor, hentikan pemrosesan, amankan kredensial, cari jangkauan insiden dan lakukan penanganan sesuai ketentuan yang berlaku.
- Jangan menyembunyikan keterbatasan akurasi atau pengujian dengan kacamata dalam presentasi.

## 8. Syarat keamanan sebelum demo

- [ ] Dokumen persetujuan tersedia dan dipahami peserta.
- [ ] Jalur absensi manual berfungsi.
- [ ] Data pribadi/bio tidak masuk Git, log, atau LLM API.
- [ ] Token operator dan perangkat memiliki izin berbeda.
- [ ] Foto mentah tidak ditahan tanpa alasan/izin.
- [ ] Penghapusan template teruji pada disk dan cache.
- [ ] Gagal koneksi, wajah asing, dan hasil ambigu tidak memproduksi check-in.
- [ ] Pembatasan keamanan kamera/jaringan lokal dan kekurangan anti-spoofing dijelaskan.
- [ ] Lisensi model pihak ketiga ditinjau.

## 9. Keselarasan dengan keputusan dan kontrak lain

| Dokumen | Bagian yang mengikat |
|---|---|
| `docs/DECISIONS.md` | D-015 (template terenkripsi, tanpa foto mentah), D-022 (AES-256-GCM per baris), D-023 (kontrak kamera JPEG), D-025 (lisensi bobot model), D-027 (Go pemilik keputusan dan database), D-029 (liveness terpisah) |
| `docs/DATA_MODEL.md` | §4.12 `face_templates` (kolom, AAD, perilaku dekripsi gagal, penghapusan keras), §5.3–§5.4, §6 |
| `docs/API_CONTRACT.md` | §6.7 (penanganan gambar), §6.8 (pemisahan tanggung jawab), §7 (status kegagalan wajah) |
| `docs/OPEN_QUESTIONS.md` | Q-16 (pengelolaan kunci — **blocker** implementasi penyimpanan biometrik), Q-07 (angka ambang kalibrasi), Q-18 (batas laju/ukuran gambar) |

Tidak ada angka ambang kecocokan maupun kebijakan pengelolaan kunci yang ditetapkan di dokumen ini; keduanya tetap tugas fase terkait.

Lihat pula `TEST_PLAN.md`, `FACE_ENGINE.md`, dan `REFERENCES.md`.

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
| Bocor embedding/foto | Penyimpanan lokal terlindungi, enkripsi template, tidak ada telemetri eksternal |
| Pengiriman melalui Wi-Fi bersama | Jaringan lab terisolasi; untuk pemakaian riil memerlukan transport terenkripsi |
| Data pribadi ter-commit | `.gitignore`, pemeriksaan sebelum commit, fixtures sintetis, pembatalan dan penanganan kebocoran |
| Duplikasi request atau perubahan sesi | Constraint database, transaksi, idempotensi, validasi ulang status |
| Pemrosesan setelah izin dicabut | Cabut akses template, bersihkan cache, uji penghapusan |

## 5. Penyimpanan, enkripsi, dan retensi

- Tentukan umur simpan **sebelum** pengambilan data asli. Prinsip awal: simpan seminimal mungkin, hapus sesegera mungkin setelah presentasi dan evaluasi selesai.
- Gunakan enkripsi template di penyimpanan; kuncinya **tidak** boleh disimpan dalam commit atau satu file bersama ciphertext tanpa perlindungan.
- Cadangan data, jika benar-benar dibutuhkan, harus tunduk pada retensi/penghapusan yang sama; jangan menyisakan template yang sudah dicabut di backup tanpa kebijakan.
- Foto JPEG pemindaian rutin tidak disimpan secara default. Cukup simpan hasil status dan metadata seperlunya.
- Catatan audit menunjukkan tindakan tanpa merekam informasi wajah atau nilai embedding.

## 6. Pengendalian akses dan transportasi

- Operator hanya mendapat hak sesuai kebutuhannya; token perangkat tidak boleh digunakan untuk endpoint admin.
- Endpoint worker AI hanya untuk backend lokal; batasi loopback/IPC dan kredensial internal jika perlu.
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

Lihat pula `TEST_PLAN.md`, `FACE_ENGINE.md`, dan `REFERENCES.md`.

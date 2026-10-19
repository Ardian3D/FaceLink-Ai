# Laporan Kalibrasi FL-CV02A — Pengenalan Wajah YuNet + SFace

**Status:** **HARNESS DAN ANALISIS SELESAI — PENGUJIAN LAPANGAN BELUM DILAKUKAN (BLOCKED)**
**Tanggal:** 20 Oktober 2026
**Cabang:** `research/fl-cv02a-face-calibration`, dari `origin/main` (`a6f4cd2`, merge PR #17 — FL-CV01)
**Lingkup:** pengukuran dan alat evaluasi lokal. **Bukan** fitur absensi, bukan endpoint enrollment, bukan penetapan ambang produksi.

> **Tidak ada satu pun peserta manusia yang diukur untuk dokumen ini.**
> Seluruh angka pada bagian 6 berasal dari vektor sintetis dan pengujian unit.
> Angka tersebut membuktikan aritmetika alat, **bukan** performa pengenalan wajah.
> Q-07 tetap **TERBUKA**.

## 1. Ringkasan

| Pertanyaan | Jawaban saat ini |
|---|---|
| Apakah harness pengukuran siap dipakai? | **Ya** — `python -m face_engine.calibrate` menjalankan persetujuan → enrollment → kueri per kondisi → analisis → pembersihan memori. |
| Apakah ambang 0,363 sudah terukur pada peserta FaceLink? | **Belum.** Angka itu tetap baseline dokumentasi OpenCV Zoo. |
| Berapa false match / false rejection sebenarnya? | **Belum ada data manusia.** Nol pengujian lapangan. |
| Apakah Q-07 dapat ditutup? | **Tidak.** Menutupnya menuntut sesi dengan peserta sukarela dan jumlah percobaan yang dicatat. |
| Apakah sistem tahan spoofing? | **Tidak.** Tidak ada liveness (bagian 8). |

## 2. Perubahan dari FL-CV01

Kode runtime FL-CV01 (kamera, detektor, embedder, matcher, kualitas, galeri)
**tidak diubah perilakunya**. Yang ditambahkan adalah jalur pengukuran terpisah:

| Berkas | Peran |
|---|---|
| `face_engine/evaluation.py` | Aritmetika evaluasi: skor per identitas, ambang + margin, false match/rejection, distribusi, margin top-1/top-2, peringkat 1:N, korelasi kualitas |
| `face_engine/calibration.py` | Harness sesi: penjelasan persetujuan, label anonim, penolakan tanpa persetujuan, pencabutan, pembersihan memori |
| `face_engine/calibrate.py` | Alat lapangan: pengambilan sampel berkamera + laporan stdout |
| `tests/test_evaluation.py` | Pengujian aritmetika dan kesetaraan aturan dengan `matcher.cocokkan()` |
| `tests/test_calibration.py` | Pengujian batas persetujuan dan pembersihan |
| `tests/test_calibrate_report.py` | Pengujian bahwa laporan mencetak setiap bagian dan hitungan yang benar |

Keputusan penting: **aturan keputusan tidak diduplikasi**. `evaluation.status_uji()`
meniru `matcher.cocokkan()` dan kesetaraannya diuji pada vektor acak. Tanpa itu,
laporan kalibrasi dapat menyimpang dari perilaku runtime tanpa terlihat.

## 3. Protokol sesi lapangan

Perintah:

```powershell
cd services/face-engine
.venv\Scripts\python.exe -m face_engine.calibrate --peserta 5 --sampel 5 --kueri 3
```

Urutan yang dipaksakan alat:

1. Teks persetujuan dibacakan; setiap peserta mengetik `ya`. Peserta yang menolak
   tetap mendapat label anonim tetapi tidak diambil sampelnya.
2. Enrollment 5 sampel per peserta, hanya frame yang lolos `quality.nilai_frame()`.
3. Kueri 3 per peserta untuk kondisi `terang`, `redup`, dan `miring`.
4. Peserta berkacamata menambah kondisi `kacamata`; setelah itu diminta melepas
   kacamata untuk kondisi `tanpa-kacamata`.
5. Peserta yang **menyetujui tetapi tidak dienroll** dipakai sebagai wajah tak
   terdaftar (`label_benar=None`). Tidak ada orang lewat yang direkam.
6. Laporan dicetak ke stdout; seluruh vektor dibuang saat sesi berakhir.

Tombol: `spasi` ambil, `n` lewati, `q` batal.

## 4. Metrik yang dilaporkan

| Metrik | Definisi yang dipakai kode |
|---|---|
| Genuine similarity | Skor identitas **benar**, bukan skor kandidat teratas |
| Impostor similarity | Seluruh skor identitas lain pada tiap percobaan; untuk wajah tak dikenal, seluruh galeri |
| False rejection | Genuine dengan skor teratas < ambang |
| False match | Genuine yang lolos ambang tetapi kandidat teratas bukan identitas benar |
| Unknown rejection | Wajah `label_benar=None` yang **tidak** menghasilkan `cocok` |
| Ambigu | Dua kandidat ≥ ambang dengan margin < batas → `perlu_verifikasi` |
| Margin top-1/top-2 | Selisih dua skor teratas, hanya untuk galeri ≥ 2 identitas |
| Peringkat 1:N | Posisi identitas benar pada daftar skor (top-1, top-2, median) |
| Latensi | Deteksi, embedding, dan total per kueri (min/median/maks) |
| Pengaruh kualitas | Korelasi Spearman ketajaman/kecerahan vs skor benar, plus tiga kelompok kualitas |
| Penolakan kualitas | Episode penolakan per kode (`NO_FACE`, `MULTIPLE_FACES`, `LOW_QUALITY`, `FACE_TOO_SMALL`) |

Ambang yang dibandingkan: **0,25 / 0,30 / 0,363 / 0,40 / 0,45 / 0,50 / 0,55 / 0,60**.
Margin bawaan **0,05**. 0,363 tetap baseline dokumentasi, bukan hasil ukur kita.

Data dipisah menjadi bagian kalibrasi dan evaluasi **per kondisi**. Pada jumlah
sampel kecil pemisahan ini tidak menghasilkan estimasi yang bermakna; laporan
mencetak keterbatasan itu, bukan menyembunyikannya.

### Aturan pemilihan ambang

`pilih_ambang()` hanya memakai bagian kalibrasi dan hanya menerima ambang yang:

1. FRR ≤ 0,10,
2. tidak menerima satu pun wajah tak dikenal,
3. tidak menghasilkan satu pun false match.

Bila tidak ada, hasilnya `None` dan laporan menyatakan data tidak layak.
Ambang terendah yang lolos adalah **kandidat**, bukan ambang produksi.

## 5. Privasi dan keamanan yang ditegakkan kode

- Enrollment dan embedding hanya di RAM; tidak ada `imwrite`, `VideoWriter`,
  `np.save`, atau serialisasi di paket `face_engine`.
- `SesiKalibrasi` tidak memiliki metode penyimpanan; ini diuji oleh
  `test_harness_tidak_punya_metode_penyimpanan`.
- Label selalu `PERSON-00N`; nama, NIM, dan inisial tidak pernah masuk objek sesi.
- Persetujuan wajib sebelum kueri; `tambah_kueri` melempar `GalatEnrollment` bila
  ada peserta yang belum ditanya.
- **Menolak dibedakan dari belum ditanya.** Peserta yang menolak dicatat lewat
  `tolak_persetujuan()`: label tetap ada (urutan stabil, jumlah peserta jujur),
  tidak ada sampel yang diambil, dan sesi peserta lain **tidak** terblokir.
  Sebelum perbaikan ini, satu penolakan membuat seluruh sesi gagal dengan
  `GalatEnrollment` saat peserta pertama mengambil sampel — perilaku yang sudah
  direproduksi sebelum ditambal.
- Pencabutan persetujuan membuang peserta **dan** sampelnya seketika.
- `akhiri_sesi()` mengosongkan vektor; rekap hanya berisi hitungan dan label.
- Penolakan kualitas hanya menyimpan angka, bukan potongan gambar.

## 6. Bukti pengujian yang benar-benar dijalankan

Perintah:

```powershell
cd services/face-engine
.venv\Scripts\python.exe -m pytest
```

Hasil nyata pada 20 Oktober 2026:

```text
102 passed, 1 skipped in 1.23s
```

Yang dilewati: `tests/test_models.py:122` — gambar acak tidak memuat wajah,
sehingga bentuk baris YuNet tidak dapat diperiksa pada run tersebut. Itu
pelewatan yang disengaja, bukan kegagalan.

Cakupan tambahan FL-CV02A:

- kesetaraan `status_uji()` dengan `matcher.cocokkan()` pada vektor acak
  (20 kueri × 4 ambang × 3 margin);
- batas ambang tepat (`>=` diterima) dan di atasnya (ditolak);
- margin kecil → ambigu, margin besar → kandidat teratas;
- wajah tak dikenal yang ambigu **tidak** dihitung sebagai penerimaan;
- galeri kosong, satu orang, dan beberapa orang;
- input kosong/NaN/bernorma nol;
- pemilihan ambang menolak kandidat yang menghasilkan false match;
- distribusi genuine/impostor/margin dan peringkat 1:N;
- korelasi kualitas dan kelompok kualitas;
- laporan memuat seluruh bagian dan hitungan, termasuk penolakan kualitas;
- pembersihan galeri dan sesi saat program berakhir;
- penolakan eksplisit: tidak memblokir peserta lain, tidak menghapus peserta
  lain, menolak label asing, dan menolak kueri untuk peserta yang menolak.

Smoke test alat:

```text
python -m face_engine.calibrate --help        → daftar argumen benar
python -m face_engine.calibrate --peserta 1   → exit 2, menolak < 2 peserta
python -m face_engine.calibrate --peserta 2   → meminta persetujuan lewat stdin
```

## 7. Hasil lapangan

| Kondisi | Jumlah peserta | Jumlah percobaan | Hasil |
|---|---|---|---|
| Terang / redup / miring | 0 | 0 | **BLOCKED — belum ada peserta** |
| Kacamata / tanpa kacamata | 0 | 0 | **BLOCKED** |
| Wajah tak terdaftar | 0 | 0 | **BLOCKED** |
| Tanpa wajah / dua wajah / buram | 0 | 0 | **BLOCKED** (alat menghitungnya, belum ada sesi) |

**Tidak ada angka false match, false rejection, atau distribusi skor manusia
yang boleh dikutip dari dokumen ini.** Angka pengukuran satu pengembang pada
FL-CV01 (satu kamera, satu orang, satu foto publik) juga **bukan** kalibrasi.

## 8. Keterbatasan dan risiko yang masih terbuka

- **Tidak ada liveness.** Foto cetak, layar ponsel, atau video dapat terdeteksi
  sebagai wajah dan, bila identitasnya terdaftar, menghasilkan skor tinggi.
  Pengujian ini **tidak boleh** disebut uji anti-spoofing.
- **Ukuran sampel.** Rencana lima peserta hanya bukti fungsi prototipe; ia tidak
  menghasilkan laju kesalahan yang stabil dan tidak dapat diekstrapolasi.
- **Satu kamera, satu ruangan.** Ambang ketajaman (20) dan kecerahan (40–220)
  diukur pada Integrated Camera 1280×720 dan bukan angka universal.
- **Kacamata** baru dijadwalkan sebagai kondisi; belum ada data yang membuktikan
  toleransi pantulan lensa.
- **Ambang belum dikalibrasi.** 0,363 berasal dari dokumentasi OpenCV Zoo.
- **Margin bukan jaminan.** Margin membantu menolak kandidat berdekatan, tetapi
  dua orang yang mirip tetap dapat melewati ambang.
- **Data sintetis tidak menggantikan manusia.** Semua angka pada bagian 6 hanya
  membuktikan alat menghitung dengan benar.

## 9. Rekomendasi pengujian lanjutan (Q-07)

1. Jalankan sesi dengan minimal lima peserta sukarela yang menyetujui, di satu
   ruangan, dengan satu kamera yang sama.
2. Catat jumlah percobaan per kondisi, jumlah false match, jumlah false
   rejection, dan jumlah wajah tak dikenal yang diterima — bukan hanya persen.
3. Bandingkan kandidat ambang pada data kalibrasi, lalu terapkan pada data
   evaluasi yang belum dipakai memilihnya.
4. Ulangi dengan peserta berkacamata; jangan mengubah ambang berdasarkan satu
   sesi.
5. Simpan hanya ringkasan agregat; jangan simpan foto, video, atau embedding.
6. Sebelum ada pengujian dengan foto/layar, nyatakan protokol spoofing dan
   keterbatasannya; jangan mengklaim keamanan.

**Q-07 tetap terbuka sampai langkah 1–4 dijalankan dan hasilnya dicatat.**

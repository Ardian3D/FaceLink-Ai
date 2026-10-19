# FaceLink AI — Mesin Wajah Lokal (Prototype FL-CV01 + FL-CV02A)

Prototype **deteksi dan pengenalan wajah lokal** memakai webcam laptop Windows.
Tujuan FL-CV01 hanya membuktikan kemampuan YuNet + SFace bekerja di mesin
pengembang; ia **bukan** fitur absensi dan **tidak** menyimpan data biometrik.
**FL-CV02A** menambahkan alat pengukuran kalibrasi terpisah
(`python -m face_engine.calibrate`) dan modul evaluasi tanpa kamera. Alat itu
**belum dijalankan dengan peserta manusia**; laporan dan batas klaimnya ada di
[`docs/CALIBRATION_FL_CV02A.md`](../../docs/CALIBRATION_FL_CV02A.md).

> **Batas yang tidak boleh dilanggar prototype ini**
>
> - Tidak ada foto, video, atau template biometrik yang ditulis ke disk. Saat
>   program ditutup, seluruh enrollment hilang.
> - Tidak ada database, endpoint, atau unggahan ke layanan eksternal.
> - Tidak ada verifikasi liveness. Prototype ini **tidak** tahan terhadap
>   spoofing memakai foto atau video (D-029).
> - Ambang kecocokan di sini adalah **ambang uji**, bukan angka produksi.
>   Q-07 tetap terbuka sampai diukur pada peserta uji.

## Kebutuhan

| Komponen | Versi yang diuji |
|---|---|
| Windows | 10.0.26200 |
| Python | 3.12.14 |
| `opencv-python` | 4.12.0.88 |
| `numpy` | 2.2.6 |
| `pytest` | 8.4.2 |

Webcam yang diuji: **Integrated Camera** 1280×720.

Python 3.12 tidak perlu dipasang global. Repositori memakai `uv` untuk membuat
virtual environment lokal, tetapi venv Python biasa juga bekerja.

## Pemasangan

```powershell
cd services/face-engine

# Virtual environment lokal (diabaikan Git).
uv venv --python 3.12 .venv
uv pip install --python .venv\Scripts\python.exe -r requirements.txt

# Unduh model + verifikasi SHA-256.
.venv\Scripts\python.exe scripts\download_models.py
```

Tanpa `uv`:

```powershell
py -3.12 -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
.venv\Scripts\python.exe scripts\download_models.py
```

Perintah `download_models.py` menolak berkas yang checksum-nya berbeda. Bila
berkas di hulu berubah, itu **kegagalan yang disengaja**: angka di
`face_engine/config.py` harus diperbarui secara sadar, bukan diterima diam-diam.

## Model dan lisensi

| Model | Berkas | Ukuran | Lisensi | Sumber |
|---|---|---|---|---|
| YuNet (deteksi + 5 landmark) | `face_detection_yunet_2023mar.onnx` | 232 KB | **MIT** | [OpenCV Zoo](https://github.com/opencv/opencv_zoo/tree/main/models/face_detection_yunet) |
| SFace (pengenalan) | `face_recognition_sface_2021dec.onnx` | 36,9 MB | **Apache-2.0** | [OpenCV Zoo](https://github.com/opencv/opencv_zoo/tree/main/models/face_recognition_sface) |

SHA-256 yang diverifikasi pada 19 Oktober 2026:

```
8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4  face_detection_yunet_2023mar.onnx
0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79  face_recognition_sface_2021dec.onnx
```

Berkas bobot **tidak** di-commit (`.gitignore`: `models/`, `*.onnx`). Yang masuk
repositori hanya skrip unduh, checksum, dan catatan lisensi di atas. Keputusan
model ada pada D-012 (YuNet + SFace) dan D-025 (menolak bobot berlisensi
non-permisif, mis. InsightFace/ArcFace).

## Menjalankan

```powershell
.venv\Scripts\python.exe -m face_engine --cek          # periksa kamera + model saja
.venv\Scripts\python.exe -m face_engine --mode detect  # kotak + landmark
.venv\Scripts\python.exe -m face_engine --mode enroll  # kumpulkan sampel
.venv\Scripts\python.exe -m face_engine --mode recognize
```

Tombol di jendela:

| Tombol | Fungsi |
|---|---|
| `D` | mode deteksi (kotak + 5 landmark) |
| `E` | mulai enrollment identitas baru (label anonim `PERSON-00N`) |
| `S` | ambil satu sampel saat mode enrollment |
| `R` | mode recognition |
| `C` | hapus seluruh galeri dari memori |
| `H` | cetak daftar tombol ke terminal |
| `Q` | keluar; data enrollment dibuang |

Argumen lain:

```
--camera N        indeks kamera (bawaan 0)
--width / --height  resolusi yang diminta (bawaan 1280x720)
--threshold F     ambang cosine untuk pengujian (bawaan 0.363)
--margin F        margin minimum antar kandidat (bawaan 0.05)
--samples N       jumlah sampel per identitas (bawaan 5)
--model-dir PATH  direktori berkas .onnx
```

## Alur pengujian manual

1. **Deteksi.** `--mode detect`, pastikan kotak dan lima titik mengikuti wajah.
   Uji juga tanpa wajah (kotak hilang, panel "Tidak ada wajah").
2. **Enrollment.** Tekan `E`, hadapkan **satu** wajah, tekan `S` beberapa kali.
   Sampel ditolak bila tidak ada wajah, lebih dari satu wajah, wajah terlalu
   kecil (<80 px), buram, atau pencahayaan di luar 40–220.
3. **Recognition.** Setelah sampel cukup, mode berpindah otomatis. Panel
   menampilkan status (`cocok`, `tidak_dikenal`, `perlu_verifikasi`), skor, dan
   ambang yang berlaku.
4. **Kondisi tepi.** Uji wajah lain (harus `tidak_dikenal`), dua orang dalam satu
   bingkai (tidak ada kandidat), ruangan gelap, dan wajah miring.

## Alat kalibrasi FL-CV02A

Sesi pengukuran lapangan berurutan: persetujuan → enrollment → kueri per kondisi
→ analisis → pembersihan memori. Alat ini **tidak menulis berkas**; laporan
dicetak ke stdout dan hanya angka agregat yang boleh disalin ke laporan.

```powershell
.venv\Scripts\python.exe -m face_engine.calibrate --help
.venv\Scripts\python.exe -m face_engine.calibrate --peserta 5 --sampel 5 --kueri 3
```

Tombol di jendela: `spasi` ambil, `n` lewati, `q` batal. Minimal dua peserta
ditolak dengan exit code 2, karena pengukuran impostor membutuhkan identitas
pembanding. Kondisi kueri: `terang`, `redup`, `miring`, lalu `kacamata` dan
`tanpa-kacamata` untuk peserta berkacamata. Wajah tak terdaftar diambil dari
peserta yang **menyetujui tetapi tidak dienroll**, bukan dari orang lewat.

## Pengujian otomatis

```powershell
.venv\Scripts\python.exe -m pytest
```

98 pengujian lulus, 1 dilewati bila tidak ada wajah pada gambar acak. Pengujian
**tidak** membuka kamera dan tidak memakai wajah peserta. Yang diuji: aturan
galeri dan matcher, penilaian kualitas, penanganan kamera/model yang tidak
tersedia, alur enrollment dengan model tiruan, serta tata letak baris YuNet dan
rasio aspek masukan (dua regresi yang ditemukan saat pengembangan), ditambah
aritmetika evaluasi FL-CV02A: kesetaraan aturan dengan `matcher.cocokkan()`,
batas ambang, margin, peringkat 1:N, korelasi kualitas, dan laporan.

## Catatan teknis yang mudah salah

Dua kesalahan berikut tidak melempar galat; keduanya hanya membuat hasil tampak
"hampir benar" dan baru terlihat saat skor antarwajah dibandingkan:

1. **Tata letak baris YuNet.** Satu baris berisi 15 nilai dengan urutan
   `x, y, w, h`, **lima pasang landmark**, lalu **skor di indeks terakhir**.
   Menaruh skor di indeks 4 membuat `alignCrop` menerima koordinat palsu.
   Gejalanya: semua wajah berbeda mendapat skor 0,4–0,9 dan tampak cocok.
2. **Rasio aspek masukan.** Memaksa frame 16:9 menjadi kotak 320×320 merusak
   geometri wajah. Pada pengukuran 19 Oktober 2026 deteksi webcam turun dari
   40/40 frame menjadi 0/20. Ukuran masukan sekarang mengikuti rasio frame dengan
   batas sisi panjang 960 px.

## Angka ambang dan asalnya

| Nilai | Asal | Status |
|---|---|---|
| Detektor skor 0,75 | Titik awal pengujian, bukan kalibrasi | Uji |
| Cosine 0,363 | Angka contoh pada dokumentasi OpenCV Zoo, **bukan** hasil pengukuran kita | Uji |
| Margin 0,05 | Dipilih agar dua kandidat berdekatan tidak dipaksa dipilih | Uji |
| Ketajaman 20 | Diukur: frame tajam 35–89, frame diburamkan 2,6–3,9 | Uji (satu kamera) |
| Kecerahan 40–220 | Batas kasar untuk menolak frame gelap/terbakar | Uji |
| Ukuran wajah 80 px | Di bawah ini landmark tidak stabil pada pengukuran | Uji |

**Tidak satu pun angka di atas boleh disebut ambang produksi.** Q-07 tetap
terbuka; kalibrasi menuntut beberapa peserta dengan pencahayaan dan kamera yang
berbeda.

## Hasil pengukuran pada mesin pengembangan

| Pengukuran | Hasil |
|---|---|
| Deteksi webcam, 60 frame, wajah di depan kamera | 60/60 frame, 1 wajah |
| Waktu deteksi per frame (960×544) | ±30 ms |
| Waktu embedding per wajah | ±40 ms |
| Skor wajah terdaftar vs dirinya (60 frame) | min 0,696 / median 0,833 / maks 0,903 |
| Skor wajah berbeda (foto publik, 5 identitas) | 0,025–0,345 |
| Variasi cahaya (+30 / −40) | 0,913–0,984 |
| Variasi skala 0,5× | 0,976–0,987 |
| Rotasi ±10° | 0,872–0,940 |
| Rotasi ±20° | 0,667–0,864 |

Angka ini berasal dari **satu kamera, satu orang, dan satu foto publik**.
Mereka tidak boleh dibaca sebagai akurasi sistem.

## Struktur modul

```
services/face-engine/
├── face_engine/
│   ├── __main__.py     # argparse, mode --cek, penanganan galat
│   ├── app.py          # loop kamera, mode, tombol, aturan enrollment
│   ├── camera.py       # pembungkus cv2.VideoCapture
│   ├── config.py       # konstanta, checksum, Konfigurasi
│   ├── detector.py     # YuNet + penskalaan koordinat
│   ├── embedder.py     # SFace alignCrop + feature + cosine
│   ├── errors.py       # galat berkode
│   ├── gallery.py      # identitas uji di RAM (tanpa serialisasi)
│   ├── matcher.py      # skor, margin, tiga status kandidat
│   ├── overlay.py      # kotak, landmark, panel status
│   ├── quality.py      # penolakan sampel (NO_FACE, MULTIPLE_FACES, ...)
│   ├── calibration.py  # harness sesi: persetujuan, label anonim, pembersihan
│   ├── evaluation.py   # aritmetika kalibrasi (tanpa kamera)
│   └── calibrate.py    # alat lapangan FL-CV02A + laporan stdout
├── scripts/download_models.py
├── tests/
├── models/             # TIDAK di-commit
└── requirements.txt
```

## Keterbatasan yang harus dibaca

- Satu wajah per bingkai untuk recognition; lebih dari satu menghasilkan
  "tidak ada kandidat" dan tidak memilih yang terbesar.
- Kacamata belum diuji pada wajah peserta. Pantulan pada lensa dapat menurunkan
  ketajaman di area mata dan membuat sampel ditolak.
- Tidak ada liveness. Foto cetak atau layar ponsel dapat terdeteksi sebagai
  wajah dan, bila identitasnya sudah terdaftar, dapat menghasilkan skor tinggi.
- Ambang dan margin belum dikalibrasi; Q-07 tetap terbuka. Alat kalibrasi
  FL-CV02A sudah ada, tetapi **belum ada sesi dengan peserta manusia** — lihat
  `docs/CALIBRATION_FL_CV02A.md`.
- Prototype tidak berjalan sebagai layanan jaringan dan tidak terhubung ke Go
  Backend. Kontrak internal `docs/API_CONTRACT.md` §6 belum diimplementasikan.

# Spesifikasi Mesin Pengenalan Wajah Lokal

**Status:** Rencana pengujian, bukan laporan hasil eksperimen. Seluruh proses dilakukan lokal; peserta harus berizin dan dapat memilih absensi manual.
**Alat yang sudah ada:** `services/face-engine` menyediakan harness FL-CV02A (`python -m face_engine.calibrate`) dan modul evaluasi tanpa kamera. Status pelaksanaannya — termasuk **belum adanya peserta manusia** — dicatat di [`CALIBRATION_FL_CV02A.md`](CALIBRATION_FL_CV02A.md). Dokumen ini tetap rencana; matriks §7 belum dijalankan.

**Bukti FL-ENR04 (27 Oktober 2026):** pipeline YuNet → quality/landmark →
SFace tersedia melalui IPC privat Go/Python pada staging sintetis/disposable.
Model/checksum asli ter-load dan negative path NO_FACE terbukti end-to-end;
**REAL_MODEL_POSITIVE_NOT_PROVEN**. Tes komponen SFace dengan landmark buatan
tidak membuktikan positive detection. Matriks manusia §7, kalibrasi, FAR/FRR,
dan PAD/liveness tetap belum terbukti. Laporan:
[`FL_ENR04_TECHNICAL_REPORT.md`](FL_ENR04_TECHNICAL_REPORT.md).

## 1. Tujuan

Mengidentifikasi **satu peserta yang sudah mendaftar** di depan terminal kelas, dengan menolak hasil yang tidak jelas. Pengenalan wajah bukan verifikasi identitas resmi, alat keamanan tingkat tinggi, maupun cara untuk menyimpulkan ketidakhadiran.

## 2. Alur teknis kandidat

1. Adapter kamera menerima satu frame JPEG dan metadata waktu/perangkat.
2. Cek format, ukuran, tingkat blur, cahaya, posisi wajah, dan jumlah wajah.
3. Deteksi wajah memakai model kandidat **YuNet** melalui OpenCV.
4. Sejajarkan wajah dan buat embedding memakai model kandidat **SFace**.
5. Bandingkan embedding dengan template peserta yang **izin biometriknya aktif dan terdaftar pada kelas terkait**.
6. Tinjau skor kandidat teratas, kandidat kedua, ambang kecocokan, margin ketidakpastian, dan versi model.
7. Kembalikan `recognized`, `unknown`, `retry`, atau `manual_review` berikut alasan yang diperlukan backend.
8. **Backend**, bukan AI worker, yang memeriksa sesi dan membuat rekaman kehadiran.

## 3. Enrollment yang dianjurkan

- Jelaskan tujuan pengambilan biometrik dan dapatkan persetujuan sukarela yang tercatat sebelum kamera merekam sampel untuk enrollment.
- Ambil sekitar **3–10 sampel berkualitas**, bukan sekadar 10 gambar acak; jumlah final harus ditentukan lewat percobaan.
- Variasikan posisi lurus, sedikit menoleh, dan perubahan cahaya yang aman; bila peserta biasa berkacamata, sertakan beberapa sampel memakai kacamata.
- Tolak sampel dengan beberapa wajah, blur, wajah terlalu kecil, atau refleksi berlebihan.
- Simpan template terlindungi sesuai kebijakan yang disetujui. **Jangan menyimpan foto mentah secara default**; jika diperlukan untuk studi kualitas, minta persetujuan terpisah serta tentukan waktu penghapusannya.
- Jangan memasukkan foto atau embedding nyata ke source control, LLM, atau screenshot publik.

## 4. Aturan penerimaan pencocokan

- Jangan menyalin angka threshold dari contoh tutorial lalu menganggapnya benar untuk semua kondisi.
- Pilih nilai berdasarkan perbandingan berizin untuk **orang yang sama** dan **orang berbeda**, termasuk variasi kacamata dan pencahayaan.
- Jika skor teratas tak memenuhi ambang atau selisih dengan kandidat kedua terlalu kecil, **jangan check-in**; gunakan `unknown`/`manual_review`.
- Jika embedding dibuat oleh versi model berbeda, hindari mencampurnya tanpa prosedur migrasi dan pengujian.
- Uji kesalahan salah-terima dan salah-tolak; catat keduanya karena dampaknya berbeda.

## 5. Perilaku ketika berkacamata

Tujuan utamanya: peserta berkacamata bening **diusahakan tetap dikenali** setelah enrollment yang cukup dan pengambilan gambar baik. Kacamata hitam, pantulan cahaya kuat, masker, atau sudut wajah ekstrem dapat meningkatkan kegagalan. Sistem harus bisa meminta pengambilan ulang atau absensi manual, **bukan** memberikan tebakan yang berisiko salah orang.

## 6. Serangan foto, video, dan spoofing

Kamera RGB murah rentan terhadap foto cetak, layar HP, atau rekaman video. Gerakan kepala/kedipan bisa digunakan sebagai eksperimen pemeriksaan interaktif, tetapi **tidak membuktikan** perlindungan terhadap replay/video atau deepfake. MVP wajib memiliki pengawasan operator dan laporan keterbatasan; jangan memasarkan sebagai “anti-titip-absen 100%”.

## 7. Matriks pengujian minimum

| Kondisi | Harapan | Ukuran yang dicatat |
|---|---|---|
| Peserta terdaftar, cahaya cukup | Cocok jika memenuhi kriteria | Hasil benar/salah, waktu respons |
| Peserta sama memakai kacamata bening | Dievaluasi setara, bukan dijamin | Perubahan tingkat diterima/ditolak |
| Peserta lain yang belum terdaftar | Tidak otomatis absen | Salah-terima, alasan respons |
| Dua wajah di satu frame | Tolak dan minta satu orang | Jumlah keberhasilan penolakan |
| Blur/cahaya minim/pantulan | Minta ulang atau manual | Kondisi kualitas yang menyebabkan gagal |
| Orang terdaftar mencoba dua kali | Tidak menduplikasi catatan | Jumlah baris database per sesi |
| Persetujuan dicabut | Tidak lagi cocok ke template | Bukti pembersihan cache/penyimpanan |
| Foto cetak/tampilan layar | Uji keterbatasan mitigasi | Cara serangan dan keluaran aktual |
| Mesin AI timeout | Tidak mencatat hadir | Durasi dan pesan kegagalan |

## 8. Rencana evaluasi

- Gunakan sukarelawan dewasa yang telah memberi izin; susun skenario uji dan cara pelabelan hasil sebelum tes.
- Laporkan jumlah peserta, jumlah percobaan, kondisi, kegagalan pengambilan, salah-terima, salah-tolak, dan distribusi latensi.
- Pisahkan data penyesuaian threshold dari data pengujian jika jumlah sampel memadai; hindari klaim akurasi luar sampel ketika data kecil.
- Hasil pada 5–10 peserta hanyalah **bukti fungsi prototipe**, bukan ukuran performa untuk ribuan orang.
- Jangan membandingkan skor FaceLink dengan angka benchmark NIST tanpa metodologi yang sebanding.

## 9. Keamanan model dan dokumen pendukung

Catat pustaka, versi model, checksum berkas, lisensi artefak dan dependensi yang digunakan. Kandidat awal YuNet/SFace dipilih karena dapat digunakan secara lokal; lisensi **berkas yang tepat** tetap harus diperiksa sebelum penggunaan dan redistribusi.

Referensi teknis ada di `REFERENCES.md`.

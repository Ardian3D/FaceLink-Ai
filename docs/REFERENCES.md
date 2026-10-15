# Referensi Teknis dan Regulasi FaceLink AI

**Tujuan:** Daftar sumber awal untuk engineering, keamanan, dan persyaratan produk. **Bukan** hasil bedah jurnal akademik yang menjadi tanggung jawab rekan tim. Periksa ulang versi pustaka, syarat lisensi, tautan, biaya dan kompatibilitas saat benar-benar digunakan.

## 1. Pengenalan wajah dan evaluasi

1. **OpenCV Zoo — Deteksi wajah YuNet**  
   https://github.com/opencv/opencv_zoo/blob/main/models/face_detection_yunet/README.md  
   Referensi model deteksi wajah, contoh pemakaian, dan informasi lisensi pada folder model. Jangan melewati pengujian kualitas gambar lokal.

2. **OpenCV Zoo — Pengenalan wajah SFace**  
   https://github.com/opencv/opencv_zoo/blob/main/models/face_recognition_sface/README.md  
   Referensi ekstraksi embedding dan pencocokan. Angka threshold pada tutorial bukan jaminan kecocokan pada peserta FaceLink AI.

3. **Contoh implementasi SFace**  
   https://github.com/opencv/opencv_zoo/blob/main/models/face_recognition_sface/sface.py  
   Contoh penggunaan skor cosine dan L2. Evaluasi dengan data lokal berizin sebelum menentukan ambang.

4. **NIST — Evaluasi teknologi pengenalan wajah**  
   https://www.nist.gov/programs-projects/face-recognition-vendor-test-frvt  
   Konteks untuk memahami kesalahan pengenalan dan metodologi pengukuran. Bukan bukti bahwa model FaceLink AI mempunyai akurasi seperti benchmark NIST.

5. **NIST — Variasi performa pengenalan wajah pada kelompok demografis**  
   https://pages.nist.gov/frvt/html/frvt_demographics.html  
   Menunjukkan pentingnya pengujian bias dan kesalahan yang dapat berbeda menurut kondisi dan kelompok pengguna.

## 2. Kamera IoT dan backend

6. **Espressif — Driver `esp32-camera`**  
   https://github.com/espressif/esp32-camera  
   Referensi dukungan SoC/sensor, konfigurasi kamera, dan keterbatasan pengambilan gambar. Harus dibandingkan dengan nomor board yang benar sebelum pembelian.

7. **Go — Dokumentasi resmi**  
   https://go.dev/doc/  
   Bahasa Go, modul, cara menguji, dan tata kelola dependensi.

8. **`go-chi/chi` — Router HTTP untuk Go**  
   https://github.com/go-chi/chi  
   Kandidat routing backend. Pilihan Go belum sepenuhnya disahkan dalam keputusan arsitektur.

9. **FastAPI — Dokumentasi resmi**  
   https://fastapi.tiangolo.com/  
   Kandidat transport lokal untuk pekerja Python atau alternatif backend Python.

10. **PostgreSQL — Dokumentasi resmi**  
    https://www.postgresql.org/docs/  
    Constraint, transaksi, dan integritas data jika database PostgreSQL dipilih.

11. **OpenCV — Dokumentasi resmi**  
    https://docs.opencv.org/4.x/  
    Pengambilan gambar, pemrosesan citra, inferensi model, dan API terkait.

## 3. Mobile Android dan pengembangan gratis

12. **React Native — Dokumentasi resmi**  
    https://reactnative.dev/docs/getting-started  
    Dasar pengembangan aplikasi Android/iOS dari satu codebase.

13. **Expo — Membuat APK Android**  
    https://docs.expo.dev/build-reference/apk/  
    Menjelaskan format APK untuk instalasi langsung dibanding AAB untuk distribusi toko.

14. **Expo — Build produksi secara lokal**  
    https://docs.expo.dev/guides/local-app-production/  
    Referensi Android SDK/Gradle dan penandatanganan build. Untuk kebijakan Rp0, utamakan build lokal; layanan cloud build mempunyai kuota dan ketentuan yang dapat berubah.

## 4. Privasi, lisensi, cloud, dan agent

15. **Republik Indonesia — UU Nomor 27 Tahun 2022 tentang Pelindungan Data Pribadi**  
    https://peraturan.bpk.go.id/Details/229798/uu-no-27-tahun-2022  
    Landasan mengenai klasifikasi data biometrik dan kewajiban perlindungan data. Tinjauan hukum lebih lengkap diperlukan sebelum penggunaan institusional.

16. **GitHub — Panduan lisensi repository**  
    https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository  
    Repository yang dapat dilihat publik tidak otomatis berlisensi open-source. Lisensi kode asli FaceLink belum dipilih.

17. **Google Cloud — Informasi program Free Trial**  
    https://cloud.google.com/signup-faqs  
    Masa trial dan kredit terbatas; jangan diperlakukan sebagai layanan gratis permanen. **Integrasi ditunda.**

18. **OmO — Situs project**  
    https://omo.dev/  
    Kandidat orkestrasi AI coding; bukan dependensi aplikasi saat dijalankan. Periksa dokumentasi instalasi/izin/model untuk versi yang digunakan.

## 5. Hal yang harus diverifikasi saat implementasi

- **Lisensi kode vs model:** lisensi pustaka OpenCV, model YuNet/SFace, berkas bobot spesifik, dan dataset pelatihan tidak boleh diasumsikan identik. Catat versi, checksum, serta izin distribusi yang berlaku.
- **Klaim gratis:** library dapat gratis sementara penyedia API atau cloud tetap mengenakan tarif. Hindari integrasi yang memerlukan kartu atau opsi fallback berbayar.
- **Hardware:** angka pada `DEVICE_HARDWARE.md` adalah plafon alokasi, bukan harga checkout yang dipastikan.
- **Model coding `deepseek-v4.1-flash`:** hanya ID yang direncanakan. Keberadaannya di endpoint `/models` tidak membuktikan tool calling, kualitas, uptime, ataupun kuota gratis.
- **Kesesuaian penelitian:** referensi teknis ini tidak menggantikan kajian jurnal atau bukti eksperimen FaceLink AI.

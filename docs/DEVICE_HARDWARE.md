# Rancangan Kamera IoT dan Anggaran Hardware

**Status:** DITUNDA hingga fase hardware setelah software lokal berfungsi. Angka berikut adalah **alokasi anggaran**, bukan kutipan harga toko yang sudah diverifikasi saat pembelian.

## 1. Perangkat target

- Kandidat: **ESP32-S3 CAM** dengan modul kamera kompatibel (contoh OV2640) dan PSRAM memadai. **Periksa varian board, pin kamera, dan ketersediaan library sebelum membeli.**
- Kamera mengambil JPEG dengan interval/laju terbatas ketika sesi absensi sah, lalu mengirim ke backend lewat Wi-Fi lokal; bukan CCTV nonstop.
- Mesin AI utama tetap di laptop Windows; ESP32 tidak dipaksa menjalankan model berat.
- Umpan balik: LED hijau ketika backend menyatakan berhasil, kuning untuk coba ulang, merah saat gangguan; buzzer dan OLED opsional.
- Gunakan sumber daya USB 5V yang sesuai spesifikasi dan casing yang melindungi rangkaian.
- Router MikroTik **tidak wajib**; gunakan Wi-Fi yang sudah ada. Membeli router baru perlu perubahan alokasi budget.

## 2. Batas anggaran Rp1.000.000

| Item | Alokasi maksimum |
|---|---:|
| ESP32-S3 CAM, kamera, dan akses pemrograman | Rp270.000 |
| Dudukan kamera dan casing | Rp160.000 |
| LED, buzzer, tombol, resistor | Rp70.000 |
| OLED opsional | Rp70.000 |
| Catu daya dan kabel USB | Rp80.000 |
| Breadboard, jumper, konektor | Rp90.000 |
| Pencahayaan/diffuser untuk mengurangi pantulan kacamata | Rp100.000 |
| Ongkir, cadangan, komponen pengganti | Rp160.000 |
| **Total batas alokasi** | **Rp1.000.000** |

Komponen tidak wajib seperti OLED dan casing premium dapat dikurangi kalau harga aktual melebihi alokasi. Laptop, smartphone dan router yang telah dimiliki tidak dihitung sebagai pembelian baru. Jangan belanja sebelum ada daftar produk yang kompatibel dan sudah ditinjau.

## 3. Kontrak adapter kamera yang direncanakan

`capture()` menghasilkan `{ device_id, captured_at_utc, frame_jpeg, correlation_id }` sementara. `result()` menerima `accepted`, `duplicate`, `retry`, `manual_review`, atau `error` beserta kode alasan pendek.

- Validasi jenis JPEG, ukuran/resolusi maksimum, autentikasi khusus device, batas usia gambar, dan rate limit.
- Jangan kirim embedding ke perangkat atau menampilkan identitas hasil tebakan saat pencocokan belum pasti.
- Jika jaringan terputus, hentikan percobaan setelah jumlah retry terbatas; jangan mencatat “hadir” secara offline tanpa validasi server.
- Catat kesehatan perangkat terbatas: status terakhir, versi firmware, kondisi daya jika tersedia.

## 4. Urutan integrasi bertahap

1. Buat dan uji antarmuka kamera melalui webcam lokal.
2. Validasi pengambilan JPEG dan alur backend memakai fixture yang diizinkan.
3. Tentukan tipe ESP32-S3 dan konfigurasi kamera/pin yang **persis**.
4. Hubungkan kamera dan satu LED dahulu; uji pengiriman serta respons backend.
5. Tambahkan buzzer/OLED dan casing setelah alur inti stabil.
6. Uji bersama APK, beberapa kondisi cahaya dan peserta berkacamata, serta kasus Wi-Fi putus.

## 5. Peringatan praktis

- Nama keluarga ESP32-S3 saja **tidak menjamin** dukungan firmware pada setiap board murah.
- HTTP polos di jaringan bersama berisiko disadap; demo harus memakai jaringan yang dipercayai, dan penggunaan nyata memerlukan keamanan transport lebih kuat.
- Kamera RGB murah rentan terhadap pencahayaan dan spoofing; jangan mengklaim keamanan produksi.
- Hindari listrik AC PLN langsung pada breadboard atau komponen terbuka.

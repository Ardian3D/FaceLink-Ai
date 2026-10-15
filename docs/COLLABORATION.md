# Kolaborasi GitHub dan Serah-Terima Agent

Proyek ini dikerjakan oleh pengembang utama untuk software dan dapat melibatkan rekan tim dalam penyusunan jurnal akademik. Pisahkan tanggung jawab agar kontribusi tetap rapi.

## 1. Pembagian tanggung jawab

| Pihak | Fokus |
|---|---|
| Pengembang utama | Kebutuhan produk, keputusan teknologi, review, integrasi, keamanan dan rilis demo |
| AI coding agent/OmO | Melaksanakan tugas spesifik, membuat tes, mencatat hasil, memperbarui status setelah disetujui |
| Rekan tim akademik | Mencari/mengkaji jurnal dan menyiapkan bahan presentasi ilmiah pada jalur `research/` |
| Peserta uji | Memberikan persetujuan sukarela dan dapat meminta penghentian serta penghapusan data biometrik |

## 2. Alur Git yang disarankan

1. `main` menyimpan kondisi yang sudah direview dan dapat dijalankan sesuai tahapnya.
2. Satu tugas menggunakan satu branch, misalnya `docs/fl-001-foundation` atau `feat/fl-014-sessions`.
3. PR memuat: tujuan, kriteria penerimaan, berkas yang berubah, keputusan terkait, tes aktual, isu yang belum selesai, dan risiko privasi bila ada.
4. Reviewer memeriksa kode serta bukti tes, bukan hanya ringkasan AI.
5. Hindari force push dan menggabungkan fitur yang gagal tes atau mengubah desain tanpa izin.
6. Perbarui `PROJECT_STATE.md` setelah tugas yang bermakna selesai dan divalidasi.

## 3. Template penjelasan Pull Request

```text
Judul: [FL-xxx] Ringkasan tugas

Tujuan dan alasan:
Ruang lingkup perubahan:
Berkas yang diubah:
Keputusan terkait:
Kriteria penerimaan:
Perintah pengujian dan hasil aktual:
Pengaruh pada keamanan/privasi:
Bug atau batasan yang masih ada:
Langkah berikutnya:
```

## 4. Pergantian AI coding model

Sebelum menyerahkan proyek ke model lain, tulis kondisi Git yang sebenarnya, tidak sekadar mengirim satu prompt besar. Model baru membaca `AGENTS.md` dan `PROJECT_STATE.md`, memeriksa diff, lalu melanjutkan **satu tugas** setelah meninjau dependensinya. Jangan membuat dua agent mengedit berkas yang sama tanpa koordinasi.

## 5. Bahan yang tidak boleh masuk repository

Jangan commit `.env`, token, password, dataset wajah nyata, hasil enrollment, embedding, database pribadi, tangkapan layar yang memperlihatkan data peserta asli, atau rahasia firmware. Untuk presentasi publik, gunakan identitas dan gambar yang sudah diberi izin atau data fiktif.

## 6. Jalur jurnal dan riset tim

Bahan jurnal disimpan terpisah di `research/`; rekan tim memegang tanggung jawab validasi literatur akademik. Referensi implementasi engineering di `docs/REFERENCES.md` tidak otomatis menggantikan telaah jurnal dan tidak boleh dilabeli sebagai hasil penelitian asli tim.

## 7. Lisensi dan kepemilikan

Sebelum menerima kontribusi banyak orang atau merilis secara komersial, sepakati siapa pemegang hak cipta dan lisensi kontribusinya. Repository publik tanpa LICENSE belum berarti bebas digunakan ulang.

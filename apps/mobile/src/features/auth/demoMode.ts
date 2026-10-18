/**
 * Mode demo (FL-UI01).
 *
 * FL-UI01 belum mengintegrasikan Google OAuth. Supaya UI tetap dapat dipreview
 * tanpa menyamar sebagai autentikasi produksi, jalur demo dibatasi ketat:
 *
 *  1. Hanya aktif pada development build (`__DEV__`). Pada build produksi
 *     seluruh jalur demo dimatikan oleh konstanta ini, sehingga tidak ada cara
 *     menaikkan hak akses lewat aplikasi.
 *  2. Tidak pernah memanggil backend, tidak menerbitkan token, dan tidak
 *     menyimpan apa pun ke penyimpanan kredensial.
 *  3. Tidak menampilkan spanduk "DEMO" di layar. Atas permintaan pemilik
 *     project (18 Oktober 2026), spanduk itu dihapus agar tampilan preview
 *     sama dengan tampilan akhir. Konsekuensinya: pada development build,
 *     beranda yang berisi data contoh **tidak lagi ditandai** di layar, jadi
 *     jangan menganggap angka di sana sebagai data nyata.
 *
 * Saat integrasi Google Auth dikerjakan (tugas terpisah), `SessionProvider`
 * diganti penyedia berbasis backend dan berkas ini dihapus bersama layar
 * `dev-preview`.
 */
export const DEMO_MODE: boolean = __DEV__;

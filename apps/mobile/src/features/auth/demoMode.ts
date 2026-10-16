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
 *  3. Selalu menampilkan penanda `DEMO` pada layar yang memakainya, supaya
 *     hasil preview tidak pernah tertukar dengan data produksi.
 *
 * Saat integrasi Google Auth dikerjakan (tugas terpisah), `SessionProvider`
 * diganti penyedia berbasis backend dan berkas ini dihapus bersama layar
 * `dev-preview`.
 */
export const DEMO_MODE: boolean = __DEV__;

/** Label penanda yang wajib tampil selama mode demo aktif. */
export const DEMO_LABEL = "DEMO";

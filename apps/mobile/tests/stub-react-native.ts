/**
 * Pengganti `react-native` untuk pengujian Node.
 *
 * Paket `react-native` berisi sintaks Flow dan modul native; ia tidak dapat
 * dimuat oleh Vite. Yang dipakai kode autentikasi hanyalah `Platform.OS`, jadi
 * hanya itu yang disediakan. Nilainya dapat diubah uji lewat
 * `globalThis.__PLATFORM_OS__` sebelum modul diimpor ulang.
 */
export const Platform = {
  get OS(): string {
    return (globalThis as unknown as { __PLATFORM_OS__?: string }).__PLATFORM_OS__ ?? "android";
  },
};

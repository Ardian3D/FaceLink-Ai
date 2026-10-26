import { defineConfig } from "vitest/config";
import path from "node:path";
/// <reference types="node" />

/**
 * Konfigurasi pengujian unit aplikasi mobile.
 *
 * **Kenapa Vitest, bukan Jest:** Jest memerlukan `babel-jest` +
 * `jest-expo` (satu set besar dependensi build) dan mengubah cara Metro
 * membundel. Vitest menjalankan TypeScript langsung lewat esbuild tanpa
 * menyentuh konfigurasi bundel, sehingga pengujian tidak dapat membuat
 * `npm start` bekerja berbeda dari produksi.
 *
 * **Batas yang disengaja:** lingkungan `node`, bukan jsdom. Yang diuji adalah
 * `authFlow.ts` (tanpa React) dan lapisan API; modul native Google, SecureStore,
 * dan `fetch` diganti tiruan. Tidak ada pengujian rendering di sini — tata letak
 * diperiksa manual di pratinjau web (lihat README).
 *
 * Alias `@/` disalin dari `tsconfig.json` supaya impor di dalam kode yang diuji
 * tidak perlu diubah.
 */
export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "src"),
      // `react-native` berisi Flow + modul native dan tidak dapat dimuat Node.
      // Kode autentikasi hanya memakai `Platform.OS`.
      "react-native": path.resolve(import.meta.dirname, "tests/stub-react-native.ts"),
    },
  },
  test: {
    environment: "node",
    include: ["tests/**/*.test.ts"],
    // Tidak ada variabel lingkungan EXPO_PUBLIC_* dari mesin ini yang boleh
    // bocor ke pengujian: nilai konfigurasi diatur per berkas uji.
    env: {},
    restoreMocks: true,
  },
});

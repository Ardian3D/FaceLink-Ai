/**
 * Titik masuk tema.
 *
 * Komponen memakai `useTheme()` untuk warna, dan token statis (spacing,
 * radius, tipografi, layout) langsung dari sini. Warna **tidak** boleh
 * diimpor sebagai konstanta oleh komponen yang tampil di layar: warna berubah
 * mengikuti tema, sedangkan spacing tidak.
 */
export { ThemeProvider, BrandThemeScope, useTheme, palet } from "./ThemeProvider";
export type { ThemeMode } from "./ThemeProvider";
export { spacing, radius, typography, touch, layout, elevation, fonts } from "./tokens";
export type { Palette } from "./tokens";

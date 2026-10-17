/**
 * Titik masuk tema. Komponen mengimpor dari sini, bukan dari `tokens` langsung,
 * supaya penambahan tema lain (mis. mode terang) nanti tidak menyentuh pemanggil.
 */
export { theme, colors, spacing, radius, typography, touch, layout, elevation } from "./tokens";
export type { Theme } from "./tokens";

/**
 * Design tokens FaceLink AI (FL-UI01).
 *
 * PENTING — status palet ini SEMENTARA. Pemilik proyek belum memberikan warna
 * brand, dan keputusan visual final ada padanya. Nilai di bawah sengaja dipilih
 * dari palet netral (keluarga slate) supaya seluruh layar terbaca rapi tanpa
 * mengarang identitas brand. Ganti nilai di berkas ini saja; tidak ada warna,
 * jarak, radius, atau ukuran teks yang ditulis langsung di komponen.
 *
 * Pemetaan saat aset brand tersedia:
 *   colors.background / surface / border  -> latar dan pemisah
 *   colors.accent                          -> satu warna aksi utama
 *   colors.textPrimary / textSecondary     -> hierarki teks
 */

export const colors = {
  /** Latar paling belakang. */
  background: "#0F172A",
  /** Latar kartu dan panel yang mengambang di atas background. */
  surface: "#1E293B",
  /** Latar elemen di dalam kartu (mis. baris daftar). */
  surfaceMuted: "#273549",
  /** Garis pemisah dan tepi kartu. */
  border: "#334155",

  /** Warna aksi utama. Placeholder — menunggu warna brand. */
  accent: "#94B0D6",
  /** Teks di atas warna accent. */
  onAccent: "#0F172A",

  textPrimary: "#F1F5F9",
  textSecondary: "#CBD5E1",
  textMuted: "#94A3B8",

  /** Status. Selalu dipakai bersama ikon dan label, tidak pernah warna saja. */
  success: "#4ADE80",
  warning: "#FBBF24",
  error: "#F87171",
  info: "#7DD3FC",

  /** Penanda mode demo pada preview. */
  demo: "#F0ABFC",
} as const;

/** Skala jarak 4pt. Semua padding/margin memakai salah satu nilai ini. */
export const spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  xxl: 32,
  xxxl: 48,
} as const;

/** Radius sudut. */
export const radius = {
  sm: 8,
  md: 12,
  lg: 16,
  xl: 24,
  pill: 999,
} as const;

/**
 * Tipografi. Ukuran mengikuti skala yang nyaman dibaca saat demo kelas; tidak
 * ada font kustom pada FL-UI01 supaya tidak menambah dependensi sebelum brand
 * tipografi ditentukan pemilik.
 */
export const typography = {
  size: {
    caption: 12,
    body: 14,
    bodyLarge: 16,
    title: 20,
    heading: 24,
    display: 32,
  },
  weight: {
    regular: "400",
    medium: "500",
    semibold: "600",
    bold: "700",
  },
  lineHeight: {
    caption: 16,
    body: 20,
    bodyLarge: 24,
    title: 28,
    heading: 32,
    display: 40,
  },
} as const;

/**
 * Ukuran target sentuh minimum. Pedoman UX (§7) menuntut target sentuh ramah
 * mobile; nilai 48 mengikuti panduan aksesibilitas Android.
 */
export const touch = {
  minHeight: 48,
  minWidth: 48,
  hitSlop: 8,
} as const;

/** Tinggi dan lebar tetap untuk elemen berulang. */
export const layout = {
  screenPadding: spacing.lg,
  maxContentWidth: 560,
  logoSize: 96,
  logoSizeCompact: 64,
  scanIndicatorSize: 132,
  avatarSize: 40,
  iconSize: 20,
  iconSizeLarge: 28,
} as const;

/** Tingkat elevasi kartu. Dipakai hemat agar tampilan tidak ramai. */
export const elevation = {
  none: {},
  card: {
    shadowColor: "#000000",
    shadowOpacity: 0.18,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 3,
  },
} as const;

export const theme = {
  colors,
  spacing,
  radius,
  typography,
  touch,
  layout,
  elevation,
} as const;

export type Theme = typeof theme;

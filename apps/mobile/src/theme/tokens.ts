/**
 * Design tokens FaceLink AI.
 *
 * Palet ini berasal dari **logo resmi** yang diberikan pemilik proyek, bukan
 * lagi placeholder: latar hitam pekat, mark putih, dan satu aksen merah.
 * Nilai merah diambil langsung dari piksel logo resmi (`#FF171B`, saluran
 * paling jenuh pada berkas `logo-white-noBg.png`).
 *
 * Seluruh warna, jarak, radius, dan ukuran teks hanya boleh didefinisikan di
 * berkas ini. Mengganti warna brand berarti mengubah satu tempat.
 */

export const colors = {
  /** Latar paling belakang. Hitam pekat, sama seperti latar logo resmi. */
  background: "#000000",
  /** Permukaan kartu dan panel. */
  surface: "#111114",
  /** Permukaan di dalam kartu, mis. baris daftar. */
  surfaceMuted: "#1C1C21",
  /** Garis pemisah dan tepi kartu. */
  border: "#2B2B31",

  /** Warna aksi utama, diambil dari aksen merah logo resmi. */
  accent: "#FF171B",
  /** Aksen saat ditekan. */
  accentPressed: "#D41115",
  /** Teks di atas aksen merah. */
  onAccent: "#FFFFFF",

  /** Tombol Google: putih, dengan teks gelap di atasnya. */
  buttonLight: "#FFFFFF",
  buttonLightPressed: "#E6E6E8",
  onButtonLight: "#111114",

  textPrimary: "#FFFFFF",
  textSecondary: "#D6D6DB",
  textMuted: "#9A9AA3",

  /** Status. Selalu dipakai bersama ikon dan label, tidak pernah warna saja. */
  success: "#4ADE80",
  warning: "#FFB020",
  error: "#FF5A5F",
  info: "#7DD3FC",

  /**
   * Penanda mode demo. Sengaja bukan merah brand dan bukan warna status, supaya
   * tidak terbaca sebagai bagian identitas atau sebagai peringatan sistem.
   */
  demo: "#B9A7FF",

  /**
   * Nuansa merah sangat gelap untuk latar animasi layar masuk. Bukan warna
   * merek dan bukan warna status: hanya kedalaman latar.
   */
  backgroundAccent: "#170406",

  /** Cahaya aksen pada latar animasi layar masuk, dari pekat ke transparan. */
  glowStrong: "rgba(255, 23, 27, 0.22)",
  glowSoft: "rgba(255, 23, 27, 0.12)",
  glowFade: "rgba(255, 23, 27, 0)",

  /** Lapisan gelap di atas video agar teks tetap terbaca. */
  overlayStrong: "rgba(0, 0, 0, 0.92)",
  overlayLight: "rgba(0, 0, 0, 0.42)",
  overlayMedium: "rgba(0, 0, 0, 0.55)",
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
 * Tipografi. Ukuran mengikuti skala yang nyaman dibaca saat demo kelas. Belum
 * ada font kustom: font brand pada mockup belum diidentifikasi, dan menebaknya
 * akan menghasilkan tampilan yang hampir mirip tetapi tidak tepat.
 */
export const typography = {
  size: {
    caption: 12,
    body: 14,
    bodyLarge: 16,
    title: 20,
    heading: 24,
    display: 32,
    wordmark: 44,
  },
  weight: {
    regular: "400",
    medium: "500",
    semibold: "600",
    bold: "700",
    heavy: "800",
  },
  lineHeight: {
    caption: 16,
    body: 20,
    bodyLarge: 24,
    title: 28,
    heading: 32,
    display: 40,
    wordmark: 52,
  },
  /** Jarak antarhuruf untuk wordmark, menyesuaikan gaya logo. */
  tracking: {
    tight: -0.5,
    wordmark: -1,
    wide: 1.2,
  },
} as const;

/**
 * Ukuran target sentuh minimum. Pedoman UX §7 menuntut target sentuh ramah
 * mobile; nilai 48 mengikuti panduan aksesibilitas Android.
 */
export const touch = {
  minHeight: 48,
  minWidth: 48,
  hitSlop: 8,
} as const;

/** Tinggi dan lebar tetap untuk elemen berulang. */
export const layout = {
  screenPadding: spacing.xl,
  maxContentWidth: 560,
  logoSize: 104,
  logoSizeCompact: 64,
  scanIndicatorSize: 132,
  avatarSize: 40,
  iconSize: 20,
  iconSizeLarge: 28,
  /** Tinggi tombol utama pada layar masuk. */
  buttonHeight: 58,
} as const;

/** Tingkat elevasi kartu. Dipakai hemat agar tampilan tidak ramai. */
export const elevation = {
  none: {},
  card: {
    shadowColor: "#000000",
    shadowOpacity: 0.35,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 6 },
    elevation: 4,
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

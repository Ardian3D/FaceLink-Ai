/**
 * Design tokens FaceLink AI.
 *
 * Palet berasal dari **logo resmi** pemilik project: hitam pekat, mark putih,
 * dan satu aksen merah (`#FF171B`, diambil dari piksel logo). Aturan mereknya
 * ketat — hanya hitam, putih, dan merah — jadi tidak ada warna merek lain yang
 * boleh ditambahkan di sini.
 *
 * Dua palet disediakan:
 *
 * - `dark`  — tampilan utama, sesuai logo resmi.
 * - `light` — dipilih pengguna lewat Settings. Bukan sekadar pembalikan warna:
 *   aksen merah digelapkan sedikit (`#D41115`) supaya kontras teks di atasnya
 *   tetap memenuhi pedoman keterbacaan pada latar putih.
 *
 * Warna status (success/warning/error/info) tetap memakai keluarga warna yang
 * berbeda dari merek. Warna itu tidak boleh menjadi satu-satunya penanda status:
 * setiap pemakaiannya selalu disertai ikon dan teks.
 */

export type Palette = {
  background: string;
  surface: string;
  surfaceMuted: string;
  surfaceElevated: string;
  border: string;

  accent: string;
  accentPressed: string;
  accentGlow: string;
  accentDim: string;
  onAccent: string;

  buttonLight: string;
  buttonLightPressed: string;
  onButtonLight: string;

  textPrimary: string;
  textSecondary: string;
  textMuted: string;
  textOnAccent: string;

  success: string;
  warning: string;
  error: string;
  info: string;

  demo: string;

  backgroundAccent: string;
  glowStrong: string;
  glowSoft: string;
  glowFade: string;

  hudLine: string;
  hudBracket: string;
  hudScanGlow: string;

  overlayStrong: string;
  overlayMedium: string;
  overlayLight: string;

  /** Isian tipis untuk kartu sorotan (mis. kartu merah utama). */
  accentSurface: string;
  /** Warna garis bantu grafik. */
  chartTrack: string;
};

/** Palet gelap: tampilan utama aplikasi. */
export const dark: Palette = {
  background: "#000000",
  surface: "#111114",
  surfaceMuted: "#1C1C21",
  surfaceElevated: "#232329",
  border: "#2B2B31",

  accent: "#FF171B",
  accentPressed: "#D41115",
  accentGlow: "rgba(255, 23, 27, 0.12)",
  accentDim: "rgba(255, 23, 27, 0.45)",
  onAccent: "#FFFFFF",

  buttonLight: "#FFFFFF",
  buttonLightPressed: "#E6E6E8",
  onButtonLight: "#111114",

  textPrimary: "#FFFFFF",
  textSecondary: "#D6D6DB",
  textMuted: "#9A9AA3",
  textOnAccent: "#FFFFFF",

  success: "#4ADE80",
  warning: "#FFB020",
  error: "#FF5A5F",
  info: "#7DD3FC",

  demo: "#B9A7FF",

  backgroundAccent: "#170406",
  glowStrong: "rgba(255, 23, 27, 0.22)",
  glowSoft: "rgba(255, 23, 27, 0.12)",
  glowFade: "rgba(255, 23, 27, 0)",

  hudLine: "#3A3A40",
  hudBracket: "#E8E8EC",
  hudScanGlow: "rgba(255, 23, 27, 0.10)",

  overlayStrong: "rgba(0, 0, 0, 0.92)",
  overlayMedium: "rgba(0, 0, 0, 0.55)",
  overlayLight: "rgba(0, 0, 0, 0.42)",

  accentSurface: "#1A0507",
  chartTrack: "#2B2B31",
};

/**
 * Palet terang. Aksen merah digelapkan supaya teks putih di atasnya tetap
 * terbaca (kontras ≥ 4,5:1 pada ukuran teks normal).
 */
export const light: Palette = {
  background: "#FFFFFF",
  surface: "#F7F7F8",
  surfaceMuted: "#EFEFF1",
  surfaceElevated: "#FFFFFF",
  border: "#DCDCE0",

  accent: "#D41115",
  accentPressed: "#B00E12",
  accentGlow: "rgba(212, 17, 21, 0.10)",
  accentDim: "rgba(212, 17, 21, 0.40)",
  onAccent: "#FFFFFF",

  buttonLight: "#111114",
  buttonLightPressed: "#2B2B31",
  onButtonLight: "#FFFFFF",

  textPrimary: "#0B0B0D",
  textSecondary: "#3A3A40",
  textMuted: "#6E6E76",
  textOnAccent: "#FFFFFF",

  success: "#128A45",
  warning: "#A65B00",
  error: "#C01519",
  info: "#0B6E99",

  demo: "#6B4FD8",

  backgroundAccent: "#FFF5F5",
  glowStrong: "rgba(212, 17, 21, 0.18)",
  glowSoft: "rgba(212, 17, 21, 0.10)",
  glowFade: "rgba(212, 17, 21, 0)",

  hudLine: "#C9C9CE",
  hudBracket: "#3A3A40",
  hudScanGlow: "rgba(212, 17, 21, 0.10)",

  overlayStrong: "rgba(255, 255, 255, 0.92)",
  overlayMedium: "rgba(255, 255, 255, 0.55)",
  overlayLight: "rgba(255, 255, 255, 0.42)",

  accentSurface: "#FFF0F0",
  chartTrack: "#E4E4E8",
};

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
  xxl: 28,
  pill: 999,
} as const;

/**
 * Tipografi. Ukuran mengikuti skala yang nyaman dibaca saat demo kelas.
 *
 * Keluarga huruf: **Poppins** (SIL Open Font License, gratis), dipilih pemilik
 * project. Berkas TTF-nya dipasang lewat `@expo-google-fonts/poppins` dan
 * dimuat sekali di `app/_layout.tsx`. Karena setiap ketebalan adalah berkas
 * font tersendiri, pemilihan ketebalan memakai `fontFamily`, **bukan**
 * `fontWeight` — mencampur keduanya membuat Android memilih fallback sistem.
 */
export const fonts = {
  regular: "Poppins_400Regular",
  medium: "Poppins_500Medium",
  semibold: "Poppins_600SemiBold",
  bold: "Poppins_700Bold",
  heavy: "Poppins_800ExtraBold",
} as const;

export const typography = {
  size: {
    micro: 10,
    caption: 12,
    body: 14,
    bodyLarge: 16,
    title: 20,
    heading: 24,
    display: 32,
    wordmark: 44,
    hero: 30,
  },
  weight: {
    regular: "400",
    medium: "500",
    semibold: "600",
    bold: "700",
    heavy: "800",
  },
  family: {
    regular: fonts.regular,
    medium: fonts.medium,
    semibold: fonts.semibold,
    bold: fonts.bold,
    heavy: fonts.heavy,
  },
  lineHeight: {
    micro: 14,
    caption: 16,
    body: 20,
    bodyLarge: 24,
    title: 28,
    heading: 32,
    display: 40,
    wordmark: 52,
    hero: 38,
  },
  /** Jarak antarhuruf untuk wordmark, menyesuaikan gaya logo. */
  tracking: {
    tight: -0.5,
    wordmark: -1,
    wide: 1.2,
    /** Untuk label kecil huruf besar, mis. "HARI INI". */
    caps: 1.6,
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
  logoSize: 112,
  logoSizeCompact: 40,
  /** Logo kecil di topbar dashboard. */
  logoSizeBar: 30,
  avatarSize: 40,
  iconSize: 20,
  iconSizeLarge: 28,
  /** Tinggi tombol utama pada layar masuk. */
  buttonHeight: 58,
  /** Diameter cincin HUD pada layar pemeriksaan peran. */
  loadingHudSize: 300,
  /** Tinggi kartu sorotan pada dashboard. */
  heroCardHeight: 168,
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
  /** Kartu yang menempel di atas latar terang tidak butuh bayangan pekat. */
  soft: {
    shadowColor: "#000000",
    shadowOpacity: 0.08,
    shadowRadius: 10,
    shadowOffset: { width: 0, height: 3 },
    elevation: 2,
  },
} as const;

export const theme = {
  dark,
  light,
  spacing,
  radius,
  typography,
  touch,
  layout,
  elevation,
} as const;

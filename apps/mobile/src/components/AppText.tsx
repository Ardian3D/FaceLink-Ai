import React from "react";
import { Text, type TextProps } from "react-native";

import { typography, useTheme } from "@/theme";

type Variant = keyof typeof typography.size;
type Weight = keyof typeof typography.weight;

/**
 * Teks bertema. Seluruh layar memakai komponen ini supaya hierarki ukuran,
 * ketebalan, dan warna teks tidak ditulis ulang di tiap komponen.
 *
 * Keluarga huruf Poppins dipilih lewat `fontFamily` (satu berkas per
 * ketebalan), bukan `fontWeight`: pada Android, memaksa `fontWeight` untuk font
 * kustom membuat sistem memilih fallback dan ketebalan yang diminta hilang.
 * `fontWeight` tetap ditulis agar pengukuran dan pemilihan fallback web masuk
 * akal bila berkas font belum termuat.
 */
export function AppText({
  variant = "body",
  color,
  weight,
  center = false,
  style,
  ...rest
}: TextProps & {
  variant?: Variant;
  color?: string;
  weight?: Weight;
  center?: boolean;
}) {
  const { colors } = useTheme();
  const ketebalan: Weight = weight ?? (variant === "body" ? "regular" : "semibold");
  // Warna teks mengikuti tema aktif; pemanggil boleh menimpanya.
  const warna = color ?? colors.textPrimary;

  return (
    <Text
      {...rest}
      style={[
        {
          fontSize: typography.size[variant],
          lineHeight: typography.lineHeight[variant],
          color: warna,
          fontFamily: typography.family[ketebalan],
          fontWeight: typography.weight[ketebalan],
        },
        center && { textAlign: "center" },
        style,
      ]}
    />
  );
}

import React from "react";
import { Text, type TextProps } from "react-native";

import { colors, typography } from "@/theme";

type Variant = keyof typeof typography.size;

/**
 * Teks bertema. Seluruh layar memakai komponen ini supaya hierarki ukuran dan
 * warna teks tidak ditulis ulang di tiap komponen.
 */
export function AppText({
  variant = "body",
  color = colors.textPrimary,
  weight,
  center = false,
  style,
  ...rest
}: TextProps & {
  variant?: Variant;
  color?: string;
  weight?: keyof typeof typography.weight;
  center?: boolean;
}) {
  return (
    <Text
      {...rest}
      style={[
        {
          fontSize: typography.size[variant],
          lineHeight: typography.lineHeight[variant],
          color,
          fontWeight: typography.weight[weight ?? (variant === "body" ? "regular" : "semibold")],
        },
        center && { textAlign: "center" },
        style,
      ]}
    />
  );
}

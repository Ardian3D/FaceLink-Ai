import React from "react";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";
import Svg, { Circle } from "react-native-svg";

import { radius, useTheme } from "@/theme";

/**
 * Ilustrasi untuk keadaan "akun belum terdaftar".
 *
 * Digambar dengan SVG, bukan gambar: cincin dan ikon tetap tajam pada semua
 * kerapatan layar, dan warnanya mengikuti tema. Bentuknya sengaja sederhana —
 * satu lingkaran dengan ikon orang dan lencana merah — supaya terbaca sebagai
 * "akun tidak ditemukan", bukan sebagai hiasan yang menutupi pesan.
 */
export function UnregisteredIllustration({ size = 132 }: { size?: number }) {
  const { colors } = useTheme();
  const tengah = size / 2;
  const jari = tengah - 2;

  return (
    <View style={{ width: size, height: size }} accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      <Svg width={size} height={size}>
        <Circle cx={tengah} cy={tengah} r={jari} stroke={colors.hudLine} strokeWidth={1.5} fill="none" />
        {/* Cincin putus-putus di dalam memberi kesan "belum lengkap". */}
        <Circle
          cx={tengah}
          cy={tengah}
          r={jari * 0.78}
          stroke={colors.hudLine}
          strokeWidth={1}
          strokeDasharray="4 6"
          fill="none"
          opacity={0.7}
        />
      </Svg>

      <View style={[StyleSheet.absoluteFill, styles.tengah]}>
        <Ionicons name="person-outline" size={size * 0.34} color={colors.textMuted} />
      </View>

      {/* Lencana merah di kanan bawah: penanda bahwa ada yang tidak cocok. */}
      <View
        style={[
          styles.lencana,
          {
            width: size * 0.28,
            height: size * 0.28,
            borderRadius: radius.pill,
            backgroundColor: colors.accent,
            borderColor: colors.background,
          },
        ]}
      >
        <Ionicons name="close" size={size * 0.16} color={colors.onAccent} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  tengah: {
    alignItems: "center",
    justifyContent: "center",
  },
  lencana: {
    position: "absolute",
    right: 0,
    bottom: 0,
    alignItems: "center",
    justifyContent: "center",
    borderWidth: 3,
  },
});

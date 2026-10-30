import React from "react";
import { StyleSheet, View, useWindowDimensions } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "./AppText";
import { radius, spacing, useTheme } from "@/theme";

/**
 * Tiga langkah proses masuk pada layar pemeriksaan peran.
 *
 * Langkah yang sedang berjalan ditandai tiga hal sekaligus — cincin merah,
 * ikon putih, dan teks putih — bukan warna saja, supaya tetap terbaca oleh
 * pengguna dengan gangguan penglihatan warna. Langkah yang sudah selesai
 * memakai centang; langkah yang belum dijalani tetap redup.
 *
 * `aksesibilitas` membacakan status tiap langkah, karena posisi visual saja
 * tidak cukup untuk pembaca layar.
 */
export type LangkahStatus = "selesai" | "berjalan" | "menunggu";

export type Langkah = {
  label: string;
  ikon: keyof typeof Ionicons.glyphMap;
  status: LangkahStatus;
};

export function StepTracker({ langkah }: { langkah: Langkah[] }) {
  const { colors } = useTheme();
  const { width } = useWindowDimensions();
  // Lebar tiap langkah dibagi rata, dengan batas atas supaya pada tablet tidak
  // melebar berlebihan. Tanpa ini, label terpotong pada layar 320 px.
  const lebarItem = Math.min(104, Math.max(64, Math.floor((width - 48) / langkah.length) - 8));
  const lebarPenghubung = Math.max(8, Math.min(36, Math.round(lebarItem * 0.35)));

  return (
    <View style={styles.baris}>
      {langkah.map((l, i) => {
        const berjalan = l.status === "berjalan";
        const selesai = l.status === "selesai";

        return (
          <React.Fragment key={l.label}>
            {i > 0 ? (
              <View
                style={[
                  styles.penghubung,
                  { width: lebarPenghubung, backgroundColor: colors.hudLine },
                  (selesai || berjalan) && { backgroundColor: colors.accentDim },
                ]}
              />
            ) : null}

            <View
              style={[styles.item, { width: lebarItem }]}
              accessible
              accessibilityLabel={`${l.label}: ${l.status}`}
            >
              <View
                style={[
                  styles.lingkaran,
                  { borderColor: colors.hudLine },
                  // Cincin merah + halo tipis, seperti langkah aktif pada mockup.
                  berjalan && { borderColor: colors.accent, backgroundColor: colors.accentGlow },
                ]}
              >
                {selesai ? (
                  <Ionicons name="checkmark" size={22} color={colors.textMuted} />
                ) : (
                  <Ionicons
                    name={l.ikon}
                    size={22}
                    color={berjalan ? colors.textPrimary : colors.textMuted}
                  />
                )}
              </View>
              <AppText
                variant="caption"
                center
                color={berjalan ? colors.textPrimary : colors.textMuted}
                weight={berjalan ? "semibold" : "regular"}
                style={styles.label}
              >
                {l.label}
              </AppText>
            </View>
          </React.Fragment>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  baris: {
    flexDirection: "row",
    alignItems: "flex-start",
    justifyContent: "center",
  },
  item: {
    alignItems: "center",
    gap: spacing.md,
  },
  lingkaran: {
    width: 56,
    height: 56,
    borderRadius: radius.pill,
    borderWidth: 1.5,
    alignItems: "center",
    justifyContent: "center",
  },
  penghubung: {
    height: 1.5,
    marginTop: 27,
  },
  label: {
    lineHeight: 18,
  },
});

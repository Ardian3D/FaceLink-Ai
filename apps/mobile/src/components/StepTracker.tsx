import React from "react";
import { StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { AppText } from "./AppText";
import { colors, radius, spacing } from "@/theme";

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
  return (
    <View style={styles.baris} accessibilityRole="progressbar">
      {langkah.map((l, i) => {
        const berjalan = l.status === "berjalan";
        const selesai = l.status === "selesai";

        return (
          <React.Fragment key={l.label}>
            {i > 0 ? (
              <View
                style={[styles.penghubung, (selesai || berjalan) && styles.penghubungAktif]}
              />
            ) : null}

            <View
              style={styles.item}
              accessible
              accessibilityLabel={`${l.label}: ${l.status}`}
            >
              <View
                style={[
                  styles.lingkaran,
                  selesai && styles.lingkaranSelesai,
                  berjalan && styles.lingkaranBerjalan,
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
    width: 92,
    alignItems: "center",
    gap: spacing.md,
  },
  lingkaran: {
    width: 56,
    height: 56,
    borderRadius: radius.pill,
    borderWidth: 1.5,
    borderColor: colors.hudLine,
    alignItems: "center",
    justifyContent: "center",
  },
  lingkaranSelesai: {
    borderColor: colors.hudLine,
  },
  /** Cincin merah + halo tipis, seperti langkah aktif pada mockup. */
  lingkaranBerjalan: {
    borderColor: colors.accent,
    backgroundColor: colors.accentGlow,
  },
  penghubung: {
    height: 1.5,
    width: 36,
    marginTop: 27,
    backgroundColor: colors.hudLine,
  },
  penghubungAktif: {
    backgroundColor: colors.accentDim,
  },
  label: {
    lineHeight: 18,
  },
});

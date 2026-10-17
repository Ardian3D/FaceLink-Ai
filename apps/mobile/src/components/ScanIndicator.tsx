import React, { useEffect, useState } from "react";
import { Animated, Easing, Platform, StyleSheet, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { colors, layout, radius } from "@/theme";

/**
 * Indikator pemindaian untuk layar Role Checking.
 *
 * Animasi memakai `Animated` bawaan React Native, bukan library animasi
 * tambahan: pedoman UX §7 meminta tampilan yang tidak membutuhkan animasi berat,
 * dan satu dependensi lebih sedikit berarti satu risiko versi lebih sedikit.
 *
 * Animasi ini murni visual. Ia tidak menandakan bahwa sesuatu sedang
 * diverifikasi, sehingga tidak boleh dipakai sebagai bukti status autentikasi.
 */
export function ScanIndicator({ size = layout.scanIndicatorSize }: { size?: number }) {
  // Web tidak memiliki modul animasi native; memaksakan driver native di sana
  // hanya menghasilkan peringatan runtime dan jatuh kembali ke animasi JS.
  // Android tetap memakai driver native supaya animasinya berjalan di luar
  // thread JavaScript.
  const driverNative = Platform.OS !== "web";
  // Nilai animasi disimpan sebagai state, bukan ref: `Animated.Value` dibaca
  // saat render (untuk membentuk interpolasi), dan nilai ref tidak boleh dibaca
  // pada fase itu.
  const [denyut] = useState(() => new Animated.Value(0));
  const [putar] = useState(() => new Animated.Value(0));

  useEffect(() => {
    const loopDenyut = Animated.loop(
      Animated.sequence([
        Animated.timing(denyut, {
          toValue: 1,
          duration: 1100,
          easing: Easing.out(Easing.quad),
          useNativeDriver: driverNative,
        }),
        Animated.timing(denyut, {
          toValue: 0,
          duration: 1100,
          easing: Easing.in(Easing.quad),
          useNativeDriver: driverNative,
        }),
      ])
    );
    const loopPutar = Animated.loop(
      Animated.timing(putar, {
        toValue: 1,
        duration: 2600,
        easing: Easing.linear,
        useNativeDriver: driverNative,
      })
    );
    loopDenyut.start();
    loopPutar.start();
    return () => {
      loopDenyut.stop();
      loopPutar.stop();
    };
  }, [denyut, putar, driverNative]);

  const skala = denyut.interpolate({ inputRange: [0, 1], outputRange: [0.92, 1.06] });
  const opasitas = denyut.interpolate({ inputRange: [0, 1], outputRange: [0.35, 0.85] });
  const rotasi = putar.interpolate({ inputRange: [0, 1], outputRange: ["0deg", "360deg"] });

  return (
    <View style={{ width: size, height: size }} accessibilityLabel="Indikator pemindaian (demo)">
      {/* Cincin luar berdenyut sebagai isyarat visual "sedang berjalan". */}
      <Animated.View
        style={[
          styles.cincin,
          {
            width: size,
            height: size,
            borderRadius: radius.pill,
            opacity: opasitas,
            transform: [{ scale: skala }],
          },
        ]}
      />
      {/* Lengkung berputar: bahasa visual pemindai wajah. */}
      <Animated.View
        style={[
          styles.lengkung,
          {
            width: size * 0.78,
            height: size * 0.78,
            borderRadius: radius.pill,
            transform: [{ rotate: rotasi }],
          },
        ]}>
        <View style={[styles.kepala, { width: size * 0.78 }]}>
          <View style={[styles.titikKepala, { width: size * 0.12 }]} />
        </View>
      </Animated.View>
      <View style={styles.pusat}>
        <Ionicons name="scan-outline" size={size * 0.32} color={colors.accent} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  cincin: {
    position: "absolute",
    borderWidth: 2,
    borderColor: colors.accent,
  },
  lengkung: {
    position: "absolute",
    alignSelf: "center",
    top: "11%",
    borderWidth: 2,
    borderColor: "transparent",
    borderTopColor: colors.info,
    borderLeftColor: colors.info,
  },
  kepala: {
    alignItems: "center",
  },
  titikKepala: {
    height: 4,
    borderRadius: radius.pill,
    backgroundColor: colors.info,
    marginTop: -2,
  },
  pusat: {
    position: "absolute",
    top: 0,
    right: 0,
    bottom: 0,
    left: 0,
    alignItems: "center",
    justifyContent: "center",
  },
});

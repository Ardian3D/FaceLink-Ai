import React, { useEffect, useState } from "react";
import { Animated, Easing, Platform, StyleSheet, View, useWindowDimensions } from "react-native";
import { LinearGradient } from "expo-linear-gradient";

import { colors } from "@/theme";

/**
 * Latar animasi yang digambar di kode untuk layar masuk.
 *
 * Dipakai sebagai pengganti klip video pada mesin yang tidak memiliki berkas
 * video (lihat `assets/video/BACA-SAYA.md`), dan sebagai cadangan bila klip
 * gagal dimuat. Nada visualnya sengaja mengikuti klipnya: hitam pekat, satu
 * sorot merah brand, dan satu garis pindai yang bergerak naik-turun — bukan
 * bidang warna datar yang terbaca sebagai layar gagal.
 *
 * Animasi memakai `Animated` bawaan React Native dan `useNativeDriver`, jadi
 * tidak menambah dependensi dan tidak membebani JS thread. Gerakannya lambat
 * dan kontrasnya rendah supaya teks di atasnya tetap terbaca.
 */
export function ScanBackdrop() {
  const { height } = useWindowDimensions();

  // Nilai animasi disimpan lewat `useState` dengan penginisialisasi malas,
  // bukan `useRef`: nilainya stabil sepanjang umur komponen, tetapi tidak
  // dibaca sebagai ref saat render.
  const [sapuan] = useState(() => new Animated.Value(0));

  useEffect(() => {
    // Web tidak memiliki modul animasi native; memaksakan driver native di sana
    // membuat `Animated.loop` berhenti setelah satu putaran. Pola ini sama
    // dengan `ScanIndicator`: driver native hanya di Android/iOS.
    const animasi = Animated.loop(
      Animated.timing(sapuan, {
        toValue: 1,
        duration: 5200,
        easing: Easing.inOut(Easing.quad),
        useNativeDriver: Platform.OS !== "web",
      })
    );

    animasi.start();
    return () => animasi.stop();
  }, [sapuan]);

  // Garis pindai bergerak dari seperempat atas ke tiga perempat bawah, lalu
  // diletakkan kembali ke titik awal oleh `Animated.loop`.
  const geser = sapuan.interpolate({
    inputRange: [0, 1],
    outputRange: [-height * 0.25, height * 0.75],
  });
  // Muncul dan menghilang di tepi supaya tidak ada lompatan yang terlihat.
  const tampak = sapuan.interpolate({
    inputRange: [0, 0.12, 0.88, 1],
    outputRange: [0, 1, 1, 0],
  });

  return (
    <View
      style={StyleSheet.absoluteFill}
      pointerEvents="none"
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
    >
      <LinearGradient
        colors={[colors.background, colors.backgroundAccent, colors.background]}
        start={{ x: 0.15, y: 0 }}
        end={{ x: 0.85, y: 1 }}
        style={StyleSheet.absoluteFill}
      />

      <Animated.View
        style={[styles.pindai, { opacity: tampak, transform: [{ translateY: geser }] }]}
      >
        <LinearGradient
          colors={[colors.glowFade, colors.glowSoft, colors.glowStrong, colors.glowFade]}
          start={{ x: 0.5, y: 0 }}
          end={{ x: 0.5, y: 1 }}
          style={styles.halo}
        />
        <View style={styles.garis} />
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  pindai: {
    position: "absolute",
    left: 0,
    right: 0,
    top: "-8%",
    alignItems: "center",
  },
  halo: {
    position: "absolute",
    left: 0,
    right: 0,
    height: 96,
  },
  garis: {
    height: StyleSheet.hairlineWidth * 2,
    width: "100%",
    backgroundColor: colors.accent,
  },
});

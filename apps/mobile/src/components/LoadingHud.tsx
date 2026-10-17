import React, { useEffect, useState } from "react";
import { Animated, Easing, Platform, StyleSheet, View } from "react-native";
import Svg, { Circle, G, Line, Path } from "react-native-svg";

import { colors, layout } from "@/theme";

/**
 * Cincin pemindaian untuk layar pemeriksaan peran.
 *
 * Seluruh geometri digambar dengan SVG, bukan gambar: cincin, tick, bracket,
 * dan busur merah tetap tajam pada semua kerapatan layar dan diskalakan
 * mengikuti ukuran cincin. Satu-satunya aset raster adalah kepala wireframe
 * (`assets/branding/loading-head.png`), yang berasal dari mockup pemilik
 * project.
 *
 * Ukuran diambil dari mockup "FaceLink Ai Loading Page" (layar 712 px):
 *
 * | Bagian | Mockup | Di sini |
 * |---|---|---|
 * | diameter cincin | 587 px (82% lebar layar) | `size` |
 * | kepala, lebar | 66% diameter | `0.66 × size` |
 * | kepala, tinggi | 74% diameter | `0.74 × size` |
 * | bracket | 4 sudut, L membulat | `SUDUT_BRACKET` |
 * | busur merah | jam 9 dan jam 3 | `BUSUR_MERAH` |
 * | garis pindai | sepertiga atas | animasi naik-turun |
 *
 * Animasi ini murni visual: ia **tidak** menandakan bahwa identitas sedang
 * diverifikasi, sehingga tidak boleh dipakai sebagai bukti status autentikasi.
 */

/**
 * Bracket sudut: L membulat di keempat sudut kotak yang dilingkupi cincin.
 * Nilainya adalah tanda sumbu (x, y) untuk tiap sudut.
 */
const SUDUT_BRACKET: readonly (readonly [number, number])[] = [
  [1, -1],
  [1, 1],
  [-1, 1],
  [-1, -1],
];

/**
 * Busur merah pada cincin. Mockup menaruh busur panjang di kiri dan kanan,
 * serta penanda pendek di atas dan bawah.
 */
const BUSUR_MERAH = [
  { mulai: -16, akhir: 16 },
  { mulai: 164, akhir: 196 },
  { mulai: 84, akhir: 96 },
  { mulai: 264, akhir: 276 },
];

/** Titik-titik kecil di sisi kanan atas. */
const TITIK = { mulai: -84, akhir: -34, jumlah: 12 };

function polar(cx: number, cy: number, r: number, derajat: number) {
  const rad = (derajat * Math.PI) / 180;
  return { x: cx + r * Math.cos(rad), y: cy + r * Math.sin(rad) };
}

/** Busur lingkaran sebagai `d` path SVG. */
function busur(cx: number, cy: number, r: number, mulai: number, akhir: number) {
  const a = polar(cx, cy, r, mulai);
  const b = polar(cx, cy, r, akhir);
  const besar = Math.abs(akhir - mulai) > 180 ? 1 : 0;
  return `M ${a.x} ${a.y} A ${r} ${r} 0 ${besar} 1 ${b.x} ${b.y}`;
}

export function LoadingHud({ size = layout.loadingHudSize }: { size?: number }) {
  const [sapuan] = useState(() => new Animated.Value(0));

  useEffect(() => {
    // Web tidak memiliki modul animasi native; memaksakan driver native di sana
    // membuat `Animated.loop` berhenti setelah satu putaran. Pola ini sama
    // dengan `ScanBackdrop`.
    const animasi = Animated.loop(
      Animated.sequence([
        Animated.timing(sapuan, {
          toValue: 1,
          duration: 2200,
          easing: Easing.inOut(Easing.sin),
          useNativeDriver: Platform.OS !== "web",
        }),
        Animated.timing(sapuan, {
          toValue: 0,
          duration: 2200,
          easing: Easing.inOut(Easing.sin),
          useNativeDriver: Platform.OS !== "web",
        }),
      ])
    );
    animasi.start();
    return () => animasi.stop();
  }, [sapuan]);

  const tengah = size / 2;
  const r = tengah - 1;

  // Garis pindai bergerak melewati wajah: dari kening ke dagu.
  const geser = sapuan.interpolate({
    inputRange: [0, 1],
    outputRange: [size * 0.26, size * 0.72],
  });

  return (
    <View
      style={{ width: size, height: size }}
      accessibilityLabel="Animasi pemeriksaan peran"
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
    >
      <Svg width={size} height={size} style={StyleSheet.absoluteFill}>
        {/* Cincin luar tipis, ditambah satu cincin dalam sebagai bingkai kepala. */}
        <Circle cx={tengah} cy={tengah} r={r} stroke={colors.hudLine} strokeWidth={1} fill="none" />
        <Circle
          cx={tengah}
          cy={tengah}
          r={r * 0.9}
          stroke={colors.hudLine}
          strokeWidth={0.75}
          fill="none"
          opacity={0.55}
        />

        {/* Tick di sekeliling cincin: tiap 4°, lebih panjang tiap 20°. */}
        <G>
          {Array.from({ length: 90 }, (_, i) => i * 4).map((derajat) => {
            const panjang = derajat % 20 === 0 ? size * 0.024 : size * 0.012;
            const luar = polar(tengah, tengah, r - size * 0.014, derajat);
            const dalam = polar(tengah, tengah, r - size * 0.014 - panjang, derajat);
            return (
              <Line
                key={derajat}
                x1={luar.x}
                y1={luar.y}
                x2={dalam.x}
                y2={dalam.y}
                stroke={colors.hudLine}
                strokeWidth={derajat % 20 === 0 ? 1.3 : 0.9}
                strokeLinecap="round"
              />
            );
          })}
        </G>

        {/* Bracket sudut: dua kaki tegak lurus dengan sudut membulat, seperti
            tanda bingkai pada mockup. */}
        <G
          stroke={colors.hudBracket}
          strokeWidth={size * 0.007}
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity={0.85}
        >
          {SUDUT_BRACKET.map(([sx, sy]) => {
            // Sudut kotak pada jarak 0.62 × jari-jari di kedua sumbu.
            const d = r * 0.62;
            const x = tengah + sx * d;
            const y = tengah + sy * d;
            const kaki = size * 0.07;
            const jalur =
              `M ${x - sx * kaki} ${y} L ${x} ${y} L ${x} ${y - sy * kaki}`;
            return <Path key={`${sx}:${sy}`} d={jalur} />;
          })}
        </G>

        {/* Busur merah di kiri dan kanan cincin. */}
        <G fill="none" strokeLinecap="round">
          {BUSUR_MERAH.map((b) => {
            const panjang = Math.abs(b.akhir - b.mulai);
            return (
              <Path
                key={b.mulai}
                d={busur(tengah, tengah, r - size * 0.004, b.mulai, b.akhir)}
                stroke={colors.accent}
                strokeWidth={panjang > 25 ? size * 0.007 : size * 0.005}
              />
            );
          })}
        </G>

        {/* Titik-titik kecil di kanan atas. */}
        <G fill={colors.hudLine}>
          {Array.from({ length: TITIK.jumlah }, (_, i) => {
            const derajat = TITIK.mulai + ((TITIK.akhir - TITIK.mulai) * i) / (TITIK.jumlah - 1);
            const p = polar(tengah, tengah, r - size * 0.038, derajat);
            return <Circle key={i} cx={p.x} cy={p.y} r={size * 0.0035} />;
          })}
        </G>
      </Svg>

      {/* Kepala wireframe: 66% lebar dan 74% tinggi cincin, sesuai mockup. */}
      <View style={[StyleSheet.absoluteFill, styles.tengah]} pointerEvents="none">
        <Animated.Image
          source={require("../../assets/branding/loading-head.png")}
          style={{ width: size * 0.66, height: size * 0.74 }}
          resizeMode="contain"
        />
      </View>

      {/* Garis pindai: garis merah tipis dengan halo lembut di atas dan bawah. */}
      <Animated.View
        style={[styles.pindai, { width: size * 0.7, transform: [{ translateY: geser }] }]}
        pointerEvents="none"
      >
        <View style={[styles.halo, { height: size * 0.045, backgroundColor: colors.hudScanGlow }]} />
        <View style={[styles.garis, { backgroundColor: colors.accent }]} />
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  tengah: {
    alignItems: "center",
    justifyContent: "center",
  },
  pindai: {
    position: "absolute",
    left: "15%",
    alignItems: "center",
    justifyContent: "center",
  },
  halo: {
    position: "absolute",
    left: 0,
    right: 0,
  },
  garis: {
    height: 1,
    width: "100%",
  },
});

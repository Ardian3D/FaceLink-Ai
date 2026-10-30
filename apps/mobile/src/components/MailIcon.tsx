import React from "react";
import Svg, { Path } from "react-native-svg";

/**
 * Ikon amplop untuk tombol masuk Email OTP.
 *
 * Digambar sebagai SVG, bukan glyph font, supaya bentuk dan ketebalannya sama
 * pada semua kerapatan layar dan tidak bergantung pada font ikon yang dimuat.
 * Warna diberikan pemanggil agar ikon mengikuti warna teks tombol.
 */
export function MailIcon({ size = 22, color }: { size?: number; color: string }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Path
        d="M3.75 6.5A2.25 2.25 0 0 1 6 4.25h12a2.25 2.25 0 0 1 2.25 2.25v11A2.25 2.25 0 0 1 18 19.75H6a2.25 2.25 0 0 1-2.25-2.25v-11Z"
        stroke={color}
        strokeWidth={1.9}
        strokeLinejoin="round"
      />
      <Path
        d="m4.4 6.9 6.55 5.05a1.75 1.75 0 0 0 2.1 0L19.6 6.9"
        stroke={color}
        strokeWidth={1.9}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Svg>
  );
}

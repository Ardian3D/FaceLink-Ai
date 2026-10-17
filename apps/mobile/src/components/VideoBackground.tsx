import React, { useEffect, useRef, useState } from "react";
import { StyleSheet, View } from "react-native";
import { useEventListener } from "expo";
import { useVideoPlayer, VideoView, type VideoSource } from "expo-video";

import { ScanBackdrop } from "./ScanBackdrop";
import { localVideoClips } from "../../assets/video/video-clips";

/**
 * Latar video layar masuk.
 *
 * Dua klip diputar bergantian tanpa jeda: ketika satu klip habis, klip
 * berikutnya menggantikannya, sehingga latarnya tampak sebagai satu loop
 * panjang (permintaan FL-UI01: pakai kedua klip).
 *
 * Berkas klip **tidak di-commit** karena memuat wajah orang yang dapat
 * dikenali — lihat `assets/video/BACA-SAYA.md`. Daftar klip karena itu datang
 * dari modul yang dipilih saat bundel dibuat (`metro.config.js`):
 *
 * - Klip ada di mesin ini → video diputar.
 * - Klip tidak ada (klon baru, CI, laptop lain) → `ScanBackdrop`, latar
 *   animasi yang digambar di kode. Bukan kotak hitam kosong, bukan galat.
 * - Klip ada tetapi gagal dimuat (codec tidak didukung, berkas rusak) →
 *   `ScanBackdrop` juga.
 *
 * Video selalu dibisukan: ini latar visual, bukan media yang ditonton, dan
 * suara otomatis pada layar masuk akan mengganggu serta melanggar kebiasaan
 * platform.
 */
export function VideoBackground() {
  if (localVideoClips.length === 0) return <ScanBackdrop />;
  return <KlipBerulang klip={localVideoClips} />;
}

/** Pemutar klip yang berganti otomatis saat klip sebelumnya habis. */
function KlipBerulang({ klip }: { klip: VideoSource[] }) {
  const [indeks, setIndeks] = useState(0);
  const [gagal, setGagal] = useState(false);
  const pertama = useRef(true);

  const player = useVideoPlayer(klip[0] ?? null, (p) => {
    p.muted = true;
    p.loop = false;
    p.play();
  });

  useEventListener(player, "statusChange", ({ status }) => {
    if (status === "error") setGagal(true);
  });

  useEventListener(player, "playToEnd", () => {
    setIndeks((sebelumnya) => (sebelumnya + 1) % klip.length);
  });

  useEffect(() => {
    // `play()` di dalam setup `useVideoPlayer` berjalan sebelum `VideoView`
    // ter-mount. Pada web, pemutar hanya meneruskan `play()` ke elemen <video>
    // yang sudah ter-mount, sehingga latar akan diam tanpa panggilan ini.
    // Pada native panggilan kedua tidak berbahaya.
    player.play();
  }, [player]);

  useEffect(() => {
    // Putaran pertama sudah ditangani `useVideoPlayer`; menggantinya lagi akan
    // memuat ulang klip yang sama dan menimbulkan kedipan.
    if (pertama.current) {
      pertama.current = false;
      return;
    }
    player.replace(klip[indeks] ?? null);
    player.play();
  }, [indeks, klip, player]);

  if (gagal) return <ScanBackdrop />;

  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      <VideoView
        player={player}
        style={StyleSheet.absoluteFill}
        contentFit="cover"
        nativeControls={false}
        playsInline
        // Video ini lapisan paling bawah, tetapi di atasnya ada lapisan gelap,
        // logo, dan tombol. `textureView` dipilih supaya video ikut dikomposisi
        // bersama view lain (SurfaceView dapat menembus lapisan di atasnya pada
        // sebagian perangkat Android).
        surfaceType="textureView"
        fullscreenOptions={{ enable: false }}
        allowsPictureInPicture={false}
      />
    </View>
  );
}

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Animated, Platform, StyleSheet, View } from "react-native";
import { useEventListener } from "expo";
import { useVideoPlayer, VideoView, type VideoPlayer, type VideoSource } from "expo-video";

import { ScanBackdrop } from "./ScanBackdrop";
import { localVideoClips } from "../../assets/video/video-clips";

/**
 * Latar video layar masuk.
 *
 * **Dua lapisan video, bukan satu.** Versi pertama memakai satu `VideoView` dan
 * memanggil `player.replace()` setiap kali klip habis. Cara itu selalu
 * meninggalkan layar hitam: permukaan video dikosongkan saat sumber baru
 * dimuat, dan pengguna melihat latar hitam sepersekian detik di setiap
 * pergantian. Sekarang kedua klip tinggal di dua pemutar terpisah yang
 * sama-sama sudah termuat:
 *
 * - Lapisan aktif diputar; lapisan lain berhenti di posisi 0, sudah siap.
 * - Saat lapisan aktif mendekati akhir (dipantau lewat `timeUpdate`), lapisan
 *   lain mulai diputar dari awal dan kedua lapisan saling menyeberang
 *   (crossfade) selama `DURASI_SEBERANG` milidetik.
 * - Setelah seberangan selesai, lapisan lama dihentikan dan dikembalikan ke
 *   posisi 0 — tersembunyi, jadi tidak terlihat.
 *
 * Hasilnya tidak ada pemuatan di tengah pergantian dan tidak ada layar hitam,
 * baik dari klip 1 ke klip 2 maupun sebaliknya. Kalau `timeUpdate` terlewat
 * (mis. tab di latar belakang), `playToEnd` menjadi jaring pengaman: pergantian
 * tetap terjadi, hanya tanpa seberangan.
 *
 * Berkas klip **tidak di-commit** karena memuat wajah orang yang dapat
 * dikenali — lihat `assets/video/BACA-SAYA.md`. Daftar klip datang dari modul
 * yang dipilih saat bundel dibuat (`metro.config.js`):
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

/** Lama seberangan antar-klip. Cukup halus, cukup singkat untuk 8 detik klip. */
const DURASI_SEBERANG_MS = 450;

/** Selang pembaruan posisi pemutaran; menentukan ketepatan awal seberangan. */
const SELANG_PANTAU_DETIK = 0.2;

export function VideoBackground() {
  if (localVideoClips.length === 0) return <ScanBackdrop />;
  return <KlipBerulang klip={localVideoClips} />;
}

/** Dua lapisan video yang saling menyeberang tanpa jeda. */
function KlipBerulang({ klip }: { klip: VideoSource[] }) {
  const [gagal, setGagal] = useState(false);
  /** Frame pertama sudah tergambar; latar animasi cadangan boleh dilepas. */
  const [siap, setSiap] = useState(false);
  const [aktif, setAktif] = useState<0 | 1>(0);

  // Nilai animasi disimpan sebagai state (bukan ref) karena dibaca saat render
  // untuk membentuk gaya; `useRef` ditolak aturan `react-hooks/refs`.
  const [opasitas] = useState(() => [new Animated.Value(1), new Animated.Value(0)]);

  /** Mencegah dua pergantian terjadi bersamaan dari `timeUpdate` dan `playToEnd`. */
  const sedangBeralih = useRef(false);

  const siapkan = useCallback(
    (p: VideoPlayer) => {
      p.muted = true;
      p.loop = klip.length < 2;
      p.timeUpdateEventInterval = SELANG_PANTAU_DETIK;
    },
    [klip.length]
  );

  // Tiap lapisan memegang klipnya sendiri dan **tidak pernah** diganti sumbernya
  // selama aplikasi berjalan. Itu sebabnya tidak ada pemuatan di tengah
  // pergantian. Modul klip menyediakan paling banyak dua berkas; bila nanti ada
  // klip ketiga, ia butuh lapisan ketiga, bukan `replace()`.
  const pemain0 = useVideoPlayer(klip[0] ?? null, siapkan);
  const pemain1 = useVideoPlayer(klip[1] ?? null, siapkan);
  const pemain = useMemo<readonly [VideoPlayer, VideoPlayer]>(
    () => [pemain0, pemain1],
    [pemain0, pemain1]
  );

  const pemilik = useRef<readonly [VideoPlayer, VideoPlayer] | null>(null);
  const animasiTransisi = useRef<Animated.CompositeAnimation | null>(null);
  useEffect(() => {
    pemilik.current = pemain;
    return () => {
      // Expo melepas player setelah cleanup efek. Batalkan kepemilikan lebih
      // dulu: stop() maupun completion native yang sudah antre bisa memanggil
      // callback animasi, tetapi tidak boleh lagi menyentuh player.
      pemilik.current = null;
      const animasi = animasiTransisi.current;
      animasiTransisi.current = null;
      animasi?.stop();
      sedangBeralih.current = false;
    };
  }, [pemain]);

  const mulaiTransisi = useCallback(
    (dari: 0 | 1) => {
      if (pemilik.current !== pemain || klip.length < 2 || sedangBeralih.current) return;

      const ke: 0 | 1 = dari === 0 ? 1 : 0;
      const masuk = pemain[ke];
      const keluar = pemain[dari];
      if (!masuk || !keluar) return;

      sedangBeralih.current = true;

      // Lapisan tujuan sudah termuat sejak awal; cukup kembalikan ke posisi
      // awal dan putar. Tidak ada `replace()` di sini — itulah yang dulu
      // menyebabkan layar hitam.
      putarDariAwal(masuk);

      const animasi = Animated.parallel([
        Animated.timing(opasitas[dari]!, {
          toValue: 0,
          duration: DURASI_SEBERANG_MS,
          useNativeDriver: Platform.OS !== "web",
        }),
        Animated.timing(opasitas[ke]!, {
          toValue: 1,
          duration: DURASI_SEBERANG_MS,
          useNativeDriver: Platform.OS !== "web",
        }),
      ]);
      animasiTransisi.current = animasi;
      animasi.start(({ finished }) => {
        if (pemilik.current !== pemain || animasiTransisi.current !== animasi) return;
        animasiTransisi.current = null;
        if (finished) {
          istirahatkan(keluar);
          setAktif(ke);
        }
        sedangBeralih.current = false;
      });
    },
    [klip.length, opasitas, pemain]
  );

  /** Memulai pergantian bila lapisan ini hampir habis. */
  const cekHampirHabis = useCallback(
    (lapisan: 0 | 1, waktuSekarang: number) => {
      if (pemilik.current !== pemain || lapisan !== aktif || sedangBeralih.current) return;
      const total = pemain[lapisan]?.duration ?? 0;
      if (!total || total <= 0) return;
      // Mulai seberangan sedikit sebelum akhir supaya lapisan tujuan sudah
      // tampil penuh tepat ketika klip lama selesai.
      if (waktuSekarang >= total - DURASI_SEBERANG_MS / 1000) mulaiTransisi(lapisan);
    },
    [aktif, mulaiTransisi, pemain]
  );

  useEventListener(pemain0, "timeUpdate", ({ currentTime }) => cekHampirHabis(0, currentTime));
  useEventListener(pemain1, "timeUpdate", ({ currentTime }) => cekHampirHabis(1, currentTime));

  // Jaring pengaman: bila `timeUpdate` terlewat (mis. aplikasi di latar
  // belakang), pergantian tetap dilakukan saat klip benar-benar habis.
  useEventListener(pemain0, "playToEnd", () => {
    if (aktif === 0) mulaiTransisi(0);
  });
  useEventListener(pemain1, "playToEnd", () => {
    if (aktif === 1) mulaiTransisi(1);
  });

  useEventListener(pemain0, "statusChange", ({ status }) => {
    if (pemilik.current === pemain && status === "error") setGagal(true);
  });
  useEventListener(pemain1, "statusChange", ({ status }) => {
    if (pemilik.current === pemain && status === "error") setGagal(true);
  });

  // Hanya saat mount: `play()` di dalam setup berjalan sebelum `VideoView`
  // ter-mount, dan pada web pemutar hanya meneruskan ke elemen <video> yang
  // sudah ter-mount. Efek ini **tidak** boleh bergantung pada `aktif`, karena
  // pergantian lapisan sudah memutar klip berikutnya sendiri — mengulanginya
  // di sini akan mengembalikan klip yang baru berjalan ke detik nol.
  const sudahMulai = useRef(false);
  useEffect(() => {
    if (sudahMulai.current) return;
    sudahMulai.current = true;
    putarDariAwal(pemain[0]);
  }, [pemain]);

  if (gagal) return <ScanBackdrop />;

  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      {/* Sebelum frame pertama tampil, yang terlihat adalah latar animasi kode,
          bukan hitam. Begitu satu frame benar-benar tergambar, lapisan ini
          dilepas supaya tidak ada animasi yang berjalan sia-sia di belakang
          video. */}
      {!siap ? <ScanBackdrop /> : null}
      {([0, 1] as const).map((lapisan) => (
        <Animated.View
          key={lapisan}
          style={[StyleSheet.absoluteFill, { opacity: opasitas[lapisan]! }]}
        >
          <VideoView
            player={pemain[lapisan]}
            style={StyleSheet.absoluteFill}
            contentFit="cover"
            nativeControls={false}
            playsInline
            onFirstFrameRender={() => {
              if (pemilik.current === pemain) setSiap(true);
            }}
            // Dua video bertumpuk harus ikut dikomposisi bersama lapisan di
            // atasnya (scrim, logo, tombol). `SurfaceView` pada Android tidak
            // mendukung itu; `textureView` mendukung.
            surfaceType="textureView"
            fullscreenOptions={{ enable: false }}
            allowsPictureInPicture={false}
          />
        </Animated.View>
      ))}
    </View>
  );
}

/**
 * Memutar pemutar dari detik nol.
 *
 * Ditulis sebagai fungsi tingkat modul, bukan di dalam komponen, supaya
 * perubahan pada objek pemutar jelas terjadi di luar fase render dan tidak
 * memicu aturan `react-hooks/immutability`.
 */
function putarDariAwal(p: VideoPlayer | undefined) {
  if (!p) return;
  p.currentTime = 0;
  p.play();
}

/** Menghentikan pemutar yang sudah tidak terlihat dan mengembalikannya ke awal. */
function istirahatkan(p: VideoPlayer | undefined) {
  if (!p) return;
  p.pause();
  p.currentTime = 0;
}

import React, { useCallback, useEffect, useState } from "react";
import { StyleSheet, View } from "react-native";

import { AppButton } from "@/components/AppButton";
import { AppText } from "@/components/AppText";
import { jalankanDiagnostik, type Diagnostik, type HasilCek } from "./client";
import { konfigurasiApi } from "./config";
import { radius, spacing, useTheme } from "@/theme";

/**
 * Diagnostik koneksi Go Backend (FL-NET01).
 *
 * Hanya dirender dari `app/dev-preview.tsx`, yang dijaga `__DEV__`. Komponen
 * ini **tidak** dipakai oleh layar mana pun di alur produksi, sehingga tidak
 * ikut terlihat pada build rilis.
 *
 * Yang dikerjakan: satu permintaan nyata ke `GET /api/v1/healthz` dan satu ke
 * `GET /api/v1/readyz`, lalu hasilnya ditampilkan apa adanya. Tidak ada login,
 * tidak ada token, dan tidak ada data pengguna yang dikirim — pemeriksaan ini
 * hanya membaca status layanan.
 *
 * Tiga keadaan dibedakan dengan nada dan teks, bukan warna saja:
 * `ok` (hijau), `peringatan` (kuning), `gagal` (merah), dan "tidak terjangkau"
 * selalu diberi kalimat tindakan karena itulah yang paling sering terjadi saat
 * perangkat belum tersambung ke laptop.
 */
export function ConnectionDiagnostics() {
  const { colors } = useTheme();
  const [hasil, setHasil] = useState<Diagnostik | null>(null);
  const [galat, setGalat] = useState<string | null>(null);
  // Pemeriksaan pertama berjalan otomatis, jadi keadaan awal sudah "memeriksa".
  const [memeriksa, setMemeriksa] = useState(true);

  const terapkan = useCallback((jawaban: Diagnostik | { galat: string }) => {
    if ("galat" in jawaban) {
      setHasil(null);
      setGalat(jawaban.galat);
    } else {
      setHasil(jawaban);
    }
    setMemeriksa(false);
  }, []);

  // Satu pemeriksaan otomatis saat layar dibuka: pengguna langsung tahu
  // keadaan tanpa harus menekan apa pun, dan tombol Retry tetap tersedia.
  // Hasil diterapkan setelah `await`, bukan di badan efek, sehingga tidak ada
  // pembaruan state sinkron yang memicu render berantai.
  useEffect(() => {
    let dibatalkan = false;
    void jalankanDiagnostik().then((jawaban) => {
      if (!dibatalkan) terapkan(jawaban);
    });
    return () => {
      dibatalkan = true;
    };
  }, [terapkan]);

  const periksa = useCallback(async () => {
    setMemeriksa(true);
    setGalat(null);
    terapkan(await jalankanDiagnostik());
  }, [terapkan]);

  const warnaNada = (nada: HasilCek["nada"]) =>
    nada === "ok" ? colors.success : nada === "peringatan" ? colors.warning : colors.error;

  return (
    <View style={[styles.kotak, { borderColor: colors.border, backgroundColor: colors.surface }]}>
      <View style={styles.judulBaris}>
        <AppText variant="bodyLarge" weight="semibold">
          Koneksi backend
        </AppText>
        <View
          style={[
            styles.lencana,
            {
              borderColor: konfigurasiApi.ok ? colors.border : colors.error,
              backgroundColor: colors.surfaceMuted,
            },
          ]}
        >
          <AppText variant="micro" weight="medium" color={colors.textSecondary}>
            {konfigurasiApi.ok ? konfigurasiApi.sumber : "belum disetel"}
          </AppText>
        </View>
      </View>

      <AppText variant="caption" color={colors.textMuted}>
        {konfigurasiApi.ok
          ? konfigurasiApi.baseUrl
          : "EXPO_PUBLIC_FACELINK_API_BASE_URL belum disetel untuk build ini."}
      </AppText>

      {galat ? (
        <AppText variant="caption" color={colors.error}>
          {galat}
        </AppText>
      ) : null}

      {hasil ? (
        <>
          <BarisHasil judul="healthz" hasil={hasil.health} warna={warnaNada(hasil.health.nada)} />
          <BarisHasil judul="readyz" hasil={hasil.ready} warna={warnaNada(hasil.ready.nada)} />
          <AppText variant="micro" color={colors.textMuted}>
            Diperiksa {hasil.pada} · health {hasil.health.durasiMs} ms · ready {hasil.ready.durasiMs} ms
          </AppText>
        </>
      ) : null}

      {memeriksa && !hasil ? (
        <AppText variant="caption" color={colors.textMuted}>
          Memeriksa…
        </AppText>
      ) : null}

      <AppButton
        label={memeriksa ? "Memeriksa…" : "Periksa ulang"}
        variant="secondary"
        loading={memeriksa}
        onPress={() => void periksa()}
      />
    </View>
  );
}

function BarisHasil({
  judul,
  hasil,
  warna,
}: {
  judul: string;
  hasil: HasilCek;
  warna: string;
}) {
  const { colors } = useTheme();

  return (
    <View style={styles.hasil}>
      <View style={styles.hasilKepala}>
        <View style={[styles.titik, { backgroundColor: warna }]} />
        <AppText variant="body" weight="semibold" style={styles.hasilJudul}>
          {judul}
        </AppText>
        <AppText variant="caption" color={warna} weight="medium">
          {hasil.terjangkau ? "terjangkau" : "tidak terjangkau"}
        </AppText>
      </View>
      <AppText variant="caption" color={colors.textSecondary}>
        {hasil.judul}
      </AppText>
      {hasil.baris.map((b) => (
        <AppText key={b} variant="caption" color={colors.textMuted}>
          {b}
        </AppText>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  kotak: {
    gap: spacing.md,
    borderRadius: radius.lg,
    borderWidth: StyleSheet.hairlineWidth,
    padding: spacing.lg,
  },
  judulBaris: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: spacing.sm,
  },
  lencana: {
    borderRadius: radius.pill,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
  },
  hasil: {
    gap: spacing.xs,
  },
  hasilKepala: {
    flexDirection: "row",
    alignItems: "center",
    gap: spacing.sm,
  },
  hasilJudul: {
    flex: 1,
  },
  titik: {
    width: 8,
    height: 8,
    borderRadius: radius.pill,
  },
});

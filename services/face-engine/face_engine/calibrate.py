"""Alat pengukuran lapangan FL-CV02A.

Menjalankan satu sesi kalibrasi dari terminal:

    python -m face_engine.calibrate --peserta 5 --sampel 5 --kueri 3

Alur yang dipaksakan alat ini, berurutan:

  1. Penjelasan dibacakan dan persetujuan dicatat per peserta (mengetik `ya`).
  2. Enrollment: beberapa sampel per peserta, dengan penilaian kualitas.
  3. Pengambilan kueri per kondisi (terang, redup, miring, kacamata).
  4. Wajah tak terdaftar diambil dari peserta yang **menyetujui tetapi tidak
     dienroll** — bukan dari orang yang lewat di depan kamera.
  5. Analisis: sapuan ambang, distribusi skor, margin, peringkat 1:N, korelasi
     kualitas, false match/rejection, latensi.
  6. Sesi diakhiri; seluruh vektor dibuang dari memori.

Tidak ada berkas yang ditulis. Laporan dicetak ke stdout; hanya angka agregat
yang boleh disalin manual ke `docs/CALIBRATION_FL_CV02A.md`.
"""
from __future__ import annotations

import argparse
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path

import cv2
import numpy as np

from .calibration import PENJELASAN_PERSETUJUAN, SesiKalibrasi
from .camera import Kamera
from .config import DEFAULT_MODEL_DIR, Konfigurasi, MATCH_MARGIN, MATCH_THRESHOLD
from .detector import Detektor
from .embedder import Embedder
from .errors import GalatKamera, GalatMesinWajah, GalatModel
from .evaluation import (
    AMBANG_KANDIDAT,
    bagi_kueri,
    distribusi_genuine,
    distribusi_impostor,
    distribusi_margin,
    format_distribusi,
    format_tabel,
    kelompok_kualitas,
    korelasi_kualitas,
    nilai_semua,
    pilih_ambang,
    ringkas,
    ringkas_distribusi,
    ringkas_peringkat,
    sapuan_ambang,
)
from .gallery import Galeri
from .quality import kecerahan, ketajaman, nilai_frame

#: Kondisi yang diambil pada setiap sesi. `miring` berarti peserta diminta
#: menoleh sekitar 20–30 derajat; `kacamata` hanya bila peserta memakainya.
KONDISI_SESI = ("terang", "redup", "miring")
TOMBOL_AMBIL = 32  # spasi
TOMBOL_LEWATI = ord("n")
TOMBOL_KELUAR = ord("q")
JUDUL = "FaceLink AI - FL-CV02A (spasi ambil, n lewati, q batal)"


@dataclass(frozen=True)
class Tangkapan:
    """Satu sampel yang diterima: embedding, durasi, dan metrik kualitas."""

    vektor: np.ndarray
    durasi_deteksi_ms: float
    durasi_embedding_ms: float
    ketajaman: float
    kecerahan: float


@dataclass
class Penolakan:
    """Hitungan frame yang ditolak pemeriksaan kualitas, per kode alasan.

    Diperlukan karena matriks `docs/FACE_ENGINE.md` §7 menuntut jumlah
    keberhasilan penolakan (tanpa wajah, lebih dari satu wajah, buram) — angka
    itu tidak dapat diambil dari galeri atau kueri, sebab frame yang ditolak
    memang tidak pernah masuk ke sana. Hitungannya hanya angka; tidak ada
    potongan gambar yang disimpan.
    """

    per_tahap: dict[str, dict[str, int]] = field(default_factory=dict)

    def catat(self, tahap: str, kode: str | None) -> None:
        if kode is None:
            return
        tahap_hitungan = self.per_tahap.setdefault(tahap, {})
        tahap_hitungan[kode] = tahap_hitungan.get(kode, 0) + 1

    @property
    def total(self) -> int:
        return sum(sum(hitungan.values()) for hitungan in self.per_tahap.values())

    def ringkas(self) -> dict[str, dict[str, int]]:
        return {
            tahap: dict(sorted(hitungan.items()))
            for tahap, hitungan in sorted(self.per_tahap.items())
        }


# --------------------------------------------------------------------- input


def tanya(prompt: str, bawaan: str = "") -> str:
    jawab = input(prompt).strip()
    return jawab if jawab else bawaan


def tanya_ya(prompt: str) -> bool:
    return tanya(prompt, "tidak").lower() in {"ya", "y", "yes"}

# ------------------------------------------------------------------ tangkap


def tangkap_satu(
    kamera: Kamera,
    detektor: Detektor,
    embedder: Embedder,
    judul_panel: str,
    petunjuk: str,
    penolakan: Penolakan | None = None,
) -> Tangkapan | None:
    """Menunggu satu frame layak, lalu mengembalikan sampel dan metriknya.

    Kembalian `None` berarti pengguna melewati langkah ini. Penolakan kualitas
    ditampilkan di panel, bukan dilempar sebagai galat: peserta perlu melihat
    apa yang salah dan memperbaikinya tanpa memulai ulang sesi. Bila
    `penolakan` diberikan, satu **episode** penolakan dihitung sekali — yaitu
    rangkaian frame berurutan dengan kode alasan yang sama. Tanpa aturan ini
    satu wajah yang hilang selama dua detik akan tercatat 60 kali.
    """
    kode_terakhir: str | None = None
    while True:
        frame = kamera.baca()
        t0 = time.perf_counter()
        deteksi = detektor.deteksi(frame)
        ms_deteksi = (time.perf_counter() - t0) * 1000

        penilaian = nilai_frame(frame, deteksi)
        if penilaian.diterima:
            kode_terakhir = None
        elif penilaian.kode != kode_terakhir:
            kode_terakhir = penilaian.kode
            if penolakan is not None:
                penolakan.catat(judul_panel, penilaian.kode)
        kanvas = frame.copy()

        warna = (80, 200, 80) if penilaian.diterima else (60, 200, 240)
        for wajah in deteksi:
            x, y, w, h = wajah.kotak
            cv2.rectangle(kanvas, (x, y), (x + w, y + h), warna, 2)

        baris = [petunjuk, penilaian.pesan or "Siap; tekan spasi untuk mengambil."]
        if penilaian.diterima:
            baris.append(
                f"ketajaman {penilaian.rincian['ketajaman']:.0f}  "
                f"kecerahan {penilaian.rincian['kecerahan']:.0f}"
            )
        _, lebar = kanvas.shape[:2]
        cv2.rectangle(kanvas, (0, 0), (lebar, 96), (25, 25, 25), -1)
        for i, teks in enumerate(baris):
            cv2.putText(
                kanvas,
                teks,
                (14, 28 + i * 26),
                cv2.FONT_HERSHEY_SIMPLEX,
                0.6,
                warna if i == 1 else (220, 220, 220),
                1,
                cv2.LINE_AA,
            )
        cv2.imshow(JUDUL, kanvas)
        tombol = cv2.waitKey(1) & 0xFF

        if tombol == TOMBOL_KELUAR:
            raise KeyboardInterrupt
        if tombol == TOMBOL_LEWATI:
            return None
        if tombol == TOMBOL_AMBIL and penilaian.diterima:
            t1 = time.perf_counter()
            vektor = embedder.embedding(frame, deteksi[0])
            ms_embed = (time.perf_counter() - t1) * 1000
            return Tangkapan(
                vektor=vektor,
                durasi_deteksi_ms=ms_deteksi,
                durasi_embedding_ms=ms_embed,
                # Metrik kualitas dihitung pada frame yang sama dengan
                # embedding-nya, supaya korelasi skor-kualitas bermakna.
                ketajaman=ketajaman(frame, deteksi[0]),
                kecerahan=kecerahan(frame, deteksi[0]),
            )


def ambil_enrollment(
    kamera: Kamera,
    detektor: Detektor,
    embedder: Embedder,
    galeri: Galeri,
    label: str,
    jumlah: int,
    penolakan: Penolakan | None = None,
) -> int:
    """Mengumpulkan `jumlah` sampel untuk satu label; mengembalikan yang berhasil."""
    if label not in galeri:
        galeri.tambah_identitas(label)
    terkumpul = 0
    while terkumpul < jumlah:
        hasil = tangkap_satu(
            kamera,
            detektor,
            embedder,
            judul_panel="Enrollment",
            petunjuk=f"{label} — sampel {terkumpul + 1}/{jumlah}. Hadapkan wajah lurus.",
            penolakan=penolakan,
        )
        if hasil is None:
            break
        galeri.tambah_sampel(label, hasil.vektor)
        terkumpul += 1
    return terkumpul


def ambil_kueri(
    kamera: Kamera,
    detektor: Detektor,
    embedder: Embedder,
    sesi: SesiKalibrasi,
    label_benar: str | None,
    kondisi: str,
    jumlah: int,
    penolakan: Penolakan | None = None,
) -> int:
    """Mengambil kueri untuk satu label; `label_benar=None` berarti tak terdaftar."""
    tampil = label_benar or "TAK-TERDAFTAR"
    terkumpul = 0
    while terkumpul < jumlah:
        hasil = tangkap_satu(
            kamera,
            detektor,
            embedder,
            judul_panel=f"Kueri: {kondisi}",
            petunjuk=f"{tampil} — {kondisi} {terkumpul + 1}/{jumlah}.",
            penolakan=penolakan,
        )
        if hasil is None:
            break
        sesi.tambah_kueri(
            label_benar,
            hasil.vektor,
            kondisi,
            hasil.durasi_deteksi_ms,
            hasil.durasi_embedding_ms,
            hasil.ketajaman,
            hasil.kecerahan,
        )
        terkumpul += 1
    return terkumpul


# ----------------------------------------------------------------- analisis


def _cetak_judul(teks: str) -> None:
    print("\n" + "-" * 78)
    print(teks)
    print("-" * 78)


def cetak_laporan(
    sesi: SesiKalibrasi,
    galeri: Galeri,
    margin: float,
    penolakan: Penolakan | None = None,
) -> None:
    """Mencetak seluruh angka. Tidak menulis berkas."""
    if not sesi.kueri:
        print("\nTidak ada kueri; tidak ada yang dapat dianalisis.")
        return
    hasil = nilai_semua(galeri, sesi.kueri)
    kalibrasi, evaluasi = bagi_kueri(hasil)
    disetujui = [p for p in sesi.peserta.values() if p.persetujuan_dicatat]
    berkacamata = [p for p in disetujui if p.kacamata]

    print("\n" + "=" * 78)
    print("RINGKASAN SESI")
    print("=" * 78)
    print(f"sesi             : {sesi.nama_sesi}")
    print(f"peserta          : {len(sesi.peserta)} (persetujuan tercatat: {len(disetujui)})")
    print(f"berkacamata      : {len(berkacamata)}")
    print(f"identitas galeri : {len(galeri)}")
    print(f"kueri            : {len(sesi.kueri)} "
          f"(kalibrasi {len(kalibrasi)}, evaluasi {len(evaluasi)})")
    print(f"per kondisi      : {sesi.jumlah_per_kondisi()}")
    print(f"per peserta      : {sesi.jumlah_per_peserta()}")
    print("\nPemisahan kalibrasi/evaluasi hanya bermakna bila setiap kondisi punya "
          "cukup sampel; pada jumlah kecil ini hasilnya indikasi, bukan estimasi.")

    _cetak_judul("DISTRIBUSI SKOR (data evaluasi saja)")
    print(format_distribusi("genuine", distribusi_genuine(evaluasi)))
    print(format_distribusi("impostor", distribusi_impostor(evaluasi)))
    print(format_distribusi("margin", distribusi_margin(evaluasi)))
    peringkat = ringkas_peringkat(evaluasi)
    if peringkat["n"]:
        print(f"peringkat 1:N    : n={peringkat['n']} top-1={peringkat['top1']} "
              f"top-2={peringkat['top2']} median={peringkat['median']:.2f}")
    else:
        print("peringkat 1:N    : (tidak ada data)")

    _cetak_judul(f"SAPUAN AMBANG — data kalibrasi (margin {margin})")
    print(format_tabel(sapuan_ambang(kalibrasi, AMBANG_KANDIDAT, margin)))
    _cetak_judul(f"SAPUAN AMBANG — data evaluasi (margin {margin})")
    print(format_tabel(sapuan_ambang(evaluasi, AMBANG_KANDIDAT, margin)))
    print("\nFR = false rejection, FM = false match, unknown = wajah tak dikenal "
          "yang diterima, ambigu = perlu verifikasi.")

    pilihan = pilih_ambang(kalibrasi, AMBANG_KANDIDAT, margin)
    if pilihan is None:
        print("\nTidak ada ambang yang memenuhi FRR <= 0,10, nol penerimaan wajah "
              "tak dikenal, dan nol false match pada data kalibrasi.")
        print("Itu bukan alasan menurunkan standar; Q-07 tetap terbuka.")
    else:
        terapan = ringkas(evaluasi, pilihan.ambang, margin)
        print(f"\nAmbang terpilih dari data kalibrasi: {pilihan.ambang:.3f} "
              f"(kandidat, BUKAN ambang produksi)")
        print(f"  pada data evaluasi: FR {terapan.false_rejection}/{terapan.genuine_total}, "
              f"FM {terapan.false_match}/{terapan.genuine_total}, "
              f"unknown diterima {terapan.unknown_diterima}/{terapan.unknown_total}, "
              f"ambigu {terapan.ambigu}/{terapan.genuine_total}")

    _cetak_judul(f"PER KONDISI (ambang baseline {MATCH_THRESHOLD:g}, margin {margin})")
    baseline = ringkas(evaluasi, MATCH_THRESHOLD, margin)
    for kondisi, r in sorted(baseline.per_kondisi.items()):
        median = r.median_skor
        teks_median = f"{median:.3f}" if median is not None else "-"
        print(f"{kondisi:<16} n={r.genuine:<3d} benar={r.benar:<3d} "
              f"FR={r.false_rejection:<3d} FM={r.false_match:<3d} "
              f"ambigu={r.ambigu:<3d} median_skor={teks_median}")

    _cetak_judul("PENGARUH KUALITAS FRAME (data evaluasi, percobaan genuine)")
    for metrik in ("ketajaman", "kecerahan"):
        k = korelasi_kualitas(evaluasi, metrik)
        rho = f"{k['rho']:.3f}" if k["rho"] is not None else "-"
        print(f"korelasi Spearman {metrik:<10}: n={k['n']:<3d} rho={rho}")
        for kelompok in kelompok_kualitas(evaluasi, metrik, jumlah=3):
            print(f"  {metrik:<10} {kelompok['min_metrik']:7.1f}–{kelompok['maks_metrik']:<7.1f} "
                  f"n={kelompok['n']:<3d} median_skor={kelompok['median_skor']:.3f}")
    print("Korelasi pada puluhan sampel hanya indikasi, bukan bukti.")

    _cetak_judul("PENOLAKAN KUALITAS (episode berurutan, bukan jumlah frame)")
    if penolakan is None or penolakan.total == 0:
        print("(tidak ada penolakan tercatat)")
    else:
        for tahap, hitungan in penolakan.ringkas().items():
            rincian = "  ".join(f"{kode}={jumlah}" for kode, jumlah in hitungan.items())
            print(f"{tahap:<28} {rincian}")

    _cetak_judul("LATENSI (seluruh kueri)")
    deteksi_nilai = [s.durasi_deteksi_ms for s in sesi.kueri if s.durasi_deteksi_ms is not None]
    embed_nilai = [s.durasi_embedding_ms for s in sesi.kueri if s.durasi_embedding_ms is not None]
    total_nilai = [s.durasi_total_ms for s in sesi.kueri if s.durasi_total_ms is not None]
    for nama, nilai in (("deteksi", deteksi_nilai), ("embedding", embed_nilai),
                        ("total", total_nilai)):
        d = ringkas_distribusi(nilai)
        if d["n"]:
            print(f"{nama:<10}: n={d['n']:<3d} min={d['min']:6.1f} ms "
                  f"median={d['median']:6.1f} ms maks={d['maks']:6.1f} ms")
        else:
            print(f"{nama:<10}: (tidak ada data)")


# ---------------------------------------------------------------------- main


def bangun_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="face_engine.calibrate",
        description="Sesi pengukuran FL-CV02A: enrollment, kueri per kondisi, analisis.",
    )
    p.add_argument("--peserta", type=int, default=5, help="jumlah peserta sukarela")
    p.add_argument("--sampel", type=int, default=5, help="sampel enrollment per peserta")
    p.add_argument("--kueri", type=int, default=3, help="kueri per peserta per kondisi")
    p.add_argument("--margin", type=float, default=MATCH_MARGIN)
    p.add_argument("--camera", type=int, default=0)
    p.add_argument("--model-dir", type=Path, default=DEFAULT_MODEL_DIR)
    p.add_argument("--nama-sesi", default="FL-CV02A")
    return p


def main(argv: list[str] | None = None) -> int:
    arg = bangun_parser().parse_args(argv)
    if arg.peserta < 2:
        print("Pengujian membutuhkan minimal dua peserta untuk mengukur impostor.",
              file=sys.stderr)
        return 2
    if arg.sampel < 1 or arg.kueri < 1:
        print("--sampel dan --kueri harus minimal 1.", file=sys.stderr)
        return 2
    konfigurasi = Konfigurasi(
        model_dir=arg.model_dir.expanduser().resolve(), camera_index=arg.camera
    )

    print(PENJELASAN_PERSETUJUAN)
    print("\nPengambilan sampel hanya dimulai setelah persetujuan dicatat.\n")
    sesi = SesiKalibrasi(nama_sesi=arg.nama_sesi)
    try:
        for i in range(arg.peserta):
            print(f"\n--- Peserta {i + 1} dari {arg.peserta} ---")
            kacamata = tanya_ya("Apakah peserta memakai kacamata? (ya/tidak) ")
            peserta = sesi.daftarkan_peserta(kacamata=kacamata)
            print(f"Label anonim: {peserta.label}")
            if not tanya_ya("Apakah peserta sudah membaca penjelasan dan setuju? (ya/tidak) "):
                print("Peserta tidak menyetujui. Sampel peserta ini tidak akan diambil.")
                continue
            sesi.catat_persetujuan(peserta.label)
    except (KeyboardInterrupt, EOFError):
        # Tanpa ini, input yang diputus meninggalkan traceback mentah; tidak ada
        # sampel yang sempat diambil pada tahap ini, jadi tidak ada yang dibuang.
        print("\nPendataan peserta dibatalkan; belum ada sampel yang diambil.",
              file=sys.stderr)
        return 130
    if not any(p.persetujuan_dicatat for p in sesi.peserta.values()):
        print("Tidak ada peserta yang menyetujui; sesi dihentikan.", file=sys.stderr)
        return 3

    try:
        detektor = Detektor(konfigurasi.yunet_path)
        embedder = Embedder(konfigurasi.sface_path)
    except GalatModel as galat:
        print(f"[MODEL] {galat}", file=sys.stderr)
        return 4

    galeri = Galeri()
    penolakan = Penolakan()
    kode_keluar = 0
    try:
        with Kamera(konfigurasi.camera_index, konfigurasi.frame_width,
                    konfigurasi.frame_height) as kamera:
            # Pemanasan: auto-exposure kamera butuh beberapa frame sebelum stabil.
            for _ in range(20):
                kamera.baca()
            time.sleep(0.8)

            print("\n=== TAHAP ENROLLMENT ===")
            for peserta in sesi.peserta.values():
                if not peserta.persetujuan_dicatat:
                    continue
                print(f"\n{peserta.label}: {arg.sampel} sampel. Spasi untuk mengambil.")
                terkumpul = ambil_enrollment(
                    kamera, detektor, embedder, galeri, peserta.label, arg.sampel,
                    penolakan,
                )
                print(f"  terkumpul {terkumpul}/{arg.sampel}")

            print("\n=== TAHAP KUERI ===")
            for kondisi in KONDISI_SESI:
                print(f"\nKondisi: {kondisi}")
                for peserta in sesi.peserta.values():
                    if not peserta.persetujuan_dicatat:
                        continue
                    if kondisi == "kacamata" and not peserta.kacamata:
                        continue
                    print(f"  {peserta.label} ({kondisi})")
                    ambil_kueri(kamera, detektor, embedder, sesi, peserta.label,
                                kondisi, arg.kueri, penolakan)
            if any(p.kacamata for p in sesi.peserta.values()):
                print("\nKondisi: tanpa-kacamata (peserta berkacamata diminta melepasnya)")
                for peserta in sesi.peserta.values():
                    if not peserta.persetujuan_dicatat or not peserta.kacamata:
                        continue
                    ambil_kueri(kamera, detektor, embedder, sesi, peserta.label,
                                "tanpa-kacamata", arg.kueri, penolakan)

            # Wajah tak terdaftar: peserta yang MENYETUJUI tetapi tidak dienroll.
            # Labelnya sengaja `None`, karena orang ini memang tidak ada di
            # galeri; memberi label peserta akan salah menghitungnya sebagai
            # percobaan genuine yang gagal.
            tidak_dienroll = [
                p for p in sesi.peserta.values()
                if p.persetujuan_dicatat and p.label not in galeri
            ]
            for peserta in tidak_dienroll:
                print(f"\nKondisi: tidak-terdaftar ({peserta.label}, tidak dienroll)")
                ambil_kueri(kamera, detektor, embedder, sesi, None,
                            "tidak-terdaftar", arg.kueri, penolakan)
    except GalatKamera as galat:
        print(f"[KAMERA] {galat}", file=sys.stderr)
        kode_keluar = 5
    except KeyboardInterrupt:
        print("\nSesi dibatalkan; seluruh data di memori dibuang.", file=sys.stderr)
        kode_keluar = 130
    except GalatMesinWajah as galat:
        print(f"[{galat.kode}] {galat}", file=sys.stderr)
        kode_keluar = 6
    finally:
        cv2.destroyAllWindows()

    if kode_keluar == 0:
        cetak_laporan(sesi, galeri, arg.margin, penolakan)
    rekap = sesi.akhiri_sesi()
    galeri.kosongkan()
    print("\nSesi diakhiri. Data di memori dibuang:", rekap["jumlah_kueri"], "kueri.")
    return kode_keluar


if __name__ == "__main__":
    raise SystemExit(main())

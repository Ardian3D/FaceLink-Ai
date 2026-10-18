"""Aplikasi preview webcam: deteksi, enrollment sementara, dan recognition.

Tiga mode berbagi satu jendela:

  ``detect``      — hanya kotak dan landmark (Tahap 1).
  ``enroll``      — kumpulkan sampel untuk satu identitas uji (Tahap 2).
  ``recognize``   — bandingkan wajah dengan galeri di RAM (Tahap 3).

Enrollment **tidak pernah** berjalan otomatis. Pengguna harus memilih mode dan
menekan tombol; orang yang lewat di depan kamera tidak akan terdaftar sendiri.
Seluruh data hilang saat jendela ditutup, karena tidak ada kode yang menulis ke
disk.
"""

from __future__ import annotations

import time
from dataclasses import dataclass

import cv2
import numpy as np

from .camera import Kamera
from .config import Konfigurasi
from .detector import Detektor, Deteksi
from .embedder import Embedder
from .gallery import Galeri
from .matcher import HasilCocok, Status, cocokkan
from .overlay import gambar_wajah, label_hasil, panel_status
from .quality import nilai_frame

#: Tombol keyboard. Huruf kecil dipakai karena `cv2.waitKey` mengembalikan kode
#: ASCII huruf yang ditekan.
TOMBOL_KELUAR = ord("q")
TOMBOL_ENROLL = ord("e")
TOMBOL_RECOGNIZE = ord("r")
TOMBOL_DETEKSI = ord("d")
TOMBOL_SIMPAN_SAMPEL = ord("s")
TOMBOL_HAPUS_GALERI = ord("c")
TOMBOL_BANTUAN = ord("h")

JUDUL_JENDELA = "FaceLink AI - FL-CV01 (q keluar)"


@dataclass
class StatusEnrollment:
    """Keadaan satu sesi pendaftaran yang sedang berjalan."""

    label: str
    terkumpul: int
    target: int
    pesan_terakhir: str = ""

    @property
    def selesai(self) -> bool:
        return self.terkumpul >= self.target


class Aplikasi:
    """Loop utama prototype."""

    def __init__(self, konfigurasi: Konfigurasi, mode_awal: str = "detect") -> None:
        self.konfigurasi = konfigurasi
        self.mode = mode_awal
        self.galeri = Galeri()
        self.enrollment: StatusEnrollment | None = None

        self.detektor = Detektor(konfigurasi.yunet_path)
        self.embedder = Embedder(konfigurasi.sface_path)

        self._fps_terakhir = 0.0
        self._waktu_lalu = time.monotonic()

    # ---------------------------------------------------------------- mode

    def mulai_enrollment(self) -> StatusEnrollment:
        """Memulai pendaftaran identitas baru. Label selalu anonim dan berurutan."""
        label = self.galeri.label_berikutnya()
        self.enrollment = StatusEnrollment(
            label=label,
            terkumpul=0,
            target=self.konfigurasi.required_samples,
            pesan_terakhir="Posisikan satu wajah di tengah bingkai, lalu tekan S.",
        )
        self.mode = "enroll"
        return self.enrollment

    def ambil_sampel(self, frame: np.ndarray, deteksi: list[Deteksi]) -> tuple[bool, str]:
        """Menilai frame dan, bila layak, menyimpan embedding ke RAM."""
        if self.enrollment is None:
            return (False, "Enrollment belum dimulai.")

        penilaian = nilai_frame(frame, deteksi)
        if not penilaian.diterima:
            self.enrollment.pesan_terakhir = penilaian.pesan
            return (False, penilaian.pesan)

        assert deteksi[0] is not None
        vektor = self.embedder.embedding(frame, deteksi[0])
        # Identitas baru masuk galeri hanya setelah ada sampel yang diterima.
        # Tanpa aturan ini, galeri dapat berisi identitas kosong yang tidak
        # pernah dipakai dan labelnya termakan percuma.
        if self.enrollment.label not in self.galeri:
            self.galeri.tambah_identitas(self.enrollment.label)
        self.galeri.tambah_sampel(self.enrollment.label, vektor)
        self.enrollment.terkumpul += 1
        self.enrollment.pesan_terakhir = (
            f"Sampel {self.enrollment.terkumpul}/{self.enrollment.target} diterima."
        )

        if self.enrollment.selesai:
            self.enrollment.pesan_terakhir = (
                f"{self.enrollment.label} terdaftar dengan "
                f"{self.enrollment.terkumpul} sampel. Tekan R untuk mengenali."
            )
            self.mode = "recognize"
        return (True, self.enrollment.pesan_terakhir)

    def cocokkan_wajah(self, frame: np.ndarray, deteksi: list[Deteksi]) -> HasilCocok | None:
        """Mencocokkan tepat satu wajah; `None` bila tidak ada atau lebih dari satu."""
        if len(deteksi) != 1:
            return None
        vektor = self.embedder.embedding(frame, deteksi[0])
        return cocokkan(
            vektor,
            self.galeri,
            ambang=self.konfigurasi.match_threshold,
            margin=self.konfigurasi.match_margin,
        )

    # ----------------------------------------------------------------- loop

    def jalankan(self) -> int:
        """Membuka kamera dan menjalankan loop sampai pengguna menekan Q."""
        with Kamera(
            self.konfigurasi.camera_index,
            self.konfigurasi.frame_width,
            self.konfigurasi.frame_height,
        ) as kamera:
            while True:
                frame = kamera.baca()
                hasil = self.proses_frame(frame)
                cv2.imshow(JUDUL_JENDELA, hasil)

                tombol = cv2.waitKey(1) & 0xFF
                if tombol == 255:
                    continue
                if tombol == TOMBOL_KELUAR:
                    break
                self.tangani_tombol(tombol, frame)

        cv2.destroyAllWindows()
        # Galeri dibersihkan secara eksplisit: bukan karena ada penyimpanan,
        # melainkan agar niat "tidak ada persistensi" terlihat di kode.
        self.galeri.kosongkan()
        return 0

    def tangani_tombol(self, tombol: int, frame: np.ndarray) -> None:
        if tombol == TOMBOL_ENROLL:
            self.mulai_enrollment()
        elif tombol == TOMBOL_RECOGNIZE:
            if len(self.galeri) == 0:
                self._pesan_sementara = "Galeri kosong; tekan E untuk mendaftarkan wajah."
            else:
                self.mode = "recognize"
        elif tombol == TOMBOL_DETEKSI:
            self.mode = "detect"
        elif tombol == TOMBOL_SIMPAN_SAMPEL and self.mode == "enroll":
            deteksi = self.detektor.deteksi(frame)
            self.ambil_sampel(frame, deteksi)
        elif tombol == TOMBOL_HAPUS_GALERI:
            self.galeri.kosongkan()
            self.enrollment = None
            self.mode = "detect"
        elif tombol == TOMBOL_BANTUAN:
            self._cetak_bantuan()

    _pesan_sementara: str = ""

    def proses_frame(self, frame: np.ndarray) -> np.ndarray:
        """Menjalankan deteksi (dan pencocokan bila perlu) untuk satu frame."""
        deteksi = self.detektor.deteksi(frame)
        kanvas = frame.copy()

        if self.mode == "recognize":
            hasil = self.cocokkan_wajah(frame, deteksi)
            if hasil is None:
                for wajah in deteksi:
                    gambar_wajah(kanvas, wajah, f"{wajah.skor:.2f}", warna=(170, 170, 170))
                if len(deteksi) > 1:
                    self._gambar_panel(kanvas, ["Lebih dari satu wajah: hasil tidak dipakai."])
                elif len(deteksi) == 0:
                    self._gambar_panel(kanvas, ["Tidak ada wajah."])
            else:
                label, warna = label_hasil(hasil)
                for wajah in deteksi:
                    gambar_wajah(kanvas, wajah, label, warna=warna)
                self._gambar_panel(kanvas, self._baris_hasil(hasil))
        else:
            for wajah in deteksi:
                gambar_wajah(kanvas, wajah, f"wajah {wajah.skor:.2f}")
            if self.mode == "enroll" and self.enrollment is not None:
                self._gambar_panel(
                    kanvas,
                    [
                        f"Label: {self.enrollment.label}",
                        f"Sampel: {self.enrollment.terkumpul}/{self.enrollment.target}",
                        "Tekan S untuk mengambil sampel.",
                        self.enrollment.pesan_terakhir,
                    ],
                    judul="ENROLLMENT (RAM saja)",
                )
            elif len(deteksi) == 0:
                self._gambar_panel(kanvas, ["Tidak ada wajah."])

        self._gambar_panel_bawah(kanvas, len(deteksi))
        return kanvas

    # -------------------------------------------------------------- gambar

    def _baris_hasil(self, hasil: HasilCocok) -> list[str]:
        baris = [
            f"Status: {hasil.status.value}",
            f"Ambang uji: {hasil.ambang:.3f}  margin: {hasil.margin:.3f}",
        ]
        for kandidat in hasil.kandidat[:3]:
            baris.append(f"  {kandidat.label}: {kandidat.skor:.3f}")
        if hasil.alasan:
            baris.append(hasil.alasan)
        return baris

    def _gambar_panel(self, kanvas: np.ndarray, baris: list[str], judul: str = "") -> None:
        warna = (245, 245, 245)
        if judul.startswith("ENROLLMENT"):
            warna = (60, 200, 240)
        panel_status(kanvas, baris, warna_judul=warna, judul=judul)

    def _gambar_panel_bawah(self, kanvas: np.ndarray, jumlah_wajah: int) -> None:
        tinggi = kanvas.shape[0]
        cv2.putText(
            kanvas,
            f"mode: {self.mode}  wajah: {jumlah_wajah}  galeri: {len(self.galeri)}  "
            f"[D] deteksi  [E] enrollment  [R] recognize  [C] hapus  [Q] keluar",
            (12, tinggi - 14),
            cv2.FONT_HERSHEY_SIMPLEX,
            0.45,
            (200, 200, 200),
            1,
            cv2.LINE_AA,
        )

    @staticmethod
    def _cetak_bantuan() -> None:
        print(
            "\nTombol:\n"
            "  D  mode deteksi (kotak + landmark)\n"
            "  E  mulai enrollment identitas baru (label anonim PERSON-00N)\n"
            "  S  ambil satu sampel saat mode enrollment\n"
            "  R  mode recognition (bandingkan dengan galeri di RAM)\n"
            "  C  hapus seluruh galeri dari memori\n"
            "  Q  keluar (data enrollment hilang)\n"
        )

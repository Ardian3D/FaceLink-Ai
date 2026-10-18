"""Penilaian kualitas frame untuk enrollment dan recognition.

Aturannya sengaja konservatif dan dapat dijelaskan: menolak sampel buruk lebih
baik daripada memasukkan sampel buruk ke galeri, karena sampel buruk merusak
semua perbandingan berikutnya. Setiap penolakan membawa kode yang sama dengan
`docs/API_CONTRACT.md` §5.3 (`NO_FACE`, `MULTIPLE_FACES`, `LOW_QUALITY`,
`FACE_TOO_SMALL`) supaya istilahnya tidak bercabang.
"""

from __future__ import annotations

from dataclasses import dataclass, field

import cv2
import numpy as np

from .config import (
    MAX_MEAN_BRIGHTNESS,
    MIN_FACE_SIZE_PX,
    MIN_MEAN_BRIGHTNESS,
    MIN_SHARPNESS,
)
from .detector import Deteksi

#: Kode alasan penolakan, sama dengan kontrak API.
NO_FACE = "NO_FACE"
MULTIPLE_FACES = "MULTIPLE_FACES"
LOW_QUALITY = "LOW_QUALITY"
FACE_TOO_SMALL = "FACE_TOO_SMALL"


@dataclass
class Penilaian:
    """Hasil penilaian satu frame."""

    diterima: bool
    kode: str | None = None
    pesan: str = ""
    rincian: dict[str, float] = field(default_factory=dict)


def ketajaman(frame: np.ndarray, deteksi: Deteksi) -> float:
    """Varians Laplacian pada potongan wajah.

    Nilai tinggi berarti tepi tajam. Angka mutlaknya bergantung pada ukuran dan
    konten, jadi ambang di sini hanya penyaring kasar terhadap wajah buram —
    bukan ukuran kualitas yang dapat dibandingkan antar kamera.
    """
    x, y, w, h = deteksi.kotak
    potongan = frame[max(0, y) : y + h, max(0, x) : x + w]
    if potongan.size == 0:
        return 0.0
    abu = cv2.cvtColor(potongan, cv2.COLOR_BGR2GRAY)
    return float(cv2.Laplacian(abu, cv2.CV_64F).var())


def kecerahan(frame: np.ndarray, deteksi: Deteksi) -> float:
    """Rata-rata kecerahan potongan wajah (0–255)."""
    x, y, w, h = deteksi.kotak
    potongan = frame[max(0, y) : y + h, max(0, x) : x + w]
    if potongan.size == 0:
        return 0.0
    abu = cv2.cvtColor(potongan, cv2.COLOR_BGR2GRAY)
    return float(abu.mean())


def nilai_frame(frame: np.ndarray, deteksi: list[Deteksi]) -> Penilaian:
    """Menilai satu frame untuk keperluan enrollment.

    Urutan pemeriksaan penting: jumlah wajah lebih dahulu, lalu ukuran, lalu
    ketajaman dan pencahayaan. Bila dua wajah terdeteksi, ukuran wajah mana pun
    tidak relevan — pengguna harus memperbaiki komposisi frame lebih dahulu.
    """
    if len(deteksi) == 0:
        return Penilaian(False, NO_FACE, "Tidak ada wajah pada frame.")
    if len(deteksi) > 1:
        return Penilaian(
            False,
            MULTIPLE_FACES,
            f"Terdeteksi {len(deteksi)} wajah; enrollment menuntut tepat satu.",
        )

    wajah = deteksi[0]
    if wajah.sisi_terpendek < MIN_FACE_SIZE_PX:
        return Penilaian(
            False,
            FACE_TOO_SMALL,
            f"Wajah terlalu kecil ({wajah.sisi_terpendek} px, minimal {MIN_FACE_SIZE_PX} px).",
            {"sisi_px": float(wajah.sisi_terpendek)},
        )

    nilai_tajam = ketajaman(frame, wajah)
    nilai_terang = kecerahan(frame, wajah)
    rincian = {"ketajaman": nilai_tajam, "kecerahan": nilai_terang, "skor_deteksi": wajah.skor}

    if nilai_tajam < MIN_SHARPNESS:
        return Penilaian(False, LOW_QUALITY, "Wajah buram; tahan diam dan dekatkan kamera.", rincian)
    if not (MIN_MEAN_BRIGHTNESS <= nilai_terang <= MAX_MEAN_BRIGHTNESS):
        arah = "terlalu gelap" if nilai_terang < MIN_MEAN_BRIGHTNESS else "terlalu terang"
        return Penilaian(False, LOW_QUALITY, f"Pencahayaan {arah}; perbaiki sumber cahaya.", rincian)

    return Penilaian(True, None, "Sampel diterima.", rincian)

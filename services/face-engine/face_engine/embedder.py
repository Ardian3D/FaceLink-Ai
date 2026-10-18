"""Ekstraksi embedding dengan SFace.

SFace menerima wajah yang **sudah disejajarkan** memakai lima landmark YuNet,
bukan potongan kotak mentah. Alignment dilakukan `cv2.FaceRecognizerSF.alignCrop`,
yang memetakan landmark ke posisi kanonik model. Melewati langkah ini membuat
skor jatuh pada wajah yang sedikit miring, dan itu bukan kesalahan model.
"""

from __future__ import annotations

from pathlib import Path

import cv2
import numpy as np

from .detector import Deteksi
from .errors import GalatModel

#: Panjang vektor embedding SFace. Dipakai untuk menolak keluaran berbentuk lain
#: sebelum vektor itu dibandingkan — membandingkan vektor dengan panjang berbeda
#: akan menghasilkan galat NumPy yang membingungkan, bukan penolakan yang jelas.
EMBEDDING_DIM = 128


class Embedder:
    """Pembungkus `cv2.FaceRecognizerSF`."""

    def __init__(self, model_path: Path) -> None:
        if not model_path.is_file():
            raise GalatModel(
                f"Berkas model SFace tidak ditemukan: {model_path}. "
                "Jalankan `python scripts/download_models.py` lebih dahulu."
            )
        try:
            self._pengenal = cv2.FaceRecognizerSF.create(str(model_path), "")
        except cv2.error as galat:  # pragma: no cover - bergantung berkas rusak
            raise GalatModel(f"SFace gagal dimuat dari {model_path}: {galat}") from galat

    def embedding(self, frame: np.ndarray, deteksi: Deteksi) -> np.ndarray:
        """Menghasilkan vektor 128 dimensi yang sudah dinormalisasi L2.

        `alignCrop` menuntut baris deteksi **lengkap** (15 nilai), bukan hanya
        landmark. Memakai landmark saja tidak melempar galat, tetapi menghasilkan
        potongan wajah yang salah dan embedding yang mirip untuk semua orang —
        kegagalan senyap yang hanya terlihat saat skor antarwajah dibandingkan.
        """
        if deteksi.baris is None:
            raise GalatModel(
                "Deteksi tidak membawa baris YuNet lengkap; "
                "jalankan ulang deteksi memakai `Detektor.deteksi()`."
            )

        wajah = self._pengenal.alignCrop(frame, deteksi.baris.astype(np.float32))
        vektor = np.asarray(self._pengenal.feature(wajah), dtype=np.float32).reshape(-1)

        if vektor.size != EMBEDDING_DIM:
            raise GalatModel(
                f"SFace mengembalikan {vektor.size} nilai, diharapkan {EMBEDDING_DIM}."
            )

        norma = float(np.linalg.norm(vektor))
        if norma == 0.0:
            raise GalatModel("SFace mengembalikan vektor nol; wajah tidak dapat dipakai.")
        return vektor / norma

    @staticmethod
    def kemiripan(a: np.ndarray, b: np.ndarray) -> float:
        """Cosine similarity dua vektor yang sudah dinormalisasi."""
        return float(np.dot(a, b))

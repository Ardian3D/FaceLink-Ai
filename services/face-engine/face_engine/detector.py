"""Deteksi wajah dan landmark memakai YuNet.

YuNet mengembalikan, untuk setiap wajah, kotak `x, y, w, h`, skor, dan lima
landmark (mata kiri, mata kanan, hidung, sudut mulut kiri, sudut mulut kanan).
Lima titik itulah yang dibutuhkan SFace untuk alignment; tanpa landmark,
embedding akan bias terhadap posisi kepala.
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

import cv2
import numpy as np

from .config import (
    DETECTOR_INPUT,
    DETECTOR_MAX_SIDE,
    DETECTOR_NMS_THRESHOLD,
    DETECTOR_SCORE_THRESHOLD,
    DETECTOR_TOP_K,
)
from .errors import GalatModel

#: Tata letak satu baris keluaran YuNet (15 nilai):
#:
#:     [0..3]  x, y, w, h
#:     [4..13] lima pasang landmark (mata kiri, mata kanan, hidung, mulut kiri,
#:             mulut kanan) — **bukan** skor di indeks 4
#:     [14]    skor deteksi
#:
#: Urutan ini mudah salah dibaca: banyak contoh menulis "kotak, skor, landmark",
#: padahal skor ada di **akhir**. Menaruh skor di indeks 4 membuat `alignCrop`
#: menerima "landmark" berisi nilai piksel, sehingga potongan wajah bergeser dan
#: semua embedding menjadi mirip satu sama lain — kegagalan senyap yang hanya
#: terlihat saat skor antarwajah dibandingkan.
_KOLOM_LANDMARK = slice(4, 14)
_INDEKS_SKOR = 14


@dataclass(frozen=True)
class Deteksi:
    """Satu wajah pada satu frame."""

    x: int
    y: int
    lebar: int
    tinggi: int
    skor: float
    landmarks: np.ndarray  # bentuk (5, 2), koordinat piksel frame asli
    #: Baris mentah YuNet untuk frame asli: `x, y, w, h, skor, 10 nilai landmark`.
    #: `cv2.FaceRecognizerSF.alignCrop` menuntut **seluruh** baris ini, bukan
    #: hanya landmark. Memberinya landmark saja menghasilkan alignment yang
    #: salah tanpa galat — dan embedding yang tampak mirip untuk semua wajah.
    baris: np.ndarray | None = None

    @property
    def kotak(self) -> tuple[int, int, int, int]:
        return (self.x, self.y, self.lebar, self.tinggi)

    @property
    def sisi_terpendek(self) -> int:
        return min(self.lebar, self.tinggi)


def ukuran_masukan(lebar: int, tinggi: int, sisi_maks: int = DETECTOR_MAX_SIDE) -> tuple[int, int]:
    """Ukuran masukan detektor yang mempertahankan rasio aspek frame.

    Frame di bawah batas dipertahankan pada resolusi native: pembesaran yang
    tidak diperlukan mengubah skala wajah yang dilihat model. Hanya frame besar
    yang diperkecil, dengan sisi pendek mengikuti rasio dan kelipatan 32.
    """
    if lebar <= 0 or tinggi <= 0:
        return (sisi_maks, sisi_maks)
    if max(lebar, tinggi) <= sisi_maks:
        return (lebar, tinggi)
    skala = sisi_maks / max(lebar, tinggi)
    lebar_m = max(32, int(round(lebar * skala / 32)) * 32)
    tinggi_m = max(32, int(round(tinggi * skala / 32)) * 32)
    return (lebar_m, tinggi_m)


class Detektor:
    """Pembungkus `cv2.FaceDetectorYN`."""

    def __init__(
        self,
        model_path: Path,
        input_size: tuple[int, int] = DETECTOR_INPUT,
        score_threshold: float = DETECTOR_SCORE_THRESHOLD,
    ) -> None:
        if not model_path.is_file():
            raise GalatModel(
                f"Berkas model YuNet tidak ditemukan: {model_path}. "
                "Jalankan `python scripts/download_models.py` lebih dahulu."
            )
        try:
            self._detektor = cv2.FaceDetectorYN.create(
                str(model_path),
                "",
                input_size,
                score_threshold,
                DETECTOR_NMS_THRESHOLD,
                DETECTOR_TOP_K,
            )
        except cv2.error as galat:  # pragma: no cover - bergantung berkas rusak
            raise GalatModel(f"YuNet gagal dimuat dari {model_path}: {galat}") from galat

    def deteksi(self, frame: np.ndarray) -> list[Deteksi]:
        """Mendeteksi wajah pada frame BGR dan mengembalikan koordinat aslinya.

        Ukuran masukan mengikuti **rasio frame**, bukan kotak tetap 320x320.
        Memaksa frame 16:9 menjadi kotak merusak geometri wajah: landmark hasil
        deteksi menjadi tidak sejajar dengan `alignCrop` SFace, dan pada
        pengukuran 19 Oktober 2026 deteksi webcam 1280x720 turun dari 40/40
        frame menjadi 0/20. Batas sisi panjang ditahan agar biaya inferensi
        tetap terkendali pada kamera resolusi tinggi.
        """
        tinggi_asli, lebar_asli = frame.shape[:2]
        lebar_model, tinggi_model = ukuran_masukan(lebar_asli, tinggi_asli)

        if (lebar_asli, tinggi_asli) != (lebar_model, tinggi_model):
            kecil = cv2.resize(frame, (lebar_model, tinggi_model), interpolation=cv2.INTER_AREA)
        else:
            kecil = frame

        self._detektor.setInputSize((lebar_model, tinggi_model))
        _, hasil = self._detektor.detect(kecil)
        if hasil is None or len(hasil) == 0:
            return []
        if (not isinstance(hasil, np.ndarray) or hasil.ndim != 2 or hasil.shape[1] != 15
                or len(hasil) > DETECTOR_TOP_K or not np.isfinite(hasil).all()):
            raise GalatModel("YuNet mengembalikan deteksi tidak sah.")

        skala_x = lebar_asli / lebar_model
        skala_y = tinggi_asli / tinggi_model

        deteksi: list[Deteksi] = []
        for baris in hasil:
            x, y, w, h = (float(nilai) for nilai in baris[:4])
            skor = float(baris[_INDEKS_SKOR])
            landmark = np.asarray(baris[_KOLOM_LANDMARK], dtype=np.float32).reshape(5, 2)
            landmark[:, 0] *= skala_x
            landmark[:, 1] *= skala_y

            # Baris untuk `alignCrop` disusun pada koordinat frame **asli**,
            # karena frame yang diberikan ke SFace juga frame asli. Urutannya
            # wajib persis seperti keluaran YuNet: x, y, w, h, skor, lalu
            # 5 pasang titik.
            baris_asli = np.empty(15, dtype=np.float32)
            baris_asli[0] = x * skala_x
            baris_asli[1] = y * skala_y
            baris_asli[2] = w * skala_x
            baris_asli[3] = h * skala_y
            baris_asli[_KOLOM_LANDMARK] = landmark.reshape(-1)
            baris_asli[_INDEKS_SKOR] = skor

            deteksi.append(
                Deteksi(
                    x=max(0, int(round(x * skala_x))),
                    y=max(0, int(round(y * skala_y))),
                    lebar=max(0, int(round(w * skala_x))),
                    tinggi=max(0, int(round(h * skala_y))),
                    skor=skor,
                    landmarks=landmark,
                    baris=baris_asli,
                )
            )
        return deteksi

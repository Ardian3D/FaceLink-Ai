"""Pembungkus kamera webcam.

Lingkupnya kecil dan sengaja bodoh: buka perangkat, baca frame, tutup. Semua
keputusan kualitas dan pengenalan ada di modul lain. Yang dikerjakan di sini
hanya menerjemahkan kegagalan OpenCV menjadi galat yang dapat dibedakan
pemanggil, karena pesan `cv2` mentah ("can't grab frame") tidak memberi tahu
pengguna apa yang harus dilakukan.
"""

from __future__ import annotations

from types import TracebackType

import cv2
import numpy as np

from .errors import GalatKamera

#: Pesan yang menyebutkan tindakan, bukan hanya keadaan.
PETUNJUK_KAMERA = (
    "Periksa: kamera tidak dipakai aplikasi lain, izin kamera Windows aktif "
    "(Settings → Privacy → Camera), dan indeks kamera benar (coba --camera 1)."
)


class Kamera:
    """Kamera webcam yang harus dipakai sebagai context manager.

    Contoh:
        with Kamera(0, 1280, 720) as cam:
            frame = cam.baca()
    """

    def __init__(self, indeks: int = 0, lebar: int = 1280, tinggi: int = 720) -> None:
        self.indeks = indeks
        self.lebar = lebar
        self.tinggi = tinggi
        self._tangkapan: cv2.VideoCapture | None = None

    def __enter__(self) -> "Kamera":
        self.buka()
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        self.tutup()

    def buka(self) -> None:
        # CAP_DSHOW dipakai di Windows: backend default MSMF kadang butuh
        # beberapa detik dan gagal pada kamera terintegrasi tertentu.
        tangkapan = cv2.VideoCapture(self.indeks, cv2.CAP_DSHOW)
        if not tangkapan.isOpened():
            tangkapan.release()
            # Percobaan kedua tanpa backend khusus, untuk mesin non-Windows
            # atau driver yang tidak mendukung DirectShow.
            tangkapan = cv2.VideoCapture(self.indeks)
        if not tangkapan.isOpened():
            tangkapan.release()
            raise GalatKamera(
                f"Kamera indeks {self.indeks} tidak dapat dibuka. {PETUNJUK_KAMERA}"
            )

        tangkapan.set(cv2.CAP_PROP_FRAME_WIDTH, self.lebar)
        tangkapan.set(cv2.CAP_PROP_FRAME_HEIGHT, self.tinggi)
        self._tangkapan = tangkapan

    def baca(self) -> np.ndarray:
        """Membaca satu frame BGR. Melempar `GalatKamera` bila frame kosong."""
        if self._tangkapan is None:
            raise GalatKamera("Kamera belum dibuka; panggil buka() lebih dahulu.")

        ok, frame = self._tangkapan.read()
        if not ok or frame is None or frame.size == 0:
            # Satu frame gagal bukan alasan menyerah: kamera USB kadang
            # melewatkan frame pertama setelah dibuka. Coba sekali lagi.
            ok, frame = self._tangkapan.read()
        if not ok or frame is None or frame.size == 0:
            raise GalatKamera(f"Frame kosong dari kamera indeks {self.indeks}. {PETUNJUK_KAMERA}")
        return frame

    def tutup(self) -> None:
        if self._tangkapan is not None:
            self._tangkapan.release()
            self._tangkapan = None

    @property
    def resolusi(self) -> tuple[int, int]:
        """Resolusi efektif yang benar-benar diberikan driver, bukan yang diminta."""
        if self._tangkapan is None:
            return (0, 0)
        lebar = int(self._tangkapan.get(cv2.CAP_PROP_FRAME_WIDTH))
        tinggi = int(self._tangkapan.get(cv2.CAP_PROP_FRAME_HEIGHT))
        return (lebar, tinggi)

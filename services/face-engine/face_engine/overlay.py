"""Penggambaran kotak, landmark, dan panel status pada frame.

Seluruh gambar di sini hanya hidup di memori dan ditampilkan lewat jendela
OpenCV. Tidak ada fungsi yang menulis berkas, dan itu disengaja: prototype ini
tidak boleh meninggalkan rekaman wajah di disk.
"""

from __future__ import annotations

import cv2
import numpy as np

from .detector import Deteksi
from .matcher import HasilCocok, Status

#: Warna BGR. Hijau untuk cocok, kuning untuk perlu verifikasi, merah untuk
#: tidak dikenal — dipilih agar status juga terbaca tanpa melihat teks.
HIJAU = (80, 200, 80)
KUNING = (60, 200, 240)
MERAH = (70, 70, 230)
PUTIH = (245, 245, 245)
ABU = (170, 170, 170)

_WARNA_STATUS = {
    Status.COCOK: HIJAU,
    Status.PERLU_VERIFIKASI: KUNING,
    Status.TIDAK_DIKENAL: MERAH,
}


def gambar_wajah(
    frame: np.ndarray,
    deteksi: Deteksi,
    label: str = "",
    warna: tuple[int, int, int] = HIJAU,
) -> None:
    """Menggambar kotak dan lima landmark pada frame (diubah di tempat)."""
    x, y, w, h = deteksi.kotak
    cv2.rectangle(frame, (x, y), (x + w, y + h), warna, 2)

    for titik in deteksi.landmarks:
        cv2.circle(frame, (int(titik[0]), int(titik[1])), 2, PUTIH, -1)

    if label:
        # Latar di belakang teks supaya terbaca di atas wajah terang.
        (lebar_teks, tinggi_teks), _ = cv2.getTextSize(label, cv2.FONT_HERSHEY_SIMPLEX, 0.55, 1)
        cv2.rectangle(
            frame,
            (x, max(0, y - tinggi_teks - 8)),
            (x + lebar_teks + 8, y),
            warna,
            -1,
        )
        cv2.putText(
            frame,
            label,
            (x + 4, max(12, y - 6)),
            cv2.FONT_HERSHEY_SIMPLEX,
            0.55,
            (20, 20, 20),
            1,
            cv2.LINE_AA,
        )


def panel_status(
    frame: np.ndarray,
    baris: list[str],
    warna_judul: tuple[int, int, int] = PUTIH,
    judul: str = "",
) -> None:
    """Menulis panel semi-transparan di kiri atas."""
    tinggi_baris = 24
    tinggi = 16 + tinggi_baris * (len(baris) + (1 if judul else 0))
    lapisan = frame.copy()
    cv2.rectangle(lapisan, (8, 8), (520, 8 + tinggi), (25, 25, 25), -1)
    cv2.addWeighted(lapisan, 0.65, frame, 0.35, 0, frame)

    y = 30
    if judul:
        cv2.putText(frame, judul, (18, y), cv2.FONT_HERSHEY_SIMPLEX, 0.6, warna_judul, 2, cv2.LINE_AA)
        y += tinggi_baris

    for teks in baris:
        cv2.putText(frame, teks, (18, y), cv2.FONT_HERSHEY_SIMPLEX, 0.5, ABU, 1, cv2.LINE_AA)
        y += tinggi_baris


def label_hasil(hasil: HasilCocok) -> tuple[str, tuple[int, int, int]]:
    """Teks dan warna untuk satu hasil pencocokan."""
    warna = _WARNA_STATUS.get(hasil.status, MERAH)
    teratas = hasil.teratas
    if teratas is None:
        return ("tidak dikenal", warna)
    if hasil.status is Status.COCOK:
        return (f"{teratas.label}  {teratas.skor:.3f}", warna)
    if hasil.status is Status.PERLU_VERIFIKASI:
        return (f"perlu verifikasi  {teratas.label} {teratas.skor:.3f}", warna)
    return (f"tidak dikenal  {teratas.skor:.3f}", warna)

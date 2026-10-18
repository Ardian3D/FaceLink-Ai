"""Pengujian penilaian kualitas frame tanpa kamera.

Frame dibuat sintetis: kotak abu-abu polos, gradien, dan pola kotak-kotak.
Tujuannya memastikan urutan pemeriksaan dan kode penolakan benar, bukan menilai
akurasi detektor.
"""

from __future__ import annotations

import cv2
import numpy as np

from face_engine.detector import Deteksi
from face_engine.quality import (
    FACE_TOO_SMALL,
    LOW_QUALITY,
    MULTIPLE_FACES,
    NO_FACE,
    nilai_frame,
)


def deteksi_palsu(x: int = 100, y: int = 100, sisi: int = 200) -> Deteksi:
    landmark = np.zeros((5, 2), dtype=np.float32)
    return Deteksi(x=x, y=y, lebar=sisi, tinggi=sisi, skor=0.9, landmarks=landmark)


def frame_polos(lebar: int = 640, tinggi: int = 480, nilai: int = 128) -> np.ndarray:
    return np.full((tinggi, lebar, 3), nilai, dtype=np.uint8)


def frame_berpola(lebar: int = 640, tinggi: int = 480) -> np.ndarray:
    """Kotak-kotak hitam-putih: tepi tajam, jadi Laplacian tinggi."""
    kotak = np.indices((tinggi, lebar)).sum(axis=0) // 8 % 2
    return (kotak * 255).astype(np.uint8)[:, :, None].repeat(3, axis=2)


def test_frame_tanpa_wajah_ditolak() -> None:
    hasil = nilai_frame(frame_polos(), [])
    assert not hasil.diterima
    assert hasil.kode == NO_FACE


def test_dua_wajah_ditolak_walau_keduanya_besar() -> None:
    hasil = nilai_frame(frame_polos(), [deteksi_palsu(sisi=200), deteksi_palsu(x=350, sisi=200)])
    assert not hasil.diterima
    assert hasil.kode == MULTIPLE_FACES
    assert "2 wajah" in hasil.pesan


def test_wajah_kecil_ditolak() -> None:
    hasil = nilai_frame(frame_berpola(), [deteksi_palsu(sisi=40)])
    assert not hasil.diterima
    assert hasil.kode == FACE_TOO_SMALL
    assert hasil.rincian["sisi_px"] == 40.0


def test_wajah_buram_ditolak() -> None:
    hasil = nilai_frame(frame_polos(), [deteksi_palsu(sisi=200)])
    assert not hasil.diterima
    assert hasil.kode == LOW_QUALITY
    assert "buram" in hasil.pesan


def test_wajah_terlalu_gelap_ditolak() -> None:
    gelap = frame_berpola()
    gelap[:] = (gelap * 0.05).astype(np.uint8)
    hasil = nilai_frame(gelap, [deteksi_palsu(sisi=200)])
    assert not hasil.diterima
    assert hasil.kode == LOW_QUALITY
    assert "gelap" in hasil.pesan


def test_wajah_terlalu_terang_ditolak() -> None:
    terang = frame_berpola()
    terang[:] = np.clip(terang.astype(np.int16) + 200, 0, 255).astype(np.uint8)
    hasil = nilai_frame(terang, [deteksi_palsu(sisi=200)])
    assert not hasil.diterima
    assert hasil.kode == LOW_QUALITY
    assert "terang" in hasil.pesan


def test_sampel_tajam_dan_terang_diterima() -> None:
    hasil = nilai_frame(frame_berpola(), [deteksi_palsu(sisi=200)])
    assert hasil.diterima
    assert hasil.kode is None
    assert hasil.rincian["ketajaman"] > 40.0


def test_kotak_wajah_di_luar_frame_tidak_melempar_galat() -> None:
    """Deteksi yang meleset keluar bingkai harus dinilai, bukan crash."""
    hasil = nilai_frame(frame_polos(), [deteksi_palsu(x=-50, y=-50, sisi=200)])
    assert isinstance(hasil.diterima, bool)

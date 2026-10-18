"""Pengujian alur enrollment di memori tanpa kamera.

Aplikasi diuji dengan detektor dan embedder tiruan: yang diperiksa adalah aturan
alur (label anonim, penolakan sampel buruk, perpindahan mode), bukan model.
"""

from __future__ import annotations

from dataclasses import dataclass, field

import numpy as np
import pytest

from face_engine.app import Aplikasi
from face_engine.config import Konfigurasi
from face_engine.detector import Deteksi
from face_engine.gallery import Galeri


@dataclass
class DetektorTiruan:
    hasil: list[Deteksi] = field(default_factory=list)

    def deteksi(self, frame: np.ndarray) -> list[Deteksi]:  # noqa: ARG002
        return self.hasil


@dataclass
class EmbedderTiruan:
    nilai: float = 1.0

    def embedding(self, frame: np.ndarray, deteksi: Deteksi) -> np.ndarray:  # noqa: ARG002
        vektor = np.array([self.nilai, 0.0], dtype=np.float32)
        return vektor / np.linalg.norm(vektor)

    @staticmethod
    def kemiripan(a: np.ndarray, b: np.ndarray) -> float:
        return float(np.dot(a, b))


def buat_aplikasi(jumlah_sampel: int = 3) -> Aplikasi:
    """Aplikasi dengan model tiruan; tidak menyentuh berkas atau kamera."""
    aplikasi = Aplikasi.__new__(Aplikasi)
    aplikasi.konfigurasi = Konfigurasi(required_samples=jumlah_sampel)
    aplikasi.mode = "detect"
    aplikasi.galeri = Galeri()
    aplikasi.enrollment = None
    aplikasi.detektor = DetektorTiruan()
    aplikasi.embedder = EmbedderTiruan()
    aplikasi._fps_terakhir = 0.0
    aplikasi._waktu_lalu = 0.0
    return aplikasi


def wajah_besar() -> Deteksi:
    return Deteksi(
        x=100,
        y=100,
        lebar=200,
        tinggi=200,
        skor=0.99,
        landmarks=np.zeros((5, 2), dtype=np.float32),
    )


def frame_berpola() -> np.ndarray:
    kotak = np.indices((480, 640)).sum(axis=0) // 8 % 2
    return (kotak * 255).astype(np.uint8)[:, :, None].repeat(3, axis=2)


def test_enrollment_memakai_label_anonim_dan_mode_enroll() -> None:
    aplikasi = buat_aplikasi()
    status = aplikasi.mulai_enrollment()

    assert status.label == "PERSON-001"
    assert aplikasi.mode == "enroll"
    assert status.target == 3


def test_sampel_buruk_ditolak_dan_tidak_menambah_galeri() -> None:
    aplikasi = buat_aplikasi()
    aplikasi.mulai_enrollment()
    aplikasi.detektor.hasil = []  # tidak ada wajah

    diterima, pesan = aplikasi.ambil_sampel(frame_berpola(), [])

    assert not diterima
    assert "Tidak ada wajah" in pesan
    # Identitas baru dibuat di galeri hanya setelah sampel pertama diterima,
    # jadi penolakan tidak boleh menyisakan entri kosong.
    assert len(aplikasi.galeri) == 0
    assert aplikasi.galeri.semua() == []


def test_dua_wajah_menolak_sampel() -> None:
    aplikasi = buat_aplikasi()
    aplikasi.mulai_enrollment()

    diterima, pesan = aplikasi.ambil_sampel(frame_berpola(), [wajah_besar(), wajah_besar()])

    assert not diterima
    assert "2 wajah" in pesan


def test_sampel_baik_mengisi_galeri_sampai_target_dan_berpindah_ke_recognize() -> None:
    aplikasi = buat_aplikasi(jumlah_sampel=2)
    aplikasi.mulai_enrollment()
    frame = frame_berpola()

    pertama, _ = aplikasi.ambil_sampel(frame, [wajah_besar()])
    assert pertama
    assert aplikasi.mode == "enroll"

    kedua, pesan = aplikasi.ambil_sampel(frame, [wajah_besar()])
    assert kedua
    assert "terdaftar dengan 2 sampel" in pesan
    assert aplikasi.mode == "recognize"

    identitas = aplikasi.galeri.semua()[0]
    assert len(identitas.embeddings) == 2
    assert all(v.shape == (2,) for v in identitas.embeddings)


def test_enrollment_tidak_pernah_berjalan_tanpa_permintaan() -> None:
    """Membuka aplikasi tidak boleh mendaftarkan siapa pun."""
    aplikasi = buat_aplikasi()
    assert aplikasi.enrollment is None
    assert len(aplikasi.galeri) == 0
    assert aplikasi.mode == "detect"


def test_ambil_sampel_tanpa_sesi_enrollment_ditolak() -> None:
    aplikasi = buat_aplikasi()
    diterima, pesan = aplikasi.ambil_sampel(frame_berpola(), [wajah_besar()])
    assert not diterima
    assert "belum dimulai" in pesan


def test_recognition_dengan_dua_wajah_tidak_menghasilkan_kandidat() -> None:
    aplikasi = buat_aplikasi()
    aplikasi.mulai_enrollment()
    aplikasi.ambil_sampel(frame_berpola(), [wajah_besar()])

    assert aplikasi.cocokkan_wajah(frame_berpola(), [wajah_besar(), wajah_besar()]) is None


def test_recognition_wajah_belum_terdaftar_tidak_menghasilkan_cocok() -> None:
    aplikasi = buat_aplikasi()
    aplikasi.mulai_enrollment()
    aplikasi.ambil_sampel(frame_berpola(), [wajah_besar()])
    aplikasi.embedder = EmbedderTiruan(nilai=-1.0)  # arah berlawanan

    hasil = aplikasi.cocokkan_wajah(frame_berpola(), [wajah_besar()])

    assert hasil is not None
    assert hasil.status.value == "tidak_dikenal"

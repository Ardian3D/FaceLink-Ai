"""Pengujian penanganan kamera dan model yang tidak tersedia.

Kamera nyata **tidak** dibuka di sini. Yang diuji adalah perilaku saat perangkat
tidak ada, karena itulah jalur yang paling sering ditemui pengguna dan paling
mudah salah ditangani (traceback OpenCV, bukan pesan tindakan).
"""

from __future__ import annotations

import pytest

from face_engine.camera import Kamera
from face_engine.detector import Detektor
from face_engine.embedder import Embedder
from face_engine.errors import GalatKamera, GalatModel


def test_kamera_dengan_indeks_tidak_ada_melempar_galat_jelas() -> None:
    """Indeks 9999 praktis tidak pernah ada; pesan harus menyebut tindakan."""
    with pytest.raises(GalatKamera) as info:
        Kamera(indeks=9999).buka()

    pesan = str(info.value)
    assert "tidak dapat dibuka" in pesan
    assert "izin kamera" in pesan
    assert info.value.kode == "KAMERA_GALAT"


def test_kamera_yang_belum_dibuka_menolak_membaca() -> None:
    with pytest.raises(GalatKamera) as info:
        Kamera().baca()
    assert "belum dibuka" in str(info.value)


def test_detektor_menolak_berkas_model_hilang(tmp_path) -> None:
    with pytest.raises(GalatModel) as info:
        Detektor(tmp_path / "tidak-ada.onnx")

    pesan = str(info.value)
    assert "tidak ditemukan" in pesan
    assert "download_models" in pesan
    assert info.value.kode == "MODEL_GALAT"


def test_embedder_menolak_berkas_model_hilang(tmp_path) -> None:
    with pytest.raises(GalatModel) as info:
        Embedder(tmp_path / "tidak-ada.onnx")
    assert "tidak ditemukan" in str(info.value)


def test_konfigurasi_menunjuk_berkas_di_direktori_model(tmp_path) -> None:
    from face_engine.config import Konfigurasi

    konfigurasi = Konfigurasi(model_dir=tmp_path)
    assert konfigurasi.yunet_path.parent == tmp_path
    assert konfigurasi.sface_path.parent == tmp_path
    assert konfigurasi.yunet_path.name.endswith(".onnx")

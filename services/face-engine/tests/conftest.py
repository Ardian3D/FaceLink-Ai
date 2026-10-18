"""Konfigurasi pytest bersama.

Tidak ada fixture yang membuka kamera atau memuat model besar. Pengujian yang
membutuhkan model ditandai `model` dan dilewati bila berkas bobot tidak ada,
supaya `pytest` tetap berguna pada checkout bersih tanpa unduhan.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parent.parent
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from face_engine.config import Konfigurasi  # noqa: E402


def pytest_configure(config: pytest.Config) -> None:
    config.addinivalue_line("markers", "model: memerlukan berkas bobot ONNX di models/")


@pytest.fixture(scope="session")
def konfigurasi() -> Konfigurasi:
    return Konfigurasi()


@pytest.fixture(scope="session")
def model_tersedia(konfigurasi: Konfigurasi) -> bool:
    return konfigurasi.yunet_path.is_file() and konfigurasi.sface_path.is_file()


@pytest.fixture(autouse=True)
def lewati_tanpa_model(request: pytest.FixtureRequest, model_tersedia: bool) -> None:
    if request.node.get_closest_marker("model") and not model_tersedia:
        pytest.skip("berkas model ONNX tidak ada; jalankan scripts/download_models.py")

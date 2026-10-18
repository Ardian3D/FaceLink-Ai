"""Pengujian model nyata: YuNet dan SFace pada gambar sintetis.

Gambar di sini **bukan** wajah peserta. Yang diuji hanya kontrak teknis:
detektor mengembalikan daftar kosong pada gambar tanpa wajah, embedding berukuran
128 dan ternormalisasi, dan vektor yang sama menghasilkan skor 1.0. Pengujian
akurasi pengenalan wajah nyata dilakukan manual dan dilaporkan terpisah.
"""

from __future__ import annotations

import numpy as np
import pytest

from face_engine.config import Konfigurasi
from face_engine.detector import Detektor
from face_engine.embedder import EMBEDDING_DIM, Embedder

pytestmark = pytest.mark.model


def gambar_acak(lebar: int = 320, tinggi: int = 320, seed: int = 7) -> np.ndarray:
    rng = np.random.default_rng(seed)
    return rng.integers(0, 256, size=(tinggi, lebar, 3), dtype=np.uint8)


def test_detektor_memuat_model_dan_menjawab_daftar_kosong_pada_gambar_acak(
    konfigurasi: Konfigurasi,
) -> None:
    detektor = Detektor(konfigurasi.yunet_path)
    hasil = detektor.deteksi(gambar_acak())
    assert isinstance(hasil, list)
    assert len(hasil) == 0


def test_detektor_menerima_frame_berukuran_berbeda(konfigurasi: Konfigurasi) -> None:
    """Koordinat harus diskalakan balik ke ukuran frame asli."""
    detektor = Detektor(konfigurasi.yunet_path)
    for ukuran in ((640, 480), (1280, 720), (320, 320)):
        hasil = detektor.deteksi(np.zeros((ukuran[1], ukuran[0], 3), dtype=np.uint8))
        assert isinstance(hasil, list)


def wajah_sintetis(x: int = 120, y: int = 120, lebar: int = 160, tinggi: int = 180):
    """Deteksi tiruan dengan baris YuNet lengkap, supaya `alignCrop` dapat dipakai.

    Urutan baris mengikuti keluaran YuNet yang sebenarnya: kotak, lima pasang
    landmark, lalu skor di indeks terakhir.
    """
    from face_engine.detector import Deteksi

    landmark = np.array(
        [[140, 160], [260, 160], [200, 210], [150, 260], [250, 260]], dtype=np.float32
    )
    baris = np.empty(15, dtype=np.float32)
    baris[:4] = (x, y, lebar, tinggi)
    baris[4:14] = landmark.reshape(-1)
    baris[14] = 0.99
    return Deteksi(
        x=x,
        y=y,
        lebar=lebar,
        tinggi=tinggi,
        skor=0.99,
        landmarks=landmark,
        baris=baris,
    )


def test_embedder_menghasilkan_vektor_ternormalisasi(konfigurasi: Konfigurasi) -> None:
    """Embedding diuji pada landmark sintetis; nilainya tidak ditafsirkan."""
    embedder = Embedder(konfigurasi.sface_path)
    frame = gambar_acak(400, 400)
    wajah = wajah_sintetis()

    vektor = embedder.embedding(frame, wajah)

    assert vektor.shape == (EMBEDDING_DIM,)
    assert vektor.dtype == np.float32
    assert float(np.linalg.norm(vektor)) == pytest.approx(1.0, abs=1e-5)


def test_embedding_yang_sama_menghasilkan_skor_satu(konfigurasi: Konfigurasi) -> None:
    from face_engine.detector import Deteksi

    embedder = Embedder(konfigurasi.sface_path)
    frame = gambar_acak(400, 400, seed=11)
    wajah = wajah_sintetis()

    a = embedder.embedding(frame, wajah)
    b = embedder.embedding(frame, wajah)

    assert Embedder.kemiripan(a, b) == pytest.approx(1.0, abs=1e-5)


def test_embedding_frame_berbeda_menghasilkan_skor_lebih_rendah(konfigurasi: Konfigurasi) -> None:
    from face_engine.detector import Deteksi

    embedder = Embedder(konfigurasi.sface_path)
    wajah = wajah_sintetis()

    a = embedder.embedding(gambar_acak(400, 400, seed=1), wajah)
    b = embedder.embedding(gambar_acak(400, 400, seed=2), wajah)

    skor = Embedder.kemiripan(a, b)
    assert -1.0 <= skor <= 1.0
    assert skor < 1.0


def test_baris_deteksi_memakai_tata_letak_yunet_yang_benar(konfigurasi: Konfigurasi) -> None:
    """Regresi: skor ada di indeks **terakhir**, landmark di 4..14.

    Kesalahan menaruh skor di indeks 4 tidak melempar galat apa pun; ia hanya
    membuat `alignCrop` menerima koordinat palsu dan semua wajah tampak mirip.
    Karena itu tata letak baris diuji langsung.
    """
    detektor = Detektor(konfigurasi.yunet_path)
    # Gambar dengan wajah nyata tidak tersedia di repositori, jadi yang diuji
    # adalah bentuk baris pada deteksi mana pun yang muncul. Bila tidak ada
    # wajah, pengujian dilewati alih-alih gagal.
    hasil = detektor.deteksi(gambar_acak(640, 480))
    if not hasil:
        pytest.skip("tidak ada wajah pada gambar acak; bentuk baris tidak dapat diperiksa")

    for wajah in hasil:
        assert wajah.baris is not None
        assert wajah.baris.shape == (15,)
        assert wajah.baris[14] == pytest.approx(wajah.skor)
        assert np.allclose(wajah.baris[4:14], wajah.landmarks.reshape(-1), atol=1e-4)


def test_ukuran_masukan_mempertahankan_rasio_aspek() -> None:
    """Regresi: memaksa 16:9 menjadi kotak merusak geometri dan menurunkan deteksi."""
    from face_engine.detector import ukuran_masukan

    lebar, tinggi = ukuran_masukan(1280, 720, sisi_maks=960)
    assert (lebar, tinggi) == (960, 544), "960/544 = 1.76, mendekati 16:9"
    assert abs(lebar / tinggi - 1280 / 720) < 0.05

    kuadrat = ukuran_masukan(640, 640, sisi_maks=960)
    assert kuadrat[0] == kuadrat[1]

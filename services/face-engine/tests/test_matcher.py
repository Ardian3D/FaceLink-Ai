"""Pengujian galeri di RAM dan pencocokan embedding.

Tidak ada wajah nyata di sini. Vektor dibuat sintetis, jadi pengujian ini
memeriksa **logika** pemilihan kandidat, bukan kualitas model. Itu memang
tujuannya: model diuji terpisah dengan wajah sungguhan pada pengujian manual.
"""

from __future__ import annotations

import numpy as np
import pytest

from face_engine.gallery import Galeri
from face_engine.matcher import Status, cocokkan


def vektor(*nilai: float) -> np.ndarray:
    """Vektor ternormalisasi L2 dari nilai yang diberikan."""
    a = np.asarray(nilai, dtype=np.float32)
    return a / np.linalg.norm(a)


def test_galeri_memberi_label_anonim_berurutan() -> None:
    galeri = Galeri()
    assert galeri.label_berikutnya() == "PERSON-001"
    galeri.tambah_identitas("PERSON-001")
    assert galeri.label_berikutnya() == "PERSON-002"


def test_galeri_menolak_label_ganda() -> None:
    galeri = Galeri()
    galeri.tambah_identitas("PERSON-001")
    with pytest.raises(Exception) as info:
        galeri.tambah_identitas("PERSON-001")
    assert "sudah terdaftar" in str(info.value)


def test_galeri_menolak_sampel_untuk_label_tak_dikenal() -> None:
    galeri = Galeri()
    with pytest.raises(Exception) as info:
        galeri.tambah_sampel("PERSON-999", vektor(1.0, 0.0))
    assert "belum terdaftar" in str(info.value)


def test_galeri_kosong_tidak_dapat_mencocokkan() -> None:
    hasil = cocokkan(vektor(1.0, 0.0), Galeri(), ambang=0.363, margin=0.05)
    assert hasil.status is Status.TIDAK_DIKENAL
    assert hasil.teratas is None
    assert "kosong" in hasil.alasan.lower()


def test_skor_di_bawah_ambang_disebut_tidak_dikenal() -> None:
    galeri = Galeri()
    galeri.tambah_identitas("PERSON-001")
    galeri.tambah_sampel("PERSON-001", vektor(1.0, 0.0))

    hasil = cocokkan(vektor(0.0, 1.0), galeri, ambang=0.363, margin=0.05)

    assert hasil.status is Status.TIDAK_DIKENAL
    assert hasil.teratas is not None
    assert hasil.teratas.skor == pytest.approx(0.0, abs=1e-6)


def test_skor_di_atas_ambang_disebut_cocok() -> None:
    galeri = Galeri()
    galeri.tambah_identitas("PERSON-001")
    galeri.tambah_sampel("PERSON-001", vektor(1.0, 0.0))

    hasil = cocokkan(vektor(1.0, 0.0), galeri, ambang=0.363, margin=0.05)

    assert hasil.status is Status.COCOK
    assert hasil.teratas is not None
    assert hasil.teratas.label == "PERSON-001"
    assert hasil.teratas.skor == pytest.approx(1.0, abs=1e-6)


def test_dua_kandidat_berdekatan_menuntut_verifikasi() -> None:
    """Dua identitas sama-sama mirip bukan alasan memilih yang tertinggi."""
    galeri = Galeri()
    galeri.tambah_identitas("PERSON-001")
    galeri.tambah_sampel("PERSON-001", vektor(1.0, 0.0))
    galeri.tambah_identitas("PERSON-002")
    galeri.tambah_sampel("PERSON-002", vektor(0.98, 0.02))

    hasil = cocokkan(vektor(1.0, 0.0), galeri, ambang=0.363, margin=0.05)

    assert hasil.status is Status.PERLU_VERIFIKASI
    assert len(hasil.kandidat) == 2
    assert "terlalu berdekatan" in hasil.alasan


def test_dua_kandidat_dengan_selisih_besar_tetap_cocok() -> None:
    galeri = Galeri()
    galeri.tambah_identitas("PERSON-001")
    galeri.tambah_sampel("PERSON-001", vektor(1.0, 0.0))
    galeri.tambah_identitas("PERSON-002")
    galeri.tambah_sampel("PERSON-002", vektor(0.5, 0.866))

    hasil = cocokkan(vektor(1.0, 0.0), galeri, ambang=0.363, margin=0.05)

    assert hasil.status is Status.COCOK
    assert hasil.teratas is not None
    assert hasil.teratas.label == "PERSON-001"


def test_skor_identitas_memakai_sampel_terbaik_bukan_rata_rata() -> None:
    """Satu sampel yang sangat mirip cukup; rata-rata akan menutupinya."""
    galeri = Galeri()
    galeri.tambah_identitas("PERSON-001")
    galeri.tambah_sampel("PERSON-001", vektor(0.0, 1.0))  # buruk untuk query
    galeri.tambah_sampel("PERSON-001", vektor(1.0, 0.0))  # sangat mirip

    hasil = cocokkan(vektor(1.0, 0.0), galeri, ambang=0.9, margin=0.05)

    assert hasil.status is Status.COCOK
    assert hasil.teratas is not None
    assert hasil.teratas.skor == pytest.approx(1.0, abs=1e-6)


def test_kandidat_diurutkan_menurun() -> None:
    galeri = Galeri()
    for label, arah in (("PERSON-001", (0.2, 0.98)), ("PERSON-002", (1.0, 0.0))):
        galeri.tambah_identitas(label)
        galeri.tambah_sampel(label, vektor(*arah))

    hasil = cocokkan(vektor(1.0, 0.0), galeri, ambang=0.3, margin=0.01)

    skor = [k.skor for k in hasil.kandidat]
    assert skor == sorted(skor, reverse=True)
    assert hasil.kandidat[0].label == "PERSON-002"


def test_galeri_dapat_dikosongkan() -> None:
    galeri = Galeri()
    galeri.tambah_identitas("PERSON-001")
    galeri.tambah_sampel("PERSON-001", vektor(1.0, 0.0))
    assert len(galeri) == 1

    galeri.kosongkan()

    assert len(galeri) == 0
    assert galeri.label_berikutnya() == "PERSON-001"

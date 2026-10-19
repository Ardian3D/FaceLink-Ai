"""Pengujian laporan sesi kalibrasi tanpa kamera.
Vektor di sini sintetis: tujuannya memastikan laporan mencetak angka yang
benar dan seluruh bagian wajib ada, bukan mengukur model. Tidak ada berkas
yang ditulis oleh pengujian ini.
"""
from __future__ import annotations

import numpy as np

from face_engine.calibrate import Penolakan, cetak_laporan
from face_engine.calibration import SesiKalibrasi
from face_engine.gallery import Galeri


def arah(sudut_derajat: float, dimensi: int = 8) -> np.ndarray:
    rad = np.deg2rad(sudut_derajat)
    v = np.zeros(dimensi, dtype=np.float32)
    v[0] = np.cos(rad)
    v[1] = np.sin(rad)
    return v


def sesi_tiga_peserta() -> tuple[SesiKalibrasi, Galeri]:
    sesi = SesiKalibrasi(nama_sesi="uji-laporan")
    galeri = Galeri()
    for label, sudut in (("PERSON-001", 0.0), ("PERSON-002", 90.0), ("PERSON-003", 180.0)):
        p = sesi.daftarkan_peserta(kacamata=(label == "PERSON-002"))
        assert p.label == label
        sesi.catat_persetujuan(label)
        galeri.tambah_identitas(label)
        for offset in (-4.0, 0.0, 4.0):
            galeri.tambah_sampel(label, arah(sudut + offset))
    for i in range(3):
        sesi.tambah_kueri(
            "PERSON-001", arah(2.0 + i), "terang",
            durasi_deteksi_ms=10.0 + i, durasi_embedding_ms=40.0 + i,
            ketajaman=30.0 + i * 5, kecerahan=100.0 + i * 5,
        )
        sesi.tambah_kueri(
            "PERSON-002", arah(92.0 + i), "kacamata",
            durasi_deteksi_ms=11.0 + i, durasi_embedding_ms=41.0 + i,
            ketajaman=28.0 + i * 5, kecerahan=95.0 + i * 5,
        )
        sesi.tambah_kueri(
            None, arah(45.0 + i), "tidak-terdaftar",
            durasi_deteksi_ms=12.0 + i, durasi_embedding_ms=42.0 + i,
            ketajaman=26.0 + i * 5, kecerahan=90.0 + i * 5,
        )
    return sesi, galeri


def test_laporan_memuat_seluruh_bagian_wajib(capsys) -> None:
    sesi, galeri = sesi_tiga_peserta()
    cetak_laporan(sesi, galeri, margin=0.05)
    teks = capsys.readouterr().out
    for bagian in (
        "RINGKASAN SESI",
        "DISTRIBUSI SKOR",
        "SAPUAN AMBANG — data kalibrasi",
        "SAPUAN AMBANG — data evaluasi",
        "PER KONDISI",
        "PENGARUH KUALITAS FRAME",
        "LATENSI",
    ):
        assert bagian in teks


def test_laporan_mencetak_jumlah_bukan_hanya_persentase(capsys) -> None:
    sesi, galeri = sesi_tiga_peserta()
    cetak_laporan(sesi, galeri, margin=0.05)
    teks = capsys.readouterr().out
    assert "(kalibrasi 3, evaluasi 6)" in teks
    assert "peringkat 1:N    : n=6" in teks or "peringkat 1:N    : n=" in teks
    assert "korelasi Spearman ketajaman" in teks
    assert "BUKAN ambang produksi" in teks or "Tidak ada ambang yang memenuhi" in teks


def test_laporan_mencetak_penolakan_kualitas(capsys) -> None:
    sesi, galeri = sesi_tiga_peserta()
    penolakan = Penolakan()
    penolakan.catat("Enrollment", "NO_FACE")
    penolakan.catat("Enrollment", "MULTIPLE_FACES")
    penolakan.catat("Enrollment", "MULTIPLE_FACES")
    cetak_laporan(sesi, galeri, margin=0.05, penolakan=penolakan)
    teks = capsys.readouterr().out
    assert "PENOLAKAN KUALITAS" in teks
    assert "Enrollment                   MULTIPLE_FACES=2  NO_FACE=1" in teks


def test_laporan_tanpa_penolakan_menyatakan_tidak_ada(capsys) -> None:
    sesi, galeri = sesi_tiga_peserta()
    cetak_laporan(sesi, galeri, margin=0.05, penolakan=Penolakan())
    assert "(tidak ada penolakan tercatat)" in capsys.readouterr().out


def test_penolakan_tidak_menghitung_kode_none() -> None:
    p = Penolakan()
    p.catat("Enrollment", None)
    assert p.total == 0
    assert p.ringkas() == {}


def test_laporan_tanpa_kueri_tidak_melempar(capsys) -> None:
    sesi = SesiKalibrasi(nama_sesi="kosong")
    cetak_laporan(sesi, Galeri(), margin=0.05)
    assert "Tidak ada kueri" in capsys.readouterr().out


def test_laporan_baseline_ambigu_tidak_menghitung_penerimaan(capsys) -> None:
    """Ambigu wajah tak dikenal tidak boleh muncul sebagai penerimaan."""
    sesi = SesiKalibrasi(nama_sesi="ambigu")
    p = sesi.daftarkan_peserta()
    sesi.catat_persetujuan(p.label)
    galeri = Galeri()
    for label, sudut in (("PERSON-001", 0.0), ("PERSON-002", 6.0)):
        galeri.tambah_identitas(label)
        galeri.tambah_sampel(label, arah(sudut))
    sesi.tambah_kueri(None, arah(3), "terang")
    cetak_laporan(sesi, galeri, margin=0.10)
    teks = capsys.readouterr().out
    # ambang 0,363 baseline: dua kandidat lolos, margin 0,023 < 0,10 → bukan cocok
    baris = next(b for b in teks.splitlines() if b.strip().startswith("0.363"))
    assert "0/1" in baris
    assert "0.0" in baris

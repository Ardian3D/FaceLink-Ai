"""Pengujian harness sesi kalibrasi.

Yang diuji: persetujuan wajib sebelum sampel, label anonim, pencabutan
persetujuan yang membuang data, dan pembersihan memori saat sesi berakhir.
Tidak ada kamera, model, atau wajah nyata di sini.
"""

from __future__ import annotations

import numpy as np
import pytest

from face_engine.calibration import PENJELASAN_PERSETUJUAN, SesiKalibrasi
from face_engine.errors import GalatEnrollment


def vektor(nilai: float = 1.0) -> np.ndarray:
    return np.array([nilai, 0.0, 0.0, 0.0], dtype=np.float32)


def sesi_dengan_dua_peserta() -> SesiKalibrasi:
    s = SesiKalibrasi(nama_sesi="uji")
    a = s.daftarkan_peserta()
    b = s.daftarkan_peserta()
    s.catat_persetujuan(a.label)
    s.catat_persetujuan(b.label)
    return s


def test_penjelasan_memuat_batas_liveness() -> None:
    """Teks persetujuan harus menyatakan bahwa sistem tidak punya liveness."""
    assert "liveness" in PENJELASAN_PERSETUJUAN.lower()
    assert "tidak ada data yang dikirim ke internet" in PENJELASAN_PERSETUJUAN.lower()


def test_label_selalu_anonim_berurutan() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    assert s.daftarkan_peserta().label == "PERSON-001"
    assert s.daftarkan_peserta().label == "PERSON-002"


def test_peserta_baru_belum_menyetujui() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    p = s.daftarkan_peserta()
    assert p.persetujuan_dicatat is False
    assert p.waktu_persetujuan is None


def test_pencatatan_persetujuan_memberi_waktu_utc() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    p = s.daftarkan_peserta()
    s.catat_persetujuan(p.label)

    assert p.persetujuan_dicatat is True
    assert p.waktu_persetujuan is not None
    assert p.waktu_persetujuan.endswith("+00:00")


def test_tambah_kueri_ditolak_tanpa_persetujuan() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    p = s.daftarkan_peserta()

    with pytest.raises(GalatEnrollment) as info:
        s.tambah_kueri(p.label, vektor(), "terang")

    assert "Status persetujuan belum dicatat" in str(info.value)
    assert s.kueri == []


def test_peserta_menolak_tidak_memblokir_peserta_lain() -> None:
    """Penolakan eksplisit bukan 'belum ditanya'.

    Tanpa pembedaan ini, satu peserta yang menolak membuat seluruh sesi gagal
    saat peserta lain mengambil sampel — perilaku yang sudah direproduksi.
    """
    s = SesiKalibrasi(nama_sesi="uji")
    a = s.daftarkan_peserta()
    b = s.daftarkan_peserta()
    s.catat_persetujuan(a.label)
    s.tolak_persetujuan(b.label)
    s.pastikan_semua_setuju()  # tidak melempar
    s.tambah_kueri(a.label, vektor(), "terang")
    assert [k.label for k in s.kueri] == [a.label]
    assert any("menolak ikut serta" in c for c in s.catatan)


def test_penolakan_tidak_menghapus_peserta_lain() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    a = s.daftarkan_peserta()
    b = s.daftarkan_peserta()
    s.catat_persetujuan(a.label)
    s.tolak_persetujuan(b.label)
    s.tambah_kueri(a.label, vektor(), "terang")
    assert set(s.peserta) == {a.label, b.label}
    assert len(s.kueri) == 1


def test_tolak_persetujuan_menolak_label_asing() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    with pytest.raises(GalatEnrollment):
        s.tolak_persetujuan("PERSON-777")


def test_kueri_untuk_peserta_yang_menolak_ditolak() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    p = s.daftarkan_peserta()
    s.tolak_persetujuan(p.label)
    with pytest.raises(GalatEnrollment) as info:
        s.tambah_kueri(p.label, vektor(), "terang")
    assert "tidak aktif" in str(info.value)
    assert s.kueri == []


def test_tambah_kueri_ditolak_untuk_label_bukan_peserta() -> None:
    s = sesi_dengan_dua_peserta()
    with pytest.raises(GalatEnrollment) as info:
        s.tambah_kueri("PERSON-999", vektor(), "terang")
    assert "bukan peserta" in str(info.value)


def test_pencatatan_persetujuan_menolak_label_asing() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    with pytest.raises(GalatEnrollment):
        s.catat_persetujuan("PERSON-777")


def test_kueri_tak_terdaftar_boleh_tanpa_label() -> None:
    s = sesi_dengan_dua_peserta()
    sampel = s.tambah_kueri(None, vektor(), "tidak-terdaftar")
    assert sampel.label is None
    assert s.jumlah_per_peserta()["unknown"] == 1


def test_pencabutan_persetujuan_membuang_peserta_dan_sampelnya() -> None:
    s = sesi_dengan_dua_peserta()
    a, b = "PERSON-001", "PERSON-002"
    s.tambah_kueri(a, vektor(), "terang")
    s.tambah_kueri(b, vektor(), "terang")
    assert len(s.kueri) == 2

    s.tarik_persetujuan(a)

    assert a not in s.peserta
    assert len(s.kueri) == 1
    assert s.kueri[0].label == b
    assert any("mencabut persetujuan" in c for c in s.catatan)


def test_pencabutan_label_tidak_dikenal_tidak_melempar() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    s.tarik_persetujuan("PERSON-404")  # tidak ada; harus diam, bukan gagal


def test_akhir_sesi_mengosongkan_seluruh_kueri() -> None:
    s = sesi_dengan_dua_peserta()
    s.tambah_kueri("PERSON-001", vektor(), "terang")
    s.tambah_kueri(None, vektor(), "tidak-terdaftar")

    rekap = s.akhiri_sesi()

    assert s.kueri == []
    assert s.selesai is True
    assert rekap["jumlah_kueri"] == 2
    assert rekap["persetujuan_tercatat"] == 2
    assert rekap["per_kondisi"] == {"terang": 1, "tidak-terdaftar": 1}


def test_akhir_sesi_tidak_menyimpan_vektor_dalam_rekap() -> None:
    s = sesi_dengan_dua_peserta()
    s.tambah_kueri("PERSON-001", vektor(), "terang")

    rekap = s.akhiri_sesi()

    for nilai in rekap.values():
        assert not isinstance(nilai, np.ndarray)
    assert "vektor" not in rekap


def test_jumlah_per_kondisi_dan_peserta() -> None:
    s = sesi_dengan_dua_peserta()
    for kondisi in ("terang", "terang", "redup"):
        s.tambah_kueri("PERSON-001", vektor(), kondisi)
    s.tambah_kueri("PERSON-002", vektor(), "terang")

    assert s.jumlah_per_kondisi() == {"redup": 1, "terang": 3}
    assert s.jumlah_per_peserta() == {"PERSON-001": 3, "PERSON-002": 1}


def test_pastikan_semua_setuju_menyebut_label_yang_belum() -> None:
    s = SesiKalibrasi(nama_sesi="uji")
    s.daftarkan_peserta()
    s.daftarkan_peserta()
    s.catat_persetujuan("PERSON-001")

    with pytest.raises(GalatEnrollment) as info:
        s.pastikan_semua_setuju()

    assert "PERSON-002" in str(info.value)


def test_harness_tidak_punya_metode_penyimpanan() -> None:
    """Batas privasi ditegakkan oleh API, bukan hanya oleh niat di dokumentasi."""
    terlarang = {"simpan", "save", "dump", "to_json", "to_csv", "ekspor", "export"}
    tersedia = {nama for nama in dir(SesiKalibrasi) if not nama.startswith("_")}
    assert not (tersedia & terlarang)


def test_kueri_menyimpan_metrik_kualitas_bukan_biometrik() -> None:
    s = sesi_dengan_dua_peserta()
    sampel = s.tambah_kueri(
        "PERSON-001", vektor(), "redup",
        durasi_deteksi_ms=12.0, durasi_embedding_ms=38.0,
        ketajaman=24.5, kecerahan=88.0,
    )
    assert sampel.ketajaman == 24.5
    assert sampel.kecerahan == 88.0
    assert sampel.durasi_total_ms == 50.0


def test_kueri_tak_terdaftar_tidak_melempar_dan_tetap_dibersihkan() -> None:
    s = sesi_dengan_dua_peserta()
    s.tambah_kueri(None, vektor(), "tidak-terdaftar")
    assert s.jumlah_per_peserta() == {"unknown": 1}
    s.akhiri_sesi()
    assert s.kueri == []


def test_pencabutan_tidak_menyisakan_kueri_tak_terdaftar() -> None:
    """Kueri tanpa label bukan milik peserta; pencabutan satu peserta tidak
    boleh menghapus data peserta lain maupun mengubahnya."""
    s = sesi_dengan_dua_peserta()
    s.tambah_kueri(None, vektor(), "tidak-terdaftar")
    s.tambah_kueri("PERSON-002", vektor(), "terang")
    s.tarik_persetujuan("PERSON-001")
    assert [k.label for k in s.kueri] == [None, "PERSON-002"]

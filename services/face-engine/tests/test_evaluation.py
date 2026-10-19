"""Pengujian evaluasi kalibrasi dengan vektor sintetis.

**Penting:** vektor di sini dibuat dengan NumPy, bukan dari wajah siapa pun.
Pengujian ini memeriksa aritmetika evaluasi (penghitungan false match,
false rejection, margin, sapuan ambang), bukan performa model. Hasilnya tidak
boleh dikutip sebagai hasil pengujian manusia.
"""

from __future__ import annotations

import numpy as np
import pytest

from face_engine.evaluation import (
    AMBANG_KANDIDAT,
    Sampel,
    bagi_kueri,
    distribusi_genuine,
    distribusi_impostor,
    distribusi_margin,
    format_tabel,
    kelompok_kualitas,
    korelasi_kualitas,
    nilai_satu_kueri,
    nilai_semua,
    pilih_ambang,
    ringkas,
    ringkas_distribusi,
    ringkas_peringkat,
    sapuan_ambang,
    status_uji,
)
from face_engine.gallery import Galeri
from face_engine.matcher import Status, cocokkan


def arah(sudut_derajat: float, dimensi: int = 8) -> np.ndarray:
    """Vektor satuan pada bidang dua dimensi pertama, sisanya nol.

    Sudut kecil berarti vektor berdekatan (skor tinggi); sudut besar berarti
    berjauhan (skor rendah). Ini memberi kendali tepat atas skor yang diuji.
    """
    rad = np.deg2rad(sudut_derajat)
    v = np.zeros(dimensi, dtype=np.float32)
    v[0] = np.cos(rad)
    v[1] = np.sin(rad)
    return v


def galeri_tiga() -> Galeri:
    """Galeri dengan tiga identitas, masing-masing satu sampel berjarak 90°."""
    g = Galeri()
    for label, sudut in (("PERSON-001", 0.0), ("PERSON-002", 90.0), ("PERSON-003", 180.0)):
        g.tambah_identitas(label)
        g.tambah_sampel(label, arah(sudut))
    return g


# --------------------------------------------------------------- dasar


def test_galeri_kosong_ditolak_dengan_pesan_jelas() -> None:
    with pytest.raises(ValueError) as info:
        nilai_satu_kueri(Galeri(), Sampel("PERSON-001", arah(0)))
    assert "kosong" in str(info.value)


def test_galeri_satu_orang_tidak_memiliki_skor_kedua() -> None:
    g = Galeri()
    g.tambah_identitas("PERSON-001")
    g.tambah_sampel("PERSON-001", arah(0))

    h = nilai_satu_kueri(g, Sampel("PERSON-001", arah(0)))

    assert h.label_teratas == "PERSON-001"
    assert h.skor_teratas == pytest.approx(1.0, abs=1e-5)
    assert h.label_kedua is None
    assert h.margin is None


def test_galeri_beberapa_orang_mengurutkan_dua_teratas() -> None:
    g = galeri_tiga()
    h = nilai_satu_kueri(g, Sampel("PERSON-002", arah(85)))

    assert h.label_teratas == "PERSON-002"
    assert h.label_kedua == "PERSON-001"
    assert h.skor_teratas > h.skor_kedua


def test_kueri_nol_dan_nan_ditolak() -> None:
    g = galeri_tiga()
    with pytest.raises(ValueError) as nol:
        nilai_satu_kueri(g, Sampel("PERSON-001", np.zeros(8, dtype=np.float32)))
    assert "norma nol" in str(nol.value)

    rusak = arah(0).copy()
    rusak[0] = np.nan
    with pytest.raises(ValueError) as nan:
        nilai_satu_kueri(g, Sampel("PERSON-001", rusak))
    assert "NaN" in str(nan.value)


def test_kueri_kosong_menghasilkan_daftar_kosong() -> None:
    assert nilai_semua(galeri_tiga(), []) == []


# ------------------------------------------------------------ penghitungan


def test_genuine_diterima_tidak_dihitung_sebagai_kesalahan() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(5))])

    r = ringkas(hasil, ambang=0.9, margin=0.05)

    assert r.genuine_total == 1
    assert r.genuine_benar == 1
    assert r.false_rejection == 0
    assert r.false_match == 0


def test_skor_di_bawah_ambang_dihitung_false_rejection() -> None:
    g = galeri_tiga()
    # 60° dari sampel terdaftar → cos 0,5; ambang 0,9 menolaknya.
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(60))])

    r = ringkas(hasil, ambang=0.9, margin=0.05)

    assert r.false_rejection == 1
    assert r.genuine_benar == 0
    assert r.frr == pytest.approx(1.0)


def test_identitas_salah_diterima_dihitung_false_match() -> None:
    """Kueri PERSON-002 tetapi galeri menganggap PERSON-001 paling mirip."""
    g = Galeri()
    g.tambah_identitas("PERSON-001")
    g.tambah_sampel("PERSON-001", arah(0))
    g.tambah_identitas("PERSON-002")
    g.tambah_sampel("PERSON-002", arah(90))

    # Kueri dekat PERSON-001 tetapi diberi label PERSON-002 (simulasi salah label).
    hasil = nilai_semua(g, [Sampel("PERSON-002", arah(2))])
    r = ringkas(hasil, ambang=0.5, margin=0.05)

    assert r.false_match == 1
    assert r.genuine_benar == 0


def test_margin_kecil_menghasilkan_ambigu_bukan_penerimaan() -> None:
    g = Galeri()
    # Dua identitas berjarak 6° → skor keduanya sangat dekat.
    for label, sudut in (("PERSON-001", 0.0), ("PERSON-002", 6.0)):
        g.tambah_identitas(label)
        g.tambah_sampel(label, arah(sudut))

    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(3))])
    r = ringkas(hasil, ambang=0.3, margin=0.10)

    assert r.ambigu == 1
    assert r.genuine_benar == 0
    assert r.false_match == 0
    assert r.false_rejection == 0


def test_margin_besar_memilih_kandidat_teratas() -> None:
    g = Galeri()
    for label, sudut in (("PERSON-001", 0.0), ("PERSON-002", 6.0)):
        g.tambah_identitas(label)
        g.tambah_sampel(label, arah(sudut))

    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(1))])
    r = ringkas(hasil, ambang=0.3, margin=0.001)

    assert r.ambigu == 0
    assert r.genuine_benar == 1


def test_wajah_tak_terdaftar_dihitung_terpisah() -> None:
    g = galeri_tiga()
    # Kueri tanpa label benar: yang diukur hanya apakah sistem menerimanya.
    hasil = nilai_semua(g, [Sampel(None, arah(2))])
    r = ringkas(hasil, ambang=0.9, margin=0.05)

    assert r.unknown_total == 1
    assert r.unknown_diterima == 1
    assert r.genuine_total == 0
    assert r.unknown_accept_rate == pytest.approx(1.0)


def test_tak_terdaftar_ditolak_pada_ambang_tinggi() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel(None, arah(45))])
    r = ringkas(hasil, ambang=0.95, margin=0.05)

    assert r.unknown_diterima == 0


def test_batas_ambang_tepat_di_nilai_skor_diterima() -> None:
    """Perbandingan memakai `>=`: skor sama dengan ambang dianggap lolos."""
    g = Galeri()
    g.tambah_identitas("PERSON-001")
    g.tambah_sampel("PERSON-001", arah(0))

    h = nilai_satu_kueri(g, Sampel("PERSON-001", arah(60)))
    skor = h.skor_teratas

    tepat = ringkas([h], ambang=skor, margin=0.05)
    lebih = ringkas([h], ambang=skor + 1e-4, margin=0.05)

    assert tepat.false_rejection == 0
    assert lebih.false_rejection == 1


# ---------------------------------------------------------------- sapuan


def test_sapuan_ambang_mengembalikan_satu_baris_per_kandidat() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(10)), Sampel("PERSON-002", arah(95))])
    sapuan = sapuan_ambang(hasil, (0.3, 0.6, 0.9), margin=0.05)

    assert [r.ambang for r in sapuan] == [0.3, 0.6, 0.9]
    # Ambang naik tidak boleh menurunkan false rejection.
    fr = [r.false_rejection for r in sapuan]
    assert fr == sorted(fr)


def test_format_tabel_memuat_setiap_ambang() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(10))])
    teks = format_tabel(sapuan_ambang(hasil, (0.3, 0.6), margin=0.05))

    assert "0.300" in teks
    assert "0.600" in teks
    assert "ambang" in teks


def test_distribusi_memisahkan_genuine_dan_impostor() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(5))])

    genuine = distribusi_genuine(hasil)
    impostor = distribusi_impostor(hasil)

    assert len(genuine) == 1
    assert genuine[0] > 0.9
    assert len(impostor) == 2  # dua identitas lain
    assert all(s < genuine[0] for s in impostor)


def test_ringkas_distribusi_kosong_tidak_melempar() -> None:
    d = ringkas_distribusi([])
    assert d["n"] == 0
    assert d["median"] is None


# ------------------------------------------------------------- pemisahan


def test_pemisahan_menjaga_kondisi_di_kedua_bagian() -> None:
    kueri = [
        Sampel("PERSON-001", arah(0), kondisi="terang"),
        Sampel("PERSON-001", arah(1), kondisi="terang"),
        Sampel("PERSON-001", arah(2), kondisi="terang"),
        Sampel("PERSON-001", arah(3), kondisi="redup"),
        Sampel("PERSON-001", arah(4), kondisi="redup"),
        Sampel("PERSON-001", arah(5), kondisi="redup"),
    ]
    kalibrasi, evaluasi = bagi_kueri(kueri, rasio_kalibrasi=0.5, seed=7)

    assert len(kalibrasi) + len(evaluasi) == len(kueri)
    assert {s.kondisi for s in kalibrasi} == {"terang", "redup"}
    assert {s.kondisi for s in evaluasi} == {"terang", "redup"}


def test_pemisahan_menolak_rasio_tidak_sah() -> None:
    with pytest.raises(ValueError):
        bagi_kueri([], rasio_kalibrasi=0.0)


def test_pemisahan_selalu_menyisakan_satu_untuk_kondisi_kecil() -> None:
    kueri = [Sampel("PERSON-001", arah(0), kondisi="terang")]
    kalibrasi, evaluasi = bagi_kueri(kueri, rasio_kalibrasi=0.5)

    assert len(kalibrasi) == 1
    assert evaluasi == []


def test_pilih_ambang_mengembalikan_none_bila_tidak_ada_yang_layak() -> None:
    g = galeri_tiga()
    # Semua genuine jauh di bawah ambang terendah pada kandidat.
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(170))])

    assert pilih_ambang(hasil, AMBANG_KANDIDAT, margin=0.05, batas_frr=0.10) is None


def test_pilih_ambang_memilih_yang_terendah_yang_layak() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(2)) for _ in range(5)])

    pilihan = pilih_ambang(hasil, (0.3, 0.5, 0.9), margin=0.05, batas_frr=0.10)

    assert pilihan is not None
    assert pilihan.ambang == 0.3
    assert pilihan.unknown_total == 0  # tidak ada kueri tak terdaftar di sini


def test_pilih_ambang_menolak_kandidat_yang_menerima_identitas_salah() -> None:
    """Ambang dengan false match bukan ambang yang layak, walau FRR-nya nol."""
    g = galeri_tiga()
    # Kueri berlabel PERSON-002 diarahkan dekat PERSON-001: sistem memilih
    # PERSON-001, sehingga percobaan itu false match pada ambang yang lolos.
    hasil = nilai_semua(g, [Sampel("PERSON-002", arah(2))])
    r = ringkas(hasil, ambang=0.25, margin=0.05)
    assert r.false_rejection == 0
    assert r.false_match == 1
    assert pilih_ambang(hasil, AMBANG_KANDIDAT, margin=0.05) is None


# --------------------------------------------------- kesetaraan dengan matcher
def test_status_uji_sama_dengan_matcher_pada_vektor_acak() -> None:
    """Aturan evaluasi harus menghasilkan keputusan identik dengan runtime.

    Dua implementasi terpisah (evaluasi menyimpan skor, matcher menghitung
    ulang) mudah menyimpang saat salah satunya diubah. Pengujian ini
    membandingkan keduanya pada vektor acak untuk beberapa ambang dan margin.
    """
    rng = np.random.default_rng(1234)
    g = Galeri()
    for i in range(4):
        label = f"PERSON-{i + 1:03d}"
        g.tambah_identitas(label)
        for _ in range(3):
            v = rng.normal(size=16).astype(np.float32)
            g.tambah_sampel(label, v / np.linalg.norm(v))
    for _ in range(20):
        v = rng.normal(size=16).astype(np.float32)
        v /= np.linalg.norm(v)
        h = nilai_satu_kueri(g, Sampel("PERSON-001", v))
        for ambang in (0.0, 0.2, 0.5, 0.9):
            for margin in (0.0, 0.05, 0.2):
                runtime = cocokkan(v, g, ambang, margin)
                assert status_uji(h, ambang, margin) is runtime.status


def test_ambigu_tak_dikenal_tidak_dihitung_sebagai_penerimaan() -> None:
    """Dua kandidat berdekatan untuk wajah tak dikenal → perlu verifikasi.

    Sebelum aturan ini disamakan dengan `matcher`, skor teratas yang lolos
    ambang dihitung sebagai penerimaan walau sistem tidak menyatakan identitas.
    """
    g = Galeri()
    for label, sudut in (("PERSON-001", 0.0), ("PERSON-002", 6.0)):
        g.tambah_identitas(label)
        g.tambah_sampel(label, arah(sudut))
    v = arah(3)
    assert cocokkan(v, g, 0.3, 0.10).status is Status.PERLU_VERIFIKASI
    hasil = nilai_semua(g, [Sampel(None, v)])
    r = ringkas(hasil, ambang=0.3, margin=0.10)
    assert r.unknown_total == 1
    assert r.unknown_diterima == 0


def test_seri_skor_diurutkan_menurut_label() -> None:
    """Dua identitas identik tidak boleh membuat urutan bergantung pada dict."""
    g = Galeri()
    for label in ("PERSON-002", "PERSON-001"):
        g.tambah_identitas(label)
        g.tambah_sampel(label, arah(0))
    h1 = nilai_satu_kueri(g, Sampel("PERSON-001", arah(0)))
    h2 = nilai_satu_kueri(g, Sampel("PERSON-001", arah(0)))
    assert h1.label_teratas == "PERSON-001"
    assert h1.label_kedua == "PERSON-002"
    assert h1 == h2


# ------------------------------------------------------- distribusi & margin
def test_distribusi_impostor_memuat_seluruh_identitas_lain() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel("PERSON-001", arah(5))])
    assert len(distribusi_impostor(hasil)) == 2
    # Wajah tak dikenal: seluruh identitas galeri adalah impostor.
    tak_dikenal = nilai_semua(g, [Sampel(None, arah(5))])
    assert len(distribusi_impostor(tak_dikenal)) == 3


def test_genuine_memakai_skor_identitas_benar_bukan_teratas() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(g, [Sampel("PERSON-002", arah(2))])
    genuine = distribusi_genuine(hasil)
    assert len(genuine) == 1
    assert genuine[0] < hasil[0].skor_teratas


def test_distribusi_margin_hanya_untuk_galeri_lebih_dari_satu() -> None:
    satu = Galeri()
    satu.tambah_identitas("PERSON-001")
    satu.tambah_sampel("PERSON-001", arah(0))
    assert distribusi_margin(nilai_semua(satu, [Sampel("PERSON-001", arah(0))])) == []
    tiga = nilai_semua(galeri_tiga(), [Sampel("PERSON-001", arah(0))])
    assert len(distribusi_margin(tiga)) == 1


def test_ringkas_peringkat_menghitung_top1_dan_top2() -> None:
    g = galeri_tiga()
    hasil = nilai_semua(
        g,
        [
            Sampel("PERSON-001", arah(2)),    # top-1 benar
            Sampel("PERSON-002", arah(2)),    # label salah: bukan top-1
            Sampel("PERSON-003", arah(179)),  # top-1 benar
        ],
    )
    r = ringkas_peringkat(hasil)
    assert r["n"] == 3
    assert r["top1"] == 2
    assert r["top2"] == 3
    assert ringkas_peringkat([]) == {"n": 0, "top1": 0, "top2": 0, "median": None}


# ------------------------------------------------------------ kualitas frame
def test_korelasi_kualitas_mendeteksi_skor_turun_pada_frame_buram() -> None:
    g = galeri_tiga()
    kueri = [
        Sampel("PERSON-001", arah(60), ketajaman=5.0),
        Sampel("PERSON-001", arah(40), ketajaman=10.0),
        Sampel("PERSON-001", arah(20), ketajaman=20.0),
        Sampel("PERSON-001", arah(5), ketajaman=40.0),
    ]
    hasil = nilai_semua(g, kueri)
    k = korelasi_kualitas(hasil, "ketajaman")
    assert k["n"] == 4
    assert k["rho"] == pytest.approx(1.0)


def test_korelasi_kualitas_mengembalikan_none_saat_data_kurang() -> None:
    hasil = nilai_semua(galeri_tiga(), [Sampel("PERSON-001", arah(0), ketajaman=30.0)])
    assert korelasi_kualitas(hasil)["rho"] is None
    with pytest.raises(ValueError):
        korelasi_kualitas(hasil, "deteksi")


def test_kelompok_kualitas_membagi_data_dan_melaporkan_rentang() -> None:
    g = galeri_tiga()
    kueri = [
        Sampel("PERSON-001", arah(60 - i * 10), kecerahan=50.0 + i * 30, ketajaman=5.0 + i)
        for i in range(6)
    ]
    hasil = nilai_semua(g, kueri)
    kelompok = kelompok_kualitas(hasil, "kecerahan", jumlah=3)
    assert len(kelompok) == 3
    assert sum(k["n"] for k in kelompok) == 6
    assert kelompok[0]["min_metrik"] == 50.0
    assert kelompok[-1]["maks_metrik"] == 200.0
    with pytest.raises(ValueError):
        kelompok_kualitas(hasil, jumlah=1)


def test_sampel_menjumlahkan_durasi_hanya_bila_lengkap() -> None:
    assert Sampel(None, arah(0), durasi_deteksi_ms=10, durasi_embedding_ms=40).durasi_total_ms == 50
    assert Sampel(None, arah(0), durasi_deteksi_ms=10).durasi_total_ms is None


def test_sampel_tanpa_metrik_kualitas_tidak_melempar() -> None:
    hasil = nilai_semua(galeri_tiga(), [Sampel("PERSON-001", arah(0))])
    assert hasil[0].ketajaman is None
    assert korelasi_kualitas(hasil)["rho"] is None
    assert kelompok_kualitas(hasil) == []

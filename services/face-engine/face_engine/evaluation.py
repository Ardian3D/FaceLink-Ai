"""Pengukuran performa pengenalan wajah — murni angka, tanpa kamera.

Modul ini menjawab pertanyaan yang **berbeda** dari `matcher.py`:

  - `matcher.py` menjawab "siapa kandidatnya untuk satu wajah ini".
  - `evaluation.py` menjawab "seberapa sering keputusan itu benar atau salah
    pada sekumpulan percobaan, untuk beberapa ambang sekaligus".

Pemisahan ini penting karena `matcher` dipakai jalur runtime dan tidak boleh
membawa kode statistik, sedangkan evaluasi harus dapat diuji dengan vektor
sintetis tanpa membuka kamera atau memuat model.

**Yang tidak dilakukan modul ini:** menyimpan apa pun. Ia hanya mengembalikan
angka agregat. Tidak ada label peserta di luar label anonim `PERSON-00N` yang
dibuat pemanggil, dan tidak ada vektor yang keluar dari proses.

**Kesetaraan aturan.** Keputusan diterima/ditolak/ambigu dihitung oleh
`status_uji()`, yang meniru urutan pemeriksaan `matcher.cocokkan()` persis.
Laporan karena itu menggambarkan perilaku sistem yang sebenarnya, bukan model
statistik lain yang kebetulan mirip.
"""
from __future__ import annotations

import math
from dataclasses import dataclass, field
from statistics import median

import numpy as np

from .gallery import Galeri
from .matcher import Status

#: Kandidat ambang yang dibandingkan pada laporan. 0,363 adalah baseline
#: dokumentasi OpenCV Zoo (lihat `config.MATCH_THRESHOLD`), bukan hasil ukur kita.
AMBANG_KANDIDAT: tuple[float, ...] = (0.25, 0.30, 0.363, 0.40, 0.45, 0.50, 0.55, 0.60)


@dataclass(frozen=True)
class Sampel:
    """Satu potongan data uji: satu embedding dengan label kebenaran.

    `label` bernilai `None` untuk wajah yang **tidak** terdaftar di galeri
    (orang lain, atau wajah yang sengaja tidak dienroll). Sampel seperti itu
    dipakai untuk mengukur penerimaan palsu terhadap orang tak dikenal.

    `ketajaman` dan `kecerahan` adalah metrik kualitas frame, **bukan**
    biometrik. Keduanya hanya dipakai untuk melihat apakah skor turun pada
    frame buram atau gelap; nilainya tidak dapat dipakai mengenali orang.
    """

    label: str | None
    vektor: np.ndarray
    kondisi: str = "netral"
    durasi_deteksi_ms: float | None = None
    durasi_embedding_ms: float | None = None
    ketajaman: float | None = None
    kecerahan: float | None = None

    @property
    def durasi_total_ms(self) -> float | None:
        if self.durasi_deteksi_ms is None or self.durasi_embedding_ms is None:
            return None
        return self.durasi_deteksi_ms + self.durasi_embedding_ms


@dataclass(frozen=True)
class HasilUji:
    """Satu percobaan: satu kueri dibandingkan dengan **seluruh** galeri.

    `skor` menyimpan kemiripan terbaik untuk setiap identitas, urut menurun.
    Menyimpan seluruh daftar — bukan hanya dua teratas — membuat distribusi
    impostor lengkap dan margin tetap dapat dihitung tanpa kehilangan data.
    Galeri kecil tetap murah: jumlah identitas pada pengujian ini satuan.
    """

    label_benar: str | None
    kondisi: str
    skor: tuple[tuple[str, float], ...]
    ketajaman: float | None = None
    kecerahan: float | None = None

    @property
    def label_teratas(self) -> str | None:
        return self.skor[0][0] if self.skor else None

    @property
    def skor_teratas(self) -> float:
        return self.skor[0][1] if self.skor else 0.0

    @property
    def label_kedua(self) -> str | None:
        return self.skor[1][0] if len(self.skor) > 1 else None

    @property
    def skor_kedua(self) -> float | None:
        return self.skor[1][1] if len(self.skor) > 1 else None

    @property
    def margin(self) -> float | None:
        """Selisih skor teratas dan kedua; `None` bila galeri hanya satu orang."""
        if self.skor_kedua is None:
            return None
        return self.skor_teratas - self.skor_kedua

    @property
    def benar(self) -> bool:
        return self.label_benar is not None and self.label_teratas == self.label_benar

    def skor_untuk(self, label: str | None) -> float | None:
        """Kemiripan terhadap satu identitas, atau `None` bila tidak ada di galeri."""
        if label is None:
            return None
        for nama, nilai in self.skor:
            if nama == label:
                return nilai
        return None


def status_uji(h: HasilUji, ambang: float, margin: float) -> Status:
    """Meniru `matcher.cocokkan()` untuk hasil yang sudah dihitung.

    Urutannya sama dan sengaja tidak diubah: skor di bawah ambang ditolak lebih
    dahulu; baru kemudian dua kandidat yang sama-sama lolos tetapi terlalu
    berdekatan dianggap ambigu. Wajah tak dikenal yang ambigu **tidak** dihitung
    sebagai penerimaan, karena sistem tidak menyatakan identitas apa pun.
    """
    if not h.skor or h.skor_teratas < ambang:
        return Status.TIDAK_DIKENAL
    if h.skor_kedua is not None and h.skor_kedua >= ambang and h.margin is not None:
        if h.margin < margin:
            return Status.PERLU_VERIFIKASI
    return Status.COCOK


@dataclass
class Ringkasan:
    """Angka agregat untuk satu ambang dan satu margin."""

    ambang: float
    margin: float
    genuine_total: int = 0
    genuine_benar: int = 0
    false_rejection: int = 0
    false_match: int = 0
    ambigu: int = 0
    unknown_total: int = 0
    unknown_diterima: int = 0
    impostor_pasangan: int = 0
    impostor_di_atas: int = 0
    per_kondisi: dict[str, "RingkasanKondisi"] = field(default_factory=dict)

    @property
    def frr(self) -> float | None:
        """False rejection rate: genuine yang ditolak, dibagi seluruh genuine."""
        if self.genuine_total == 0:
            return None
        return self.false_rejection / self.genuine_total

    @property
    def far_pasangan(self) -> float | None:
        """False accept rate tingkat pasangan (impostor), bukan tingkat percobaan."""
        if self.impostor_pasangan == 0:
            return None
        return self.impostor_di_atas / self.impostor_pasangan

    @property
    def false_match_rate(self) -> float | None:
        """False match tingkat percobaan: menerima identitas yang salah."""
        if self.genuine_total == 0:
            return None
        return self.false_match / self.genuine_total

    @property
    def unknown_accept_rate(self) -> float | None:
        if self.unknown_total == 0:
            return None
        return self.unknown_diterima / self.unknown_total


@dataclass
class RingkasanKondisi:
    """Angka per kondisi pengujian, untuk melihat pengaruh kacamata/cahaya."""

    genuine: int = 0
    benar: int = 0
    false_rejection: int = 0
    false_match: int = 0
    ambigu: int = 0
    skor: list[float] = field(default_factory=list)

    @property
    def median_skor(self) -> float | None:
        return median(self.skor) if self.skor else None


def _periksa_vektor(vektor: np.ndarray, asal: str) -> np.ndarray:
    """Menolak vektor yang tidak dapat dibandingkan, dengan alasan yang jelas."""
    a = np.asarray(vektor, dtype=np.float32).reshape(-1)
    if a.size == 0:
        raise ValueError(f"{asal}: vektor kosong.")
    if not np.all(np.isfinite(a)):
        raise ValueError(f"{asal}: vektor memuat NaN atau tak hingga.")
    norma = float(np.linalg.norm(a))
    if norma == 0.0:
        raise ValueError(f"{asal}: vektor bernorma nol sehingga cosine tidak terdefinisi.")
    return a / norma


def nilai_satu_kueri(galeri: Galeri, sampel: Sampel) -> HasilUji:
    """Menghitung kemiripan kueri terhadap setiap identitas di galeri.

    Fungsi ini **tidak** memakai ambang: ia hanya mengumpulkan skor. Ambang
    diterapkan terpisah di `ringkas`, supaya satu kali perhitungan dapat dipakai
    untuk seluruh kandidat ambang tanpa mengulang inferensi.
    """
    if len(galeri) == 0:
        raise ValueError("Galeri kosong; tidak ada yang dapat dibandingkan.")
    vektor = _periksa_vektor(sampel.vektor, f"kueri {sampel.label or 'unknown'}")
    skor: list[tuple[str, float]] = []
    for identitas in galeri.semua():
        if not identitas.embeddings:
            continue
        terbaik = max(
            float(np.dot(vektor, _periksa_vektor(s, f"galeri {identitas.label}")))
            for s in identitas.embeddings
        )
        skor.append((identitas.label, terbaik))
    if not skor:
        raise ValueError("Galeri tidak berisi sampel; tidak ada yang dapat dibandingkan.")
    # Urutan tie-break memakai label supaya hasil deterministik ketika skor sama.
    skor.sort(key=lambda pasangan: (-pasangan[1], pasangan[0]))
    return HasilUji(
        label_benar=sampel.label,
        kondisi=sampel.kondisi,
        skor=tuple(skor),
        ketajaman=sampel.ketajaman,
        kecerahan=sampel.kecerahan,
    )


def nilai_semua(galeri: Galeri, kueri: list[Sampel]) -> list[HasilUji]:
    if not kueri:
        return []
    return [nilai_satu_kueri(galeri, s) for s in kueri]


def ringkas(hasil: list[HasilUji], ambang: float, margin: float) -> Ringkasan:
    """Menerapkan satu ambang + margin pada hasil yang sudah dihitung.

    Klasifikasi tiap percobaan memakai `status_uji()`, sehingga aturannya sama
    dengan `matcher.cocokkan()`:
      - skor teratas < ambang                     → `tidak_dikenal`
      - skor kedua >= ambang dan margin < batas   → `perlu_verifikasi`
      - selain itu                                → cocok dengan kandidat teratas
    """
    r = Ringkasan(ambang=ambang, margin=margin)
    for h in hasil:
        status = status_uji(h, ambang, margin)
        if h.label_benar is None:
            # Wajah tak dikenal: satu-satunya pertanyaan adalah apakah sistem
            # menerimanya. `perlu_verifikasi` tidak dihitung sebagai penerimaan,
            # karena sistem memang tidak menyatakan identitas.
            r.unknown_total += 1
            if status is Status.COCOK:
                r.unknown_diterima += 1
            continue
        kondisi = r.per_kondisi.setdefault(h.kondisi, RingkasanKondisi())
        r.genuine_total += 1
        kondisi.genuine += 1
        skor_benar = h.skor_untuk(h.label_benar)
        if skor_benar is not None:
            kondisi.skor.append(skor_benar)
        if status is Status.TIDAK_DIKENAL:
            r.false_rejection += 1
            kondisi.false_rejection += 1
            continue
        if status is Status.PERLU_VERIFIKASI:
            r.ambigu += 1
            kondisi.ambigu += 1
            continue
        if h.benar:
            r.genuine_benar += 1
            kondisi.benar += 1
        else:
            r.false_match += 1
            kondisi.false_match += 1
    # Tingkat pasangan impostor dihitung terpisah. Untuk kueri genuine, setiap
    # identitas selain label benar adalah pasangan impostor; untuk wajah tak
    # dikenal, seluruh identitas galeri adalah pasangan impostor.
    for h in hasil:
        for kandidat in _skor_impostor(h):
            r.impostor_pasangan += 1
            if kandidat >= ambang:
                r.impostor_di_atas += 1
    return r


def _skor_impostor(h: HasilUji) -> list[float]:
    """Seluruh skor identitas yang bukan label benar pada satu percobaan.

    Untuk wajah tak dikenal (`label_benar is None`), semua identitas galeri
    adalah orang lain, sehingga semuanya dihitung sebagai pasangan impostor.
    """
    return [skor for label, skor in h.skor if label != h.label_benar]


def sapuan_ambang(
    hasil: list[HasilUji],
    ambang_kandidat: tuple[float, ...] = AMBANG_KANDIDAT,
    margin: float = 0.05,
) -> list[Ringkasan]:
    """Menghitung ringkasan untuk setiap kandidat ambang pada data yang sama."""
    return [ringkas(hasil, a, margin) for a in ambang_kandidat]


def distribusi_genuine(hasil: list[HasilUji]) -> list[float]:
    """Skor identitas **benar** untuk setiap percobaan genuine.

    Memakai skor identitas benar, bukan skor kandidat teratas: bila sistem
    memilih orang lain, skor itu adalah false match, bukan genuine similarity.
    """
    nilai: list[float] = []
    for h in hasil:
        skor = h.skor_untuk(h.label_benar)
        if skor is not None:
            nilai.append(skor)
    return nilai


def distribusi_impostor(hasil: list[HasilUji]) -> list[float]:
    """Skor pasangan impostor dari seluruh percobaan (genuine dan tak dikenal)."""
    nilai: list[float] = []
    for h in hasil:
        nilai.extend(_skor_impostor(h))
    return nilai


def distribusi_margin(hasil: list[HasilUji]) -> list[float]:
    """Margin top-1 terhadap top-2 untuk galeri yang berisi minimal dua orang."""
    return [h.margin for h in hasil if h.margin is not None]


def peringkat_identitas_benar(hasil: list[HasilUji]) -> list[int]:
    """Peringkat identitas benar pada setiap percobaan genuine (1 = top-1).

    Peringkat hanya dihitung bila label benar memang ada di galeri. Nilai ini
    menjawab pertanyaan identifikasi 1:N — berapa sering orang yang benar berada
    di posisi pertama — tanpa perlu memilih ambang terlebih dahulu.
    """
    peringkat: list[int] = []
    for h in hasil:
        if h.label_benar is None:
            continue
        for i, (label, _) in enumerate(h.skor):
            if label == h.label_benar:
                peringkat.append(i + 1)
                break
    return peringkat


def ringkas_peringkat(hasil: list[HasilUji]) -> dict[str, float | int | None]:
    """Hitungan top-1/top-2 dan peringkat median untuk percobaan genuine."""
    peringkat = peringkat_identitas_benar(hasil)
    if not peringkat:
        return {"n": 0, "top1": 0, "top2": 0, "median": None}
    return {
        "n": len(peringkat),
        "top1": sum(1 for p in peringkat if p == 1),
        "top2": sum(1 for p in peringkat if p <= 2),
        "median": float(np.median(peringkat)),
    }



def ringkas_distribusi(nilai: list[float]) -> dict[str, float | int | None]:
    """Ringkasan lima angka + jumlah, tanpa asumsi distribusi."""
    if not nilai:
        return {"n": 0, "min": None, "median": None, "maks": None, "rata": None}
    a = np.asarray(nilai, dtype=np.float64)
    return {
        "n": int(a.size),
        "min": float(a.min()),
        "median": float(np.median(a)),
        "maks": float(a.max()),
        "rata": float(a.mean()),
    }


def bagi_kueri(
    kueri: list[Sampel], rasio_kalibrasi: float = 0.5, seed: int = 20261019
) -> tuple[list[Sampel], list[Sampel]]:
    """Memisahkan kueri menjadi bagian kalibrasi dan bagian evaluasi.

    Pemisahan dilakukan **per kondisi** agar kedua bagian memuat campuran
    kondisi yang sebanding. Dengan jumlah sampel kecil, pemisahan ini tidak
    menghasilkan kesimpulan yang bermakna secara statistik — keterbatasan itu
    harus dinyatakan di laporan, bukan disembunyikan.
    """
    if not 0.0 < rasio_kalibrasi < 1.0:
        raise ValueError("rasio_kalibrasi harus di antara 0 dan 1.")
    rng = np.random.default_rng(seed)
    kalibrasi: list[Sampel] = []
    evaluasi: list[Sampel] = []
    per_kondisi: dict[str, list[Sampel]] = {}
    for s in kueri:
        per_kondisi.setdefault(s.kondisi, []).append(s)
    for kondisi in sorted(per_kondisi):
        kelompok = list(per_kondisi[kondisi])
        urutan = rng.permutation(len(kelompok))
        potong = max(1, int(math.floor(len(kelompok) * rasio_kalibrasi)))
        for i, indeks in enumerate(urutan):
            (kalibrasi if i < potong else evaluasi).append(kelompok[indeks])
    return kalibrasi, evaluasi


def pilih_ambang(
    hasil_kalibrasi: list[HasilUji],
    ambang_kandidat: tuple[float, ...] = AMBANG_KANDIDAT,
    margin: float = 0.05,
    batas_frr: float = 0.10,
) -> Ringkasan | None:
    """Memilih ambang **hanya** dari bagian kalibrasi.

    Aturan pemilihan sengaja sederhana dan dapat diaudit. Ambang layak harus:
      - false rejection <= `batas_frr`,
      - **tidak** menerima satu pun wajah tak dikenal,
      - **tidak** menghasilkan satu pun false match.

    Syarat terakhir penting: ambang yang "tidak menolak siapa pun" tetapi
    menerima orang yang salah bukan ambang yang layak, sekalipun FRR-nya nol.
    Bila tidak ada yang memenuhi, kembalikan `None` — itu tanda data terlalu
    kecil atau ambang tidak layak, bukan alasan menurunkan standar. Memilih
    ambang terendah yang layak juga bukan jaminan angka itu aman untuk
    produksi: Q-07 tetap terbuka.
    """
    layak = [
        r
        for r in sapuan_ambang(hasil_kalibrasi, ambang_kandidat, margin)
        if (r.frr or 0.0) <= batas_frr
        and r.unknown_diterima == 0
        and r.false_match == 0
    ]
    if not layak:
        return None
    return min(layak, key=lambda r: r.ambang)


def _peringkat(nilai: np.ndarray) -> np.ndarray:
    """Peringkat rata-rata untuk nilai yang sama (dasar korelasi Spearman)."""
    urutan = np.argsort(nilai, kind="stable")
    peringkat = np.empty(nilai.size, dtype=np.float64)
    i = 0
    while i < nilai.size:
        j = i
        while j + 1 < nilai.size and nilai[urutan[j + 1]] == nilai[urutan[i]]:
            j += 1
        rata = (i + j) / 2.0 + 1.0
        peringkat[urutan[i : j + 1]] = rata
        i = j + 1
    return peringkat


def korelasi_kualitas(
    hasil: list[HasilUji], metrik: str = "ketajaman"
) -> dict[str, float | int | str | None]:
    """Korelasi Spearman antara satu metrik kualitas frame dan skor genuine.

    Dipakai untuk pertanyaan "apakah skor turun saat frame buram atau gelap".
    Korelasi pada belasan sampel hanya indikasi, bukan bukti; nilainya tidak
    boleh disebut koefisien akurasi.
    """
    if metrik not in {"ketajaman", "kecerahan"}:
        raise ValueError("metrik harus 'ketajaman' atau 'kecerahan'.")
    pasangan = [
        (float(getattr(h, metrik)), h.skor_untuk(h.label_benar))
        for h in hasil
        if h.label_benar is not None and getattr(h, metrik) is not None
    ]
    pasangan = [(k, s) for k, s in pasangan if s is not None]
    if len(pasangan) < 3:
        return {"metrik": metrik, "n": len(pasangan), "rho": None}
    kualitas = np.asarray([k for k, _ in pasangan], dtype=np.float64)
    skor = np.asarray([s for _, s in pasangan], dtype=np.float64)
    rk = _peringkat(kualitas)
    rs = _peringkat(skor)
    if rk.std() == 0.0 or rs.std() == 0.0:
        return {"metrik": metrik, "n": len(pasangan), "rho": None}
    rho = float(np.corrcoef(rk, rs)[0, 1])
    return {"metrik": metrik, "n": len(pasangan), "rho": rho}


def kelompok_kualitas(
    hasil: list[HasilUji], metrik: str = "ketajaman", jumlah: int = 3
) -> list[dict[str, float | int | None]]:
    """Membagi percobaan genuine menjadi beberapa kelompok menurut kualitas.

    Setiap kelompok melaporkan rentang metrik dan median skor identitas benar.
    Pembagian berdasarkan urutan, bukan ambang kualitas tetap, supaya hasil
    tidak menyiratkan bahwa ambang kualitas kami adalah angka universal.
    """
    if metrik not in {"ketajaman", "kecerahan"}:
        raise ValueError("metrik harus 'ketajaman' atau 'kecerahan'.")
    if jumlah < 2:
        raise ValueError("jumlah kelompok minimal 2.")
    pasangan = [
        (float(getattr(h, metrik)), h.skor_untuk(h.label_benar))
        for h in hasil
        if h.label_benar is not None and getattr(h, metrik) is not None
    ]
    pasangan = sorted((k, s) for k, s in pasangan if s is not None)
    if len(pasangan) < jumlah:
        return []
    batas = [round(i * len(pasangan) / jumlah) for i in range(jumlah + 1)]
    hasil_kelompok: list[dict[str, float | int | None]] = []
    for i in range(jumlah):
        potongan = pasangan[batas[i] : batas[i + 1]]
        if not potongan:
            continue
        skor = [s for _, s in potongan]
        hasil_kelompok.append(
            {
                "metrik": metrik,
                "n": len(potongan),
                "min_metrik": potongan[0][0],
                "maks_metrik": potongan[-1][0],
                "median_skor": float(np.median(skor)),
            }
        )
    return hasil_kelompok


def format_tabel(sapuan: list[Ringkasan]) -> str:
    """Tabel teks untuk laporan; sengaja tanpa warna dan tanpa grafik."""
    baris = [
        f"{'ambang':>7}  {'margin':>6}  {'FR':>9}  {'FM':>7}  {'unknown':>9}  "
        f"{'ambigu':>9}  {'FRR%':>6}  {'FM%':>6}  {'UNK%':>6}",
        "-" * 7
        + "  "
        + "-" * 6
        + "  "
        + "-" * 9
        + "  "
        + "-" * 7
        + "  "
        + "-" * 9
        + "  "
        + "-" * 9
        + "  "
        + "-" * 6
        + "  "
        + "-" * 6
        + "  "
        + "-" * 6,
    ]

    def persen(nilai: float | None) -> str:
        return "-" if nilai is None else f"{nilai * 100:5.1f}"

    for r in sapuan:
        baris.append(
            f"{r.ambang:7.3f}  {r.margin:6.3f}  "
            f"{r.false_rejection:3d}/{r.genuine_total:<5d}  "
            f"{r.false_match:3d}/{r.genuine_total:<3d}  "
            f"{r.unknown_diterima:4d}/{r.unknown_total:<4d}  "
            f"{r.ambigu:3d}/{r.genuine_total:<5d}  "
            f"{persen(r.frr):>6}  {persen(r.false_match_rate):>6}  "
            f"{persen(r.unknown_accept_rate):>6}"
        )
    return "\n".join(baris)


def format_distribusi(nama: str, nilai: list[float]) -> str:
    d = ringkas_distribusi(nilai)
    if d["n"] == 0:
        return f"{nama:<10}: (tidak ada data)"
    return (
        f"{nama:<10}: n={d['n']:<4d} min={d['min']:.3f} median={d['median']:.3f} "
        f"maks={d['maks']:.3f} rata={d['rata']:.3f}"
    )

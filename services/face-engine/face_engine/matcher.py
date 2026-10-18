"""Pencocokan embedding dan penentuan status kandidat.

Modul ini **tidak** membuat keputusan absensi. Ia hanya menjawab: siapa kandidat
terdekat, seberapa mirip, dan apakah hasilnya cukup jelas untuk disebut cocok.
Keputusan domain tetap milik backend Go (D-027), dan ambang di sini adalah
ambang **uji**, bukan angka produksi.
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum

import numpy as np

from .gallery import Galeri


class Status(str, Enum):
    """Tiga hasil yang dibedakan, sesuai `docs/API_CONTRACT.md` §7."""

    COCOK = "cocok"
    TIDAK_DIKENAL = "tidak_dikenal"
    PERLU_VERIFIKASI = "perlu_verifikasi"


@dataclass(frozen=True)
class Kandidat:
    label: str
    skor: float


@dataclass(frozen=True)
class HasilCocok:
    status: Status
    kandidat: list[Kandidat]
    ambang: float
    margin: float
    alasan: str = ""

    @property
    def teratas(self) -> Kandidat | None:
        return self.kandidat[0] if self.kandidat else None


def cocokkan(
    query: np.ndarray,
    galeri: Galeri,
    ambang: float,
    margin: float,
) -> HasilCocok:
    """Membandingkan satu embedding dengan seluruh sampel di galeri.

    Skor satu identitas adalah kemiripan **terbaik** terhadap salah satu
    sampelnya, bukan rata-rata: satu sampel yang sangat mirip sudah cukup untuk
    menandai kandidat, dan rata-rata akan menyembunyikan variasi antar sampel
    yang justru ingin diamati pada prototype ini.

    Hasil `PERLU_VERIFIKASI` dipakai ketika dua identitas berbeda sama-sama
    melewati ambang dan selisihnya lebih kecil dari `margin`. Pada keadaan itu
    sistem **tidak** boleh memilih yang tertinggi dan berpura-pura yakin.
    """
    if len(galeri) == 0:
        return HasilCocok(Status.TIDAK_DIKENAL, [], ambang, margin, "Galeri kosong.")

    skor_terbaik: list[Kandidat] = []
    for identitas in galeri.semua():
        if not identitas.embeddings:
            continue
        terbaik = max(float(np.dot(query, sampel)) for sampel in identitas.embeddings)
        skor_terbaik.append(Kandidat(identitas.label, terbaik))

    if not skor_terbaik:
        return HasilCocok(Status.TIDAK_DIKENAL, [], ambang, margin, "Galeri tidak berisi sampel.")

    skor_terbaik.sort(key=lambda k: k.skor, reverse=True)
    teratas = skor_terbaik[0]

    if teratas.skor < ambang:
        return HasilCocok(
            Status.TIDAK_DIKENAL,
            skor_terbaik,
            ambang,
            margin,
            f"Skor tertinggi {teratas.skor:.3f} di bawah ambang {ambang:.3f}.",
        )

    if len(skor_terbaik) > 1:
        kedua = skor_terbaik[1]
        if kedua.skor >= ambang and (teratas.skor - kedua.skor) < margin:
            return HasilCocok(
                Status.PERLU_VERIFIKASI,
                skor_terbaik,
                ambang,
                margin,
                f"Dua kandidat terlalu berdekatan "
                f"({teratas.label} {teratas.skor:.3f} vs {kedua.label} {kedua.skor:.3f}).",
            )

    return HasilCocok(
        Status.COCOK,
        skor_terbaik,
        ambang,
        margin,
        f"Kandidat {teratas.label} melewati ambang dengan skor {teratas.skor:.3f}.",
    )

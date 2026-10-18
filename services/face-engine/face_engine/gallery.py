"""Galeri identitas uji — **hanya di RAM**.

Tidak ada metode `simpan`, `muat`, `dump`, atau `to_json` di kelas ini, dan itu
disengaja. FL-CV01 tidak boleh mempersistensikan template biometrik: skema
penyimpanan permanen (AES-256-GCM, D-022) dan pengelolaan kunci (D-030) belum
diimplementasikan. Menambahkan serialisasi di sini akan melewati dua keputusan
yang belum punya kode.

Galeri menyimpan beberapa embedding per identitas, bukan satu rata-rata. Dengan
begitu skor dapat dilaporkan sebagai kemiripan terbaik terhadap salah satu
sampel, dan penyimpangan antar sampel tetap terlihat saat pengujian.
"""

from __future__ import annotations

from dataclasses import dataclass, field

import numpy as np

from .errors import GalatEnrollment


@dataclass
class Identitas:
    """Satu peserta uji anonim."""

    label: str
    embeddings: list[np.ndarray] = field(default_factory=list)

    def tambah(self, vektor: np.ndarray) -> None:
        self.embeddings.append(vektor)


class Galeri:
    """Kumpulan identitas uji dalam memori proses."""

    def __init__(self) -> None:
        self._identitas: dict[str, Identitas] = {}

    def __len__(self) -> int:
        return len(self._identitas)

    def __contains__(self, label: str) -> bool:
        return label in self._identitas

    @property
    def label_terpakai(self) -> list[str]:
        return sorted(self._identitas)

    def tambah_identitas(self, label: str) -> Identitas:
        """Membuat identitas baru. Label harus belum dipakai."""
        if label in self._identitas:
            raise GalatEnrollment(f"Label {label} sudah terdaftar pada proses ini.")
        identitas = Identitas(label=label)
        self._identitas[label] = identitas
        return identitas

    def tambah_sampel(self, label: str, vektor: np.ndarray) -> None:
        if label not in self._identitas:
            raise GalatEnrollment(f"Label {label} belum terdaftar.")
        self._identitas[label].tambah(vektor)

    def label_berikutnya(self) -> str:
        """Label anonim berurutan: PERSON-001, PERSON-002, dan seterusnya."""
        nomor = len(self._identitas) + 1
        while f"PERSON-{nomor:03d}" in self._identitas:
            nomor += 1
        return f"PERSON-{nomor:03d}"

    def semua(self) -> list[Identitas]:
        return [self._identitas[label] for label in sorted(self._identitas)]

    def kosongkan(self) -> None:
        self._identitas.clear()

"""Protokol pengujian lapangan: persetujuan, sesi, dan rekap.

Modul ini **bukan** fitur produk. Ia ada supaya sesi pengukuran dapat
dijalankan dengan urutan yang sama setiap kali, dan supaya persetujuan peserta
tercatat sebagai fakta sebelum satu frame pun diproses.

Aturan yang ditegakkan di sini, bukan hanya didokumentasikan:

  - Sesi tidak dapat dimulai tanpa `catat_persetujuan()` untuk setiap peserta.
  - Label peserta selalu anonim (`PERSON-00N`); tidak ada nama, NIM, atau
    inisial yang pernah masuk objek sesi.
  - Tidak ada metode yang menulis berkas. Rekap hanya dikembalikan sebagai
    objek Python agar pemanggil memutuskan sendiri apa yang dicetak.
  - `akhiri_sesi()` mengosongkan galeri dan seluruh daftar kueri, termasuk
    vektor embedding-nya.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone

import numpy as np

from .errors import GalatEnrollment
from .evaluation import Sampel

#: Teks penjelasan minimum yang harus dibacakan sebelum meminta persetujuan.
#: Isinya sengaja menyebut risiko dan alternatif, bukan hanya manfaat.
PENJELASAN_PERSETUJUAN = """\
Pengujian ini mengukur seberapa baik pengenalan wajah bekerja pada kondisi
berbeda. Yang perlu diketahui sebelum Anda setuju:

1. Yang diambil hanya potongan wajah dari kamera laptop ini, dan hanya selama
   jendela pengujian terbuka.
2. Tidak ada foto, video, atau template wajah yang ditulis ke disk. Data hanya
   berada di memori dan hilang saat program ditutup.
3. Tidak ada data yang dikirim ke internet atau ke layanan mana pun.
4. Dalam laporan, Anda hanya muncul sebagai label anonim (misalnya PERSON-001).
   Nama, NIM, dan foto Anda tidak dicatat.
5. Anda boleh berhenti kapan saja tanpa menjelaskan alasan, dan boleh meminta
   hasil Anda dihapus dari sesi yang sedang berjalan.
6. Sistem ini TIDAK dapat membedakan wajah asli dari foto atau video wajah
   (tidak ada liveness). Jangan menilai keamanan dari pengujian ini.
"""


@dataclass
class Peserta:
    """Satu peserta sukarela, dengan label anonim dan status persetujuan."""

    label: str
    persetujuan_dicatat: bool = False
    waktu_persetujuan: str | None = None
    kacamata: bool = False
    #: True bila peserta secara eksplisit menolak. Menolak **berbeda** dari
    #: "belum ditanya": penolakan tidak boleh memblokir seluruh sesi, sedangkan
    #: peserta yang belum ditanya memang harus memblokir pengambilan sampel.
    menolak: bool = False

    def catat(self) -> None:
        self.persetujuan_dicatat = True
        self.waktu_persetujuan = datetime.now(timezone.utc).isoformat(timespec="seconds")

    def tolak(self) -> None:
        self.persetujuan_dicatat = False
        self.waktu_persetujuan = None
        self.menolak = True


@dataclass
class SesiKalibrasi:
    """Satu sesi pengukuran.

    Sesi memegang label peserta dan daftar kueri; ia **tidak** memegang gambar.
    Vektor embedding ikut tersimpan di `kueri` selama sesi berjalan dan
    dibersihkan oleh `akhiri_sesi()`.
    """

    nama_sesi: str
    peserta: dict[str, Peserta] = field(default_factory=dict)
    kueri: list[Sampel] = field(default_factory=list)
    catatan: list[str] = field(default_factory=list)
    selesai: bool = False

    # ------------------------------------------------------------ peserta

    def daftarkan_peserta(self, kacamata: bool = False) -> Peserta:
        """Membuat peserta baru dengan label anonim berurutan.

        Persetujuan **belum** dicatat pada titik ini. Peserta yang belum
        menyetujui tetap boleh ada dalam daftar (supaya urutan label stabil),
        tetapi `tambah_kueri()` menolak berjalan sampai statusnya jelas:
        menyetujui lewat `catat_persetujuan()` atau menolak lewat
        `tolak_persetujuan()`.
        """
        nomor = len(self.peserta) + 1
        while f"PERSON-{nomor:03d}" in self.peserta:
            nomor += 1
        label = f"PERSON-{nomor:03d}"
        p = Peserta(label=label, kacamata=kacamata)
        self.peserta[label] = p
        return p

    def catat_persetujuan(self, label: str) -> None:
        if label not in self.peserta:
            raise GalatEnrollment(f"Peserta {label} belum terdaftar pada sesi ini.")
        self.peserta[label].catat()

    def tolak_persetujuan(self, label: str) -> None:
        """Mencatat penolakan eksplisit; label tetap dipakai agar urutan stabil.

        Ini bukan pencabutan: tidak ada data yang pernah diambil dari peserta
        yang menolak. Bedanya penting — penolakan tidak boleh menghapus data
        peserta lain atau memblokir seluruh sesi.
        """
        if label not in self.peserta:
            raise GalatEnrollment(f"Peserta {label} belum terdaftar pada sesi ini.")
        self.peserta[label].tolak()
        self.catatan.append(f"{label} menolak ikut serta; tidak ada sampel diambil.")

    def tarik_persetujuan(self, label: str) -> None:
        """Mencabut persetujuan: peserta dan seluruh sampelnya dibuang.

        Pencabutan **seketika**, tanpa masa tunggu, sesuai
        `docs/SECURITY_PRIVACY.md` §5d.4 butir 5.
        """
        if label not in self.peserta:
            return
        del self.peserta[label]
        self.kueri = [s for s in self.kueri if s.label != label]
        self.catatan.append(f"{label} mencabut persetujuan; data di memori dibuang.")

    def pastikan_semua_setuju(self) -> None:
        """Menolak pengambilan sampel selama ada peserta yang belum ditanya."""
        belum = [
            p.label
            for p in self.peserta.values()
            if not p.persetujuan_dicatat and not p.menolak
        ]
        if belum:
            raise GalatEnrollment(
                "Status persetujuan belum dicatat untuk: " + ", ".join(belum) + ". "
                "Catat persetujuan atau penolakan sebelum mengambil sampel."
            )

    # ------------------------------------------------------------- kueri

    def tambah_kueri(
        self,
        label_benar: str | None,
        vektor: np.ndarray,
        kondisi: str,
        durasi_deteksi_ms: float | None = None,
        durasi_embedding_ms: float | None = None,
        ketajaman: float | None = None,
        kecerahan: float | None = None,
    ) -> Sampel:
        """Menyimpan satu kueri di memori.

        `label_benar=None` menandai wajah tak terdaftar (orang lain atau wajah
        yang sengaja tidak dienroll). Label yang tidak ada di daftar peserta
        ditolak, supaya tidak ada entri yang lolos tanpa persetujuan.

        `ketajaman` dan `kecerahan` adalah skalar kualitas frame dari
        `quality.nilai_frame()`; keduanya bukan biometrik dan hanya dipakai
        untuk melihat apakah skor turun pada frame buram atau gelap.
        """
        self.pastikan_semua_setuju()
        if label_benar is not None:
            peserta = self.peserta.get(label_benar)
            if peserta is None:
                raise GalatEnrollment(f"Label {label_benar} bukan peserta sesi ini.")
            if not peserta.persetujuan_dicatat:
                raise GalatEnrollment(
                    f"Persetujuan {label_benar} tidak aktif; sampel tidak disimpan."
                )

        sampel = Sampel(
            label=label_benar,
            vektor=np.asarray(vektor, dtype=np.float32),
            kondisi=kondisi,
            durasi_deteksi_ms=durasi_deteksi_ms,
            durasi_embedding_ms=durasi_embedding_ms,
            ketajaman=ketajaman,
            kecerahan=kecerahan,
        )
        self.kueri.append(sampel)
        return sampel

    def jumlah_per_kondisi(self) -> dict[str, int]:
        hasil: dict[str, int] = {}
        for s in self.kueri:
            hasil[s.kondisi] = hasil.get(s.kondisi, 0) + 1
        return dict(sorted(hasil.items()))

    def jumlah_per_peserta(self) -> dict[str, int]:
        hasil: dict[str, int] = {}
        for s in self.kueri:
            kunci = s.label or "unknown"
            hasil[kunci] = hasil.get(kunci, 0) + 1
        return dict(sorted(hasil.items()))

    # -------------------------------------------------------------- akhir

    def akhiri_sesi(self) -> dict[str, object]:
        """Mengosongkan seluruh data biometrik dan mengembalikan rekap ringkas.

        Rekap berisi **hanya** hitungan dan label anonim. Setelah fungsi ini
        berjalan, `kueri` kosong dan vektor tidak lagi dapat dijangkau dari sesi.
        """
        rekap: dict[str, object] = {
            "nama_sesi": self.nama_sesi,
            "jumlah_peserta": len(self.peserta),
            "persetujuan_tercatat": sum(1 for p in self.peserta.values() if p.persetujuan_dicatat),
            "jumlah_kueri": len(self.kueri),
            "per_kondisi": self.jumlah_per_kondisi(),
            "per_peserta": self.jumlah_per_peserta(),
            "catatan": list(self.catatan),
        }
        self.kueri.clear()
        self.catatan.clear()
        self.selesai = True
        return rekap

"""Galat yang dapat dibedakan oleh lapisan pemanggil.

Setiap galat membawa kode pendek yang stabil supaya pesan ke pengguna dan
pengujian tidak bergantung pada teks bahasa Indonesia yang bisa berubah.
"""


class GalatMesinWajah(Exception):
    """Kelas dasar seluruh galat prototype."""

    kode = "MESIN_WAJAH_GALAT"


class GalatKamera(GalatMesinWajah):
    """Kamera tidak dapat dibuka atau frame tidak dapat dibaca."""

    kode = "KAMERA_GALAT"


class GalatModel(GalatMesinWajah):
    """Berkas model hilang, checksum tidak cocok, atau gagal dimuat."""

    kode = "MODEL_GALAT"


class GalatEnrollment(GalatMesinWajah):
    """Permintaan enrollment tidak sah (mis. sudah ada identitas aktif)."""

    kode = "ENROLLMENT_GALAT"

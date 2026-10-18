"""Mengunduh bobot YuNet dan SFace dari OpenCV Zoo dengan verifikasi SHA-256.

Model **tidak** di-commit: `.gitignore` sudah mengabaikan `*.onnx` dan `models/`.
Yang masuk repositori hanya skrip ini, angka checksum, dan lisensi masing-masing
model. Alasan pemisahan itu ada dua:

  1. Berkas bobot berukuran puluhan megabukti dan tidak boleh berubah tanpa
     jejak yang terlihat di riwayat Git.
  2. Checksum yang di-commit membuat perubahan di hulu **ditolak**, bukan
     diterima diam-diam. Bila angka berubah, itu keputusan sadar yang harus
     melewati tinjauan, bukan efek samping `curl`.

Jalankan dari `services/face-engine/`:

    python scripts/download_models.py
"""

from __future__ import annotations

import hashlib
import sys
import urllib.request
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from face_engine.config import (  # noqa: E402
    DEFAULT_MODEL_DIR,
    SFACE_FILENAME,
    SFACE_SHA256,
    YUNET_FILENAME,
    YUNET_SHA256,
)

#: Sumber resmi: OpenCV Zoo. Jangan ganti ke cermin pihak ketiga.
BASE_URL = "https://github.com/opencv/opencv_zoo/raw/main/models"

MODEL = [
    {
        "nama": YUNET_FILENAME,
        "jalur": f"{BASE_URL}/face_detection_yunet/{YUNET_FILENAME}",
        "sha256": YUNET_SHA256,
        "lisensi": "MIT (OpenCV Zoo, direktori face_detection_yunet)",
        "ukuran": "232 KB",
    },
    {
        "nama": SFACE_FILENAME,
        "jalur": f"{BASE_URL}/face_recognition_sface/{SFACE_FILENAME}",
        "sha256": SFACE_SHA256,
        "lisensi": "Apache-2.0 (OpenCV Zoo, direktori face_recognition_sface)",
        "ukuran": "37 MB",
    },
]


def sha256_berkas(jalur: Path) -> str:
    """Menghitung SHA-256 dengan pembacaan bertahap, bukan seluruh berkas di RAM."""
    h = hashlib.sha256()
    with jalur.open("rb") as berkas:
        for potong in iter(lambda: berkas.read(1024 * 1024), b""):
            h.update(potong)
    return h.hexdigest()


def unduh(salin_ulang: bool = False) -> int:
    """Mengunduh model yang belum ada atau checksum-nya tidak cocok."""
    DEFAULT_MODEL_DIR.mkdir(parents=True, exist_ok=True)
    gagal = 0

    for model in MODEL:
        tujuan = DEFAULT_MODEL_DIR / model["nama"]
        diharapkan = model["sha256"]

        if tujuan.is_file() and not salin_ulang:
            aktual = sha256_berkas(tujuan)
            if aktual == diharapkan:
                print(f"[ada]     {model['nama']} — checksum cocok")
                continue
            print(f"[ulang]   {model['nama']} — checksum tidak cocok, mengunduh ulang")

        print(f"[unduh]   {model['nama']} ({model['ukuran']}) ...")
        sementara = tujuan.with_suffix(tujuan.suffix + ".part")
        try:
            urllib.request.urlretrieve(model["jalur"], sementara)
        except Exception as galat:  # noqa: BLE001 - pesan diteruskan apa adanya
            print(f"[GAGAL]   {model['nama']}: {galat}")
            sementara.unlink(missing_ok=True)
            gagal += 1
            continue

        aktual = sha256_berkas(sementara)
        if aktual != diharapkan:
            print(
                f"[GAGAL]   {model['nama']}: checksum berbeda.\n"
                f"          diharapkan {diharapkan}\n"
                f"          diperoleh {aktual}\n"
                "          Berkas dihapus. Periksa apakah berkas di hulu berubah; "
                "jangan memakai bobot yang tidak terverifikasi."
            )
            sementara.unlink(missing_ok=True)
            gagal += 1
            continue

        sementara.replace(tujuan)
        print(f"[ok]      {model['nama']} — SHA-256 cocok")

    print("\nLisensi model:")
    for model in MODEL:
        print(f"  {model['nama']}: {model['lisensi']}")
    print(
        "\nCatatan: YuNet dan SFace tidak menyediakan liveness detection. "
        "Prototype ini tidak tahan terhadap spoofing foto/video (D-029)."
    )
    return 1 if gagal else 0


if __name__ == "__main__":
    raise SystemExit(unduh(salin_ulang="--force" in sys.argv))

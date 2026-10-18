"""Titik masuk baris perintah prototype FL-CV01.

Contoh:

    python -m face_engine --mode detect
    python -m face_engine --mode enroll
    python -m face_engine --mode recognize --threshold 0.363 --camera 0

Program ini tidak menulis berkas apa pun. Bila kamera atau model tidak tersedia,
ia berhenti dengan pesan yang menyebutkan tindakan perbaikan, bukan traceback
mentah.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import __version__
from .app import Aplikasi
from .camera import Kamera
from .config import (
    DEFAULT_MODEL_DIR,
    MATCH_MARGIN,
    MATCH_THRESHOLD,
    REQUIRED_SAMPLES,
    Konfigurasi,
)
from .detector import Detektor
from .embedder import Embedder
from .errors import GalatKamera, GalatMesinWajah, GalatModel


def bangun_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="face_engine",
        description="Prototype deteksi dan pengenalan wajah lokal FaceLink AI (FL-CV01).",
    )
    parser.add_argument("--mode", choices=["detect", "enroll", "recognize"], default="detect")
    parser.add_argument("--camera", type=int, default=0, help="indeks kamera (bawaan 0)")
    parser.add_argument("--width", type=int, default=1280)
    parser.add_argument("--height", type=int, default=720)
    parser.add_argument(
        "--model-dir",
        type=Path,
        default=DEFAULT_MODEL_DIR,
        help="direktori berkas .onnx (bawaan services/face-engine/models)",
    )
    parser.add_argument(
        "--threshold",
        type=float,
        default=MATCH_THRESHOLD,
        help="ambang cosine similarity untuk pengujian (BUKAN angka produksi)",
    )
    parser.add_argument("--margin", type=float, default=MATCH_MARGIN)
    parser.add_argument("--samples", type=int, default=REQUIRED_SAMPLES)
    parser.add_argument(
        "--cek",
        action="store_true",
        help="periksa kamera dan model tanpa membuka jendela, lalu keluar",
    )
    parser.add_argument("--version", action="version", version=f"face_engine {__version__}")
    return parser


def jalankan_cek(konfigurasi: Konfigurasi) -> int:
    """Pemeriksaan lingkungan tanpa membuka jendela.

    Dipakai untuk memisahkan tiga kegagalan yang sering tercampur saat debugging:
    kamera tidak ada, model tidak ada, atau model ada tetapi tidak dapat dimuat.
    """
    kode = 0

    print(f"Model dir : {konfigurasi.model_dir}")
    for nama, jalur in (("YuNet", konfigurasi.yunet_path), ("SFace", konfigurasi.sface_path)):
        status = "ada" if jalur.is_file() else "TIDAK ADA"
        ukuran = f"{jalur.stat().st_size / 1_048_576:.1f} MB" if jalur.is_file() else "-"
        print(f"  {nama:<6}: {status} ({ukuran})")

    try:
        Detektor(konfigurasi.yunet_path)
        print("YuNet    : berhasil dimuat")
    except GalatModel as galat:
        print(f"YuNet    : GAGAL — {galat}")
        kode = 1

    try:
        Embedder(konfigurasi.sface_path)
        print("SFace    : berhasil dimuat")
    except GalatModel as galat:
        print(f"SFace    : GAGAL — {galat}")
        kode = 1

    try:
        with Kamera(konfigurasi.camera_index, konfigurasi.frame_width, konfigurasi.frame_height) as kamera:
            frame = kamera.baca()
            tinggi, lebar = frame.shape[:2]
            print(f"Kamera   : indeks {konfigurasi.camera_index}, frame {lebar}x{tinggi}")
    except GalatKamera as galat:
        print(f"Kamera   : GAGAL — {galat}")
        kode = 1

    print("Hasil    :", "siap" if kode == 0 else "ada komponen yang gagal")
    return kode


def main(argv: list[str] | None = None) -> int:
    argumen = bangun_parser().parse_args(argv)
    konfigurasi = Konfigurasi(
        model_dir=argumen.model_dir,
        camera_index=argumen.camera,
        frame_width=argumen.width,
        frame_height=argumen.height,
        match_threshold=argumen.threshold,
        match_margin=argumen.margin,
        required_samples=argumen.samples,
    )

    if argumen.cek:
        return jalankan_cek(konfigurasi)

    print(
        "FL-CV01 — prototype lokal. Enrollment hanya di RAM dan hilang saat keluar.\n"
        "Tekan H untuk daftar tombol, Q untuk keluar."
    )
    try:
        aplikasi = Aplikasi(konfigurasi, mode_awal=argumen.mode)
        if argumen.mode == "enroll":
            aplikasi.mulai_enrollment()
        elif argumen.mode == "recognize":
            print(
                "Mode recognize dimulai dengan galeri kosong; tekan E untuk mendaftarkan "
                "wajah terlebih dahulu."
            )
        return aplikasi.jalankan()
    except GalatMesinWajah as galat:
        print(f"[{galat.kode}] {galat}", file=sys.stderr)
        return 2
    except KeyboardInterrupt:
        print("\nDihentikan pengguna; data enrollment di memori dibuang.")
        return 130


if __name__ == "__main__":
    raise SystemExit(main())

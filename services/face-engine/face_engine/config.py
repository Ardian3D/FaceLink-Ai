"""Konfigurasi prototype.

Nilai bawaan dipilih untuk demo laptop, bukan untuk produksi. Yang penting di
sini: **tidak ada** kunci, token, atau alamat jaringan yang ditanam. Model dan
ambang dapat diubah lewat argumen baris perintah.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

#: Direktori berkas bobot model. Diabaikan Git (`*.onnx` pada `.gitignore`).
DEFAULT_MODEL_DIR = Path(__file__).resolve().parent.parent / "models"

#: Nama berkas bobot yang diharapkan oleh `scripts/download_models.py`.
YUNET_FILENAME = "face_detection_yunet_2023mar.onnx"
SFACE_FILENAME = "face_recognition_sface_2021dec.onnx"

#: SHA-256 berkas bobot yang benar-benar diunduh dan diverifikasi pada
#: 19 Oktober 2026. Bila berkas di hulu berubah, unduhan **ditolak** sampai
#: angka ini diperbarui secara sadar — bukan diterima diam-diam.
YUNET_SHA256 = "8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4"
SFACE_SHA256 = "0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79"

#: Ukuran masukan YuNet bawaan (dipakai saat ukuran frame belum diketahui).
#: Bentuk sebenarnya selalu mengikuti rasio frame; lihat `detector.ukuran_masukan`.
DETECTOR_INPUT = (640, 640)

#: Batas sisi panjang masukan detektor. 960 px menjaga biaya inferensi di bawah
#: ~30 ms pada laptop pengembangan sambil mempertahankan detail wajah.
DETECTOR_MAX_SIDE = 960

#: Ambang detektor. Nilai ini **bukan** ambang pengenalan dan belum dikalibrasi
#: pada peserta uji; ia hanya menekan deteksi palsu saat preview.
DETECTOR_SCORE_THRESHOLD = 0.75
DETECTOR_NMS_THRESHOLD = 0.3
DETECTOR_TOP_K = 5000

#: Ambang uji pengenalan (cosine similarity). **Bukan angka produksi.**
#: Dokumentasi OpenCV Zoo memakai 0.363 untuk SFace dengan normalisasi L2 pada
#: metrik cosine; angka itu dipakai sebagai titik awal pengujian, bukan hasil
#: kalibrasi kita, dan wajib diukur ulang pada peserta uji (Q-07).
MATCH_THRESHOLD = 0.363

#: Margin minimum antara kandidat teratas dan kedua. Bila lebih kecil, hasil
#: dianggap ambigu dan **tidak** boleh dianggap cocok (lihat `ambiguous_match`
#: pada `docs/API_CONTRACT.md` §7).
MATCH_MARGIN = 0.05

#: Kualitas sampel enrollment.
#:
#: Ambang ketajaman diukur pada mesin pengembangan (19 Oktober 2026, Integrated
#: Camera 1280x720): frame **tajam** menghasilkan varians Laplacian 35–89
#: (median 59,7), sedangkan frame yang sengaja diburamkan (Gaussian 11x11)
#: menghasilkan 2,6–3,9. Nilai 20 berada jauh di atas lantai buram dan di bawah
#: frame tajam terendah yang terukur. Ini **bukan** angka universal: kamera lain
#: dengan sensor atau pencahayaan berbeda akan menghasilkan rentang berbeda, dan
#: angka ini wajib diukur ulang sebelum dipakai di luar demo.
MIN_FACE_SIZE_PX = 80
MIN_SHARPNESS = 20.0
MIN_MEAN_BRIGHTNESS = 40.0
MAX_MEAN_BRIGHTNESS = 220.0

#: Jumlah sampel yang diminta per identitas uji.
REQUIRED_SAMPLES = 5


@dataclass(frozen=True)
class Konfigurasi:
    """Kumpulan pengaturan satu proses prototype."""

    model_dir: Path = DEFAULT_MODEL_DIR
    camera_index: int = 0
    frame_width: int = 1280
    frame_height: int = 720
    match_threshold: float = MATCH_THRESHOLD
    match_margin: float = MATCH_MARGIN
    required_samples: int = REQUIRED_SAMPLES
    detector_input: tuple[int, int] = field(default=DETECTOR_INPUT)

    @property
    def yunet_path(self) -> Path:
        return self.model_dir / YUNET_FILENAME

    @property
    def sface_path(self) -> Path:
        return self.model_dir / SFACE_FILENAME

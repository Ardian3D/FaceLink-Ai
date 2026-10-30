"""Bounded, synthetic-only YuNet/SFace extraction; no recognition decision."""

from __future__ import annotations

import hashlib
import hmac
from dataclasses import dataclass, field
from pathlib import Path

import cv2
import numpy as np

from .config import (
    DETECTOR_SCORE_THRESHOLD,
    DETECTOR_TOP_K,
    SFACE_FILENAME,
    SFACE_SHA256,
    YUNET_FILENAME,
    YUNET_SHA256,
)
from .detector import Deteksi, Detektor
from .embedder import EMBEDDING_DIM, Embedder
from .errors import GalatModel
from .quality import nilai_frame

MAX_JPEG_BYTES = 1_048_576
MAX_PIXELS = 2_073_600
MAX_SIDE = 4096
MAX_MODEL_BYTES = 64 * 1024 * 1024
MAX_FACE_COUNT = 1024
PIPELINE_REVISION = "enr04-synthetic-native-v2"
FAILURE_STATUSES = frozenset({
    "NO_FACE", "MULTIPLE_FACES", "LOW_QUALITY", "FACE_TOO_SMALL",
    "INVALID_IMAGE", "MODEL_UNAVAILABLE", "INFERENCE_FAILED",
})


def manifest() -> dict[str, str | bool]:
    """Return a fresh model/pipeline fingerprint, never mutable shared state."""
    return {
        "synthetic_only": True,
        "model_id": "opencv-sface",
        "model_version": "2021dec",
        "model_checksum": SFACE_SHA256,
        "detector_id": "opencv-yunet",
        "detector_version": "2023mar",
        "detector_checksum": YUNET_SHA256,
        "pipeline_revision": PIPELINE_REVISION,
    }


class InvalidImage(ValueError):
    """The input is not a bounded, metadata-free supported JPEG."""


class ModelUnavailable(GalatModel):
    """Pinned local model files could not be verified or loaded."""

    def __init__(self) -> None:
        super().__init__("MODEL_UNAVAILABLE")


@dataclass(frozen=True)
class ProcessingResult:
    status: str
    face_count: int = 0
    embedding: np.ndarray = field(default_factory=lambda: np.empty(0, dtype=np.float32), repr=False)


def jpeg_dimensions(jpeg: bytes) -> tuple[int, int]:
    """Validate JPEG structure and dimensions before handing bytes to a decoder.

    Only 8-bit baseline/progressive grayscale or three-component JPEG is accepted.
    APP1 is rejected altogether: no EXIF orientation is silently applied or retained.
    Entropy bytes are scanned as well, so concatenated/trailing images are rejected.
    """
    if not isinstance(jpeg, bytes) or not 4 <= len(jpeg) <= MAX_JPEG_BYTES:
        raise InvalidImage("INVALID_IMAGE")
    if jpeg[:2] != b"\xff\xd8":
        raise InvalidImage("INVALID_IMAGE")
    position = 2
    dimensions = None
    scanned = False
    in_scan = False
    while position < len(jpeg):
        if in_scan:
            position = jpeg.find(b"\xff", position)
            if position < 0:
                break
        elif jpeg[position] != 0xFF:
            raise InvalidImage("INVALID_IMAGE")
        while position < len(jpeg) and jpeg[position] == 0xFF:
            position += 1
        if position >= len(jpeg):
            break
        marker = jpeg[position]
        position += 1
        if in_scan and (marker == 0 or 0xD0 <= marker <= 0xD7):
            continue
        in_scan = False
        if marker == 0xD9:
            if dimensions is not None and scanned and position == len(jpeg):
                return dimensions
            raise InvalidImage("INVALID_IMAGE")
        if marker in (0, 0xD8, 0x01) or 0xD0 <= marker <= 0xD7:
            raise InvalidImage("INVALID_IMAGE")
        if position + 2 > len(jpeg):
            break
        length = int.from_bytes(jpeg[position:position + 2], "big")
        if length < 2 or position + length > len(jpeg):
            break
        start = position + 2
        end = position + length
        if marker == 0xE1:
            raise InvalidImage("INVALID_IMAGE")
        if 0xC0 <= marker <= 0xCF and marker not in (0xC4, 0xC8, 0xCC):
            if marker not in (0xC0, 0xC2) or dimensions is not None or length < 8:
                raise InvalidImage("INVALID_IMAGE")
            precision = jpeg[start]
            height = int.from_bytes(jpeg[start + 1:start + 3], "big")
            width = int.from_bytes(jpeg[start + 3:start + 5], "big")
            components = jpeg[start + 5]
            if (precision != 8 or components not in (1, 3)
                    or length != 8 + 3 * components
                    or not 1 <= width <= MAX_SIDE or not 1 <= height <= MAX_SIDE
                    or width * height > MAX_PIXELS):
                raise InvalidImage("INVALID_IMAGE")
            dimensions = (width, height)
        if marker == 0xDA:
            if dimensions is None or length < 6:
                raise InvalidImage("INVALID_IMAGE")
            scanned = True
            in_scan = True
        position = end
    raise InvalidImage("INVALID_IMAGE")


def decode_jpeg(jpeg: bytes) -> np.ndarray:
    width, height = jpeg_dimensions(jpeg)
    try:
        frame = cv2.imdecode(np.frombuffer(jpeg, dtype=np.uint8),
                             cv2.IMREAD_COLOR | cv2.IMREAD_IGNORE_ORIENTATION)
    except cv2.error:
        raise InvalidImage("INVALID_IMAGE") from None
    if (frame is None or frame.dtype != np.uint8
            or frame.shape != (height, width, 3)):
        raise InvalidImage("INVALID_IMAGE")
    return frame


def _verify_model(path: Path, expected: str) -> None:
    digest = hashlib.sha256()
    total = 0
    with path.open("rb") as stream:
        while chunk := stream.read(128 * 1024):
            total += len(chunk)
            if total > MAX_MODEL_BYTES:
                raise ModelUnavailable()
            digest.update(chunk)
    if not hmac.compare_digest(digest.hexdigest(), expected):
        raise ModelUnavailable()


def _valid_detection(face: Deteksi, width: int, height: int) -> bool:
    row = face.baris
    points = face.landmarks
    if (not isinstance(row, np.ndarray) or row.shape != (15,)
            or row.dtype != np.float32 or not np.isfinite(row).all()
            or not isinstance(points, np.ndarray) or points.shape != (5, 2)
            or points.dtype != np.float32 or not np.isfinite(points).all()
            or not np.array_equal(row[4:14], points.reshape(-1))
            or not np.isfinite(face.skor)
            or not DETECTOR_SCORE_THRESHOLD <= face.skor <= 1.0
            or abs(float(row[14]) - face.skor) > 1e-6):
        return False
    x, y, w, h = (float(value) for value in row[:4])
    if (w <= 0 or h <= 0 or w > 2 * width or h > 2 * height
            or x >= width or y >= height or x + w <= 0 or y + h <= 0
            or face.lebar <= 0 or face.tinggi <= 0):
        return False
    if (np.any(points[:, 0] < 0) or np.any(points[:, 0] >= width)
            or np.any(points[:, 1] < 0) or np.any(points[:, 1] >= height)
            or np.any(points[:, 0] < x) or np.any(points[:, 0] > x + w)
            or np.any(points[:, 1] < y) or np.any(points[:, 1] > y + h)):
        return False
    # Five distinct points and two noncollinear eye/mouth axes are required for alignment.
    distances = np.sum((points[:, None, :] - points[None, :, :]) ** 2, axis=2)
    distances[np.diag_indices(5)] = np.inf
    eye = points[1] - points[0]
    nose = points[2] - points[0]
    mouth = points[4] - points[3]
    nose_mouth = points[2] - points[3]
    eye_area = abs(float(eye[0] * nose[1] - eye[1] * nose[0]))
    mouth_area = abs(float(mouth[0] * nose_mouth[1] - mouth[1] * nose_mouth[0]))
    return bool(np.min(distances) >= 1.0 and eye_area >= 1.0 and mouth_area >= 1.0)


def _validated_vector(vector: np.ndarray) -> np.ndarray:
    if (not isinstance(vector, np.ndarray) or vector.shape != (EMBEDDING_DIM,)
            or vector.dtype != np.float32 or not np.isfinite(vector).all()):
        raise ValueError("INVALID_EMBEDDING")
    norm = float(np.linalg.norm(vector.astype(np.float64)))
    if not np.isfinite(norm) or abs(norm - 1.0) > 1e-5:
        raise ValueError("INVALID_EMBEDDING")
    return vector


def similarity(a: np.ndarray, b: np.ndarray,
               a_manifest: dict[str, str | bool],
               b_manifest: dict[str, str | bool]) -> float:
    """Cosine only, no acceptance threshold; reject mismatched fingerprints."""
    expected = manifest()
    if (a_manifest != expected or b_manifest != expected
            or type(a_manifest.get("synthetic_only")) is not bool
            or type(b_manifest.get("synthetic_only")) is not bool):
        raise ValueError("INCOMPATIBLE_MANIFEST")
    score = Embedder.kemiripan(_validated_vector(a), _validated_vector(b))
    if not np.isfinite(score) or not -1.00001 <= score <= 1.00001:
        raise ValueError("INVALID_SIMILARITY")
    return min(1.0, max(-1.0, score))


class Pipeline:
    """Real local models only. Construct once for one disposable IPC request."""

    def __init__(self, model_dir: Path) -> None:
        try:
            yunet = Path(model_dir) / YUNET_FILENAME
            sface = Path(model_dir) / SFACE_FILENAME
            # Both checks complete before either native runtime is constructed.
            _verify_model(yunet, YUNET_SHA256)
            _verify_model(sface, SFACE_SHA256)
            cv2.setNumThreads(1)
            cv2.ocl.setUseOpenCL(False)
            self._detector = Detektor(yunet)
            self._embedder = Embedder(sface)
        except Exception:
            raise ModelUnavailable() from None

    def process(self, jpeg: bytes) -> ProcessingResult:
        try:
            frame = decode_jpeg(jpeg)
        except InvalidImage:
            return ProcessingResult("INVALID_IMAGE")
        count = 0
        try:
            faces = self._detector.deteksi(frame)
            if not isinstance(faces, list) or len(faces) > min(DETECTOR_TOP_K, MAX_FACE_COUNT):
                return ProcessingResult("INFERENCE_FAILED")
            count = len(faces)
            if any(not _valid_detection(face, frame.shape[1], frame.shape[0]) for face in faces):
                return ProcessingResult("INFERENCE_FAILED", count)
            quality = nilai_frame(frame, faces)
            if not quality.diterima:
                status = quality.kode if quality.kode in FAILURE_STATUSES else "INFERENCE_FAILED"
                return ProcessingResult(status, count)
            if count != 1 or not all(np.isfinite(value) for value in quality.rincian.values()):
                return ProcessingResult("INFERENCE_FAILED", count)
            vector = _validated_vector(self._embedder.embedding(frame, faces[0]))
            return ProcessingResult("OK", 1, vector)
        except Exception:
            return ProcessingResult("INFERENCE_FAILED", count)

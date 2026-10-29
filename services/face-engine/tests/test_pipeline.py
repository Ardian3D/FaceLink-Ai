"""Synthetic-only boundary tests; fabricated detections are isolated domain tests."""

from dataclasses import replace
from types import SimpleNamespace

import cv2
import numpy as np
import pytest

from face_engine import pipeline
from face_engine.config import SFACE_FILENAME, YUNET_FILENAME
from face_engine.detector import Deteksi
from face_engine.embedder import Embedder
from face_engine.errors import GalatModel


def synthetic_jpeg(width=160, height=160, uniform=False):
    frame = np.zeros((height, width, 3), dtype=np.uint8)
    if not uniform:
        frame[:] = np.random.default_rng(817).integers(0, 256, frame.shape, dtype=np.uint8)
    ok, encoded = cv2.imencode(".jpg", frame)
    assert ok
    return encoded.tobytes()


def synthetic_detection():
    points = np.array([[40, 50], [100, 50], [70, 75], [45, 110], [95, 110]], dtype=np.float32)
    row = np.array([16, 16, 120, 120, *points.reshape(-1), 0.99], dtype=np.float32)
    return Deteksi(16, 16, 120, 120, 0.99, points, row)


def unit_vector():
    vector = np.zeros(128, dtype=np.float32)
    vector[0] = 1
    return vector


def domain_pipeline(faces, vector=None):
    """No production factory or runtime fallback: replace dependencies only in this test."""
    instance = pipeline.Pipeline.__new__(pipeline.Pipeline)
    instance._detector = SimpleNamespace(deteksi=lambda frame: faces)
    instance._embedder = SimpleNamespace(embedding=lambda frame, face: unit_vector() if vector is None else vector)
    return instance


def test_bounded_jpeg_decodes_synthetic_pixels():
    jpeg = synthetic_jpeg()
    assert pipeline.jpeg_dimensions(jpeg) == (160, 160)
    assert pipeline.decode_jpeg(jpeg).shape == (160, 160, 3)


@pytest.mark.parametrize("data", [b"", b"not jpeg", b"\xff\xd8\xff\xd9", b"\xff\xd8\xff\xe0\x00\x01"])
def test_invalid_jpeg_never_reaches_decoder(monkeypatch, data):
    monkeypatch.setattr(cv2, "imdecode", lambda *args: pytest.fail("decoder called"))
    with pytest.raises(pipeline.InvalidImage, match="^INVALID_IMAGE$"):
        pipeline.decode_jpeg(data)


def test_oversize_bytes_rejected_before_decoder(monkeypatch):
    monkeypatch.setattr(cv2, "imdecode", lambda *args: pytest.fail("decoder called"))
    with pytest.raises(pipeline.InvalidImage):
        pipeline.decode_jpeg(b"\xff\xd8" + bytes(pipeline.MAX_JPEG_BYTES))


@pytest.mark.parametrize("width,height", [(4097, 1), (1, 4097), (1921, 1080), (0, 100), (100, 0)])
def test_forged_sof_bounds_rejected_before_decoder(monkeypatch, width, height):
    jpeg = bytearray(synthetic_jpeg())
    start = jpeg.index(b"\xff\xc0")
    jpeg[start + 5:start + 7] = height.to_bytes(2, "big")
    jpeg[start + 7:start + 9] = width.to_bytes(2, "big")
    monkeypatch.setattr(cv2, "imdecode", lambda *args: pytest.fail("decoder called"))
    with pytest.raises(pipeline.InvalidImage):
        pipeline.decode_jpeg(bytes(jpeg))


def test_progressive_jpeg_supported():
    frame = np.full((80, 120, 3), 100, dtype=np.uint8)
    ok, encoded = cv2.imencode(".jpg", frame, [cv2.IMWRITE_JPEG_PROGRESSIVE, 1])
    assert ok
    assert pipeline.decode_jpeg(encoded.tobytes()).shape == frame.shape


@pytest.mark.parametrize("suffix", [b"x", b"\x00", b"\xff\xd8\xff\xd9"])
def test_trailing_or_concatenated_jpeg_rejected(suffix):
    with pytest.raises(pipeline.InvalidImage):
        pipeline.jpeg_dimensions(synthetic_jpeg() + suffix)


def test_exif_rejected_before_decoder(monkeypatch):
    jpeg = synthetic_jpeg()
    metadata = b"Exif\x00\x00II\x2a\x00\x08\x00\x00\x00"
    jpeg = jpeg[:2] + b"\xff\xe1" + (len(metadata) + 2).to_bytes(2, "big") + metadata + jpeg[2:]
    monkeypatch.setattr(cv2, "imdecode", lambda *args: pytest.fail("decoder called"))
    with pytest.raises(pipeline.InvalidImage):
        pipeline.decode_jpeg(jpeg)


def test_decoded_dimensions_must_equal_inspected_sof(monkeypatch):
    monkeypatch.setattr(cv2, "imdecode", lambda *args: np.zeros((1, 1, 3), dtype=np.uint8))
    with pytest.raises(pipeline.InvalidImage):
        pipeline.decode_jpeg(synthetic_jpeg())


@pytest.mark.parametrize("kind", ["missing", "corrupt"])
def test_model_failure_precedes_any_runtime_construction(monkeypatch, tmp_path, kind):
    if kind == "corrupt":
        (tmp_path / YUNET_FILENAME).write_bytes(b"corrupt synthetic bytes")
        (tmp_path / SFACE_FILENAME).write_bytes(b"corrupt synthetic bytes")
    monkeypatch.setattr(pipeline, "Detektor", lambda *args: pytest.fail("runtime created"))
    monkeypatch.setattr(pipeline, "Embedder", lambda *args: pytest.fail("runtime created"))
    with pytest.raises(pipeline.ModelUnavailable, match="^MODEL_UNAVAILABLE$"):
        pipeline.Pipeline(tmp_path)


def test_corrupt_sface_prevents_loading_verified_detector(monkeypatch, tmp_path):
    import hashlib

    verified_fixture = b"synthetic model-checksum fixture, never a usable model"
    (tmp_path / YUNET_FILENAME).write_bytes(verified_fixture)
    (tmp_path / SFACE_FILENAME).write_bytes(b"corrupt synthetic SFace")
    monkeypatch.setattr(pipeline, "YUNET_SHA256", hashlib.sha256(verified_fixture).hexdigest())
    monkeypatch.setattr(pipeline, "Detektor", lambda *args: pytest.fail("untrusted model pair loaded"))
    monkeypatch.setattr(pipeline, "Embedder", lambda *args: pytest.fail("untrusted model pair loaded"))
    with pytest.raises(pipeline.ModelUnavailable, match="^MODEL_UNAVAILABLE$"):
        pipeline.Pipeline(tmp_path)


def test_no_face_and_multiple_faces_do_not_extract():
    jpeg = synthetic_jpeg()
    for faces, status in [([], "NO_FACE"), ([synthetic_detection()] * 2, "MULTIPLE_FACES")]:
        instance = domain_pipeline(faces)
        instance._embedder.embedding = lambda *args: pytest.fail("embedding called")
        result = instance.process(jpeg)
        assert result.status == status
        assert result.face_count == len(faces)
        assert result.embedding.size == 0


def test_too_many_detections_fail_bounded():
    result = domain_pipeline([synthetic_detection()] * (pipeline.MAX_FACE_COUNT + 1)).process(synthetic_jpeg())
    assert result.status == "INFERENCE_FAILED"
    assert result.face_count == 0
    assert result.embedding.size == 0


def test_quality_gate_not_bypassed():
    instance = domain_pipeline([synthetic_detection()])
    instance._embedder.embedding = lambda *args: pytest.fail("embedding called")
    assert instance.process(synthetic_jpeg(uniform=True)).status == "LOW_QUALITY"
    face = synthetic_detection()
    points = face.landmarks / 2
    row = face.baris.copy()
    row[:14] /= 2
    small = replace(face, x=8, y=8, lebar=60, tinggi=60, landmarks=points, baris=row)
    assert domain_pipeline([small]).process(synthetic_jpeg()).status == "FACE_TOO_SMALL"


def test_domain_success_uses_existing_quality_and_embedder():
    result = domain_pipeline([synthetic_detection()]).process(synthetic_jpeg())
    assert result.status == "OK"
    assert result.face_count == 1
    assert result.embedding.dtype == np.float32
    assert result.embedding.shape == (128,)
    assert np.linalg.norm(result.embedding) == pytest.approx(1)


@pytest.mark.parametrize("defect", ["nan", "duplicate", "collinear", "outside", "row_missing", "row_shape", "score"])
def test_landmark_and_detection_defects_fail_closed(defect):
    face = synthetic_detection()
    points = face.landmarks.copy()
    row = face.baris.copy()
    if defect == "nan":
        points[0, 0] = np.nan
    elif defect == "duplicate":
        points[1] = points[0]
    elif defect == "collinear":
        points[:, 1] = 50
    elif defect == "outside":
        points[0, 0] = 10000
    row[4:14] = points.reshape(-1)
    face = replace(face, landmarks=points, baris=row)
    if defect == "row_missing":
        face = replace(face, baris=None)
    elif defect == "row_shape":
        face = replace(face, baris=row[:14])
    elif defect == "score":
        face = replace(face, skor=float("nan"))
    instance = domain_pipeline([face])
    instance._embedder.embedding = lambda *args: pytest.fail("invalid landmarks reached alignment")
    result = instance.process(synthetic_jpeg())
    assert result.status == "INFERENCE_FAILED"
    assert result.embedding.size == 0


@pytest.mark.parametrize("vector", [np.zeros(128, np.float32), np.ones(127, np.float32),
                                  np.full(128, np.nan, np.float32), np.full(128, np.inf, np.float32),
                                  np.ones(128, np.float64), np.ones((1, 128), np.float32)])
def test_invalid_embedding_never_returns_success(vector):
    result = domain_pipeline([synthetic_detection()], vector).process(synthetic_jpeg())
    assert result.status == "INFERENCE_FAILED"
    assert result.embedding.size == 0


def test_native_exception_is_sanitized():
    instance = domain_pipeline([])
    def fail(frame):
        raise RuntimeError("sensitive payload must not escape")
    instance._detector.deteksi = fail
    result = instance.process(synthetic_jpeg())
    assert result.status == "INFERENCE_FAILED"
    assert result.embedding.size == 0


def test_similarity_has_fingerprint_guard_and_no_threshold():
    vector = unit_vector()
    assert pipeline.similarity(vector, vector, pipeline.manifest(), pipeline.manifest()) == 1
    different = dict(pipeline.manifest(), pipeline_revision="different")
    with pytest.raises(ValueError, match="INCOMPATIBLE_MANIFEST"):
        pipeline.similarity(vector, vector, pipeline.manifest(), different)
    different = dict(pipeline.manifest(), synthetic_only=1)
    with pytest.raises(ValueError, match="INCOMPATIBLE_MANIFEST"):
        pipeline.similarity(vector, vector, different, pipeline.manifest())
    with pytest.raises(ValueError, match="INVALID_EMBEDDING"):
        pipeline.similarity(np.zeros(128, np.float32), vector, pipeline.manifest(), pipeline.manifest())


@pytest.mark.parametrize("invalid", [
    np.zeros(127, np.float32),
    np.full(128, np.nan, np.float32),
    np.full(128, np.inf, np.float32),
])
def test_similarity_rejects_invalid_query_and_template(invalid):
    vector = unit_vector()
    fingerprint = pipeline.manifest()
    with pytest.raises(ValueError, match="INVALID_EMBEDDING"):
        pipeline.similarity(invalid, vector, fingerprint, fingerprint)
    with pytest.raises(ValueError, match="INVALID_EMBEDDING"):
        pipeline.similarity(vector, invalid, fingerprint, fingerprint)


@pytest.mark.parametrize("field,value", [
    ("model_version", "incompatible-model-version"),
    ("model_checksum", "f" * 64),
])
def test_similarity_rejects_different_model_fingerprint(field, value):
    vector = unit_vector()
    trusted = pipeline.manifest()
    different = dict(trusted, **{field: value})
    with pytest.raises(ValueError, match="INCOMPATIBLE_MANIFEST"):
        pipeline.similarity(vector, vector, trusted, different)


@pytest.mark.parametrize("feature", [np.full((1, 128), np.nan, np.float32),
                                   np.full((1, 128), np.inf, np.float32),
                                   np.zeros((1, 128), np.float32), np.zeros((1, 127), np.float32)])
def test_embedder_rejects_nonfinite_zero_or_wrong_native_feature(feature):
    embedder = Embedder.__new__(Embedder)
    embedder._pengenal = SimpleNamespace(
        alignCrop=lambda frame, row: np.zeros((112, 112, 3), np.uint8),
        feature=lambda crop: feature,
    )
    with pytest.raises(GalatModel):
        embedder.embedding(np.zeros((160, 160, 3), np.uint8), synthetic_detection())


def test_embedder_rejects_bad_crop_before_feature():
    embedder = Embedder.__new__(Embedder)
    embedder._pengenal = SimpleNamespace(
        alignCrop=lambda frame, row: np.zeros((10, 10, 3), np.uint8),
        feature=lambda crop: pytest.fail("invalid crop reached feature"),
    )
    with pytest.raises(GalatModel):
        embedder.embedding(np.zeros((160, 160, 3), np.uint8), synthetic_detection())


@pytest.mark.model
def test_real_pinned_pipeline_rejects_generated_nonhuman_jpeg(konfigurasi):
    result = pipeline.Pipeline(konfigurasi.model_dir).process(synthetic_jpeg(uniform=True))
    assert result.status == "NO_FACE"
    assert result.face_count == 0
    assert result.embedding.size == 0

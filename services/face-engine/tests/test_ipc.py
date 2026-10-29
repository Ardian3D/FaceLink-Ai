"""Behavioral protocol/security tests, no camera, network, human images or vectors."""

import base64
import hashlib
import hmac
import io
import json
import struct
from types import SimpleNamespace

import cv2
import numpy as np
import pytest

from face_engine import ipc, pipeline

SECRET = bytes(range(32))
REQUEST_ID = "ed3edada-f4a9-4f34-8622-99ba7663d9c6"


def synthetic_jpeg():
    ok, jpeg = cv2.imencode(".jpg", np.zeros((100, 100, 3), dtype=np.uint8))
    assert ok
    return jpeg.tobytes()


def request_object(jpeg=None):
    if jpeg is None:
        jpeg = synthetic_jpeg()
    return {
        "version": 1,
        "request_id": REQUEST_ID,
        "artifact_sha256": hashlib.sha256(jpeg).hexdigest(),
        "jpeg_base64": base64.b64encode(jpeg).decode("ascii"),
    }


def frame(payload, secret=SECRET):
    if isinstance(payload, dict):
        payload = json.dumps(payload, separators=(",", ":")).encode()
    return struct.pack(">I", len(payload)) + hmac.digest(secret, payload, "sha256") + payload


def response(output):
    wire = output.getvalue()
    length = struct.unpack(">I", wire[:4])[0]
    assert 0 < length <= ipc.MAX_RESPONSE_BYTES
    assert len(wire) == 36 + length
    assert hmac.compare_digest(wire[4:36], hmac.digest(SECRET, wire[36:], "sha256"))
    result = json.loads(wire[36:])
    assert set(result) == {"version", "request_id", "artifact_sha256", "status", "face_count", "embedding", "manifest"}
    assert result["version"] == 1
    assert result["request_id"] == REQUEST_ID
    assert result["manifest"] == pipeline.manifest()
    return result


def test_one_authenticated_request_returns_bound_finite_response(monkeypatch, tmp_path):
    vector = np.zeros(128, dtype=np.float32)
    vector[0] = 1
    # Isolated domain dependency, never a production fallback or real-model evidence.
    calls = []
    def construct(model_dir):
        calls.append(model_dir)
        return SimpleNamespace(process=lambda jpeg: pipeline.ProcessingResult("OK", 1, vector))
    monkeypatch.setattr(ipc, "Pipeline", construct)
    request = request_object()
    output = io.BytesIO()
    ipc.serve_once(io.BytesIO(frame(request)), output, SECRET, tmp_path)
    result = response(output)
    assert calls == [tmp_path]
    assert result["artifact_sha256"] == request["artifact_sha256"]
    assert result["status"] == "OK"
    assert result["face_count"] == 1
    assert len(result["embedding"]) == 128
    assert np.isfinite(result["embedding"]).all()
    assert np.linalg.norm(result["embedding"]) == pytest.approx(1)


@pytest.mark.parametrize("mutate", [
    lambda data: data.update(version=2),
    lambda data: data.update(version=True),
    lambda data: data.update(version=1.0),
    lambda data: data.update(extra="unexpected"),
    lambda data: data.pop("jpeg_base64"),
    lambda data: data.update(request_id="not-a-uuid"),
    lambda data: data.update(request_id="ed3edada-f4a9-1f34-8622-99ba7663d9c6"),
    lambda data: data.update(request_id=REQUEST_ID.upper()),
    lambda data: data.update(artifact_sha256="A" * 64),
    lambda data: data.update(artifact_sha256=None),
    lambda data: data.update(jpeg_base64=None),
    lambda data: data.update(jpeg_base64="A" * (4 * ((pipeline.MAX_JPEG_BYTES + 2) // 3) + 1)),
])
def test_invalid_schema_has_no_response_or_inference(monkeypatch, tmp_path, mutate):
    request = request_object()
    mutate(request)
    monkeypatch.setattr(ipc, "Pipeline", lambda *args: pytest.fail("inference started"))
    output = io.BytesIO()
    with pytest.raises(ipc.ProtocolError, match="^INVALID_PROTOCOL$"):
        ipc.serve_once(io.BytesIO(frame(request)), output, SECRET, tmp_path)
    assert output.getvalue() == b""


@pytest.mark.parametrize("payload", [b"{} {}", b"[]", b"null", b"\xff", b'{"version":NaN}',
                                     b'{"version":1,"version":1}', b'{"version":Infinity}',
                                     b'[' * 1500 + b']' * 1500])
def test_ambiguous_or_nonfinite_json_rejected(payload):
    with pytest.raises(ipc.ProtocolError):
        ipc.read_request(io.BytesIO(frame(payload)), SECRET)


@pytest.mark.parametrize("size", [0, ipc.MAX_REQUEST_BYTES + 1, 0xFFFFFFFF])
def test_invalid_size_rejected_without_reading_body(size):
    class HeaderOnly(io.BytesIO):
        def read(self, size=-1):
            assert size <= 4
            return super().read(size)
    with pytest.raises(ipc.ProtocolError):
        ipc.read_request(HeaderOnly(struct.pack(">I", size)), SECRET)


@pytest.mark.parametrize("truncate", [1, 4, 35, 40])
def test_truncated_frame_rejected(truncate):
    wire = frame(request_object())
    with pytest.raises(ipc.ProtocolError):
        ipc.read_request(io.BytesIO(wire[:truncate]), SECRET)


def test_bad_authentication_rejected_before_json_decode():
    wire = frame(b"not JSON", bytes(reversed(SECRET)))
    with pytest.raises(ipc.ProtocolError):
        ipc.read_request(io.BytesIO(wire), SECRET)


@pytest.mark.parametrize("suffix", [b"\x00", b"garbage", frame(request_object())])
def test_trailing_bytes_and_replayed_second_frame_rejected(suffix):
    with pytest.raises(ipc.ProtocolError):
        ipc.read_request(io.BytesIO(frame(request_object()) + suffix), SECRET)


@pytest.mark.parametrize("secret", [b"", b"x" * 31, b"x" * 33])
def test_secret_must_be_exactly_32_bytes(secret):
    with pytest.raises(ipc.ProtocolError):
        ipc.read_request(io.BytesIO(frame(request_object())), secret)


@pytest.mark.parametrize("defect", ["digest", "base64", "padding", "nonjpeg", "oversize"])
def test_bad_artifact_returns_authenticated_invalid_image_without_models(monkeypatch, tmp_path, defect):
    request = request_object()
    if defect == "digest":
        request["artifact_sha256"] = "0" * 64
    elif defect == "base64":
        request["jpeg_base64"] = "not base64!"
    elif defect == "padding":
        request["jpeg_base64"] += "="
    elif defect == "nonjpeg":
        request = request_object(b"synthetic invalid image")
    elif defect == "oversize":
        # The final base64 block can decode two bytes over the byte limit while
        # remaining within the encoded-character cap; JPEG validation must reject.
        request = request_object(b"x" * (pipeline.MAX_JPEG_BYTES + 2))
    monkeypatch.setattr(ipc, "Pipeline", lambda *args: pytest.fail("models loaded for invalid image"))
    output = io.BytesIO()
    ipc.serve_once(io.BytesIO(frame(request)), output, SECRET, tmp_path)
    result = response(output)
    assert result["status"] == "INVALID_IMAGE"
    assert result["embedding"] == []
    assert result["face_count"] == 0
    assert result["artifact_sha256"] == request["artifact_sha256"]


def test_missing_models_return_authenticated_sanitized_failure(tmp_path):
    request = request_object()
    output = io.BytesIO()
    ipc.serve_once(io.BytesIO(frame(request)), output, SECRET, tmp_path)
    result = response(output)
    assert result["status"] == "MODEL_UNAVAILABLE"
    assert result["embedding"] == []
    assert str(tmp_path).encode() not in output.getvalue()


@pytest.mark.parametrize("status,count", [("NO_FACE", 0), ("MULTIPLE_FACES", 2), ("LOW_QUALITY", 1),
                                         ("FACE_TOO_SMALL", 1), ("INVALID_IMAGE", 0),
                                         ("MODEL_UNAVAILABLE", 0), ("INFERENCE_FAILED", 0)])
def test_allowlisted_failure_frames_always_have_no_vector(status, count):
    request = ipc.Request(REQUEST_ID, "a" * 64, "")
    output = io.BytesIO()
    ipc.write_response(output, SECRET, request, pipeline.ProcessingResult(status, count))
    result = response(output)
    assert result["status"] == status
    assert result["embedding"] == []


@pytest.mark.parametrize("result", [
    pipeline.ProcessingResult("UNKNOWN"),
    pipeline.ProcessingResult("NO_FACE", 1),
    pipeline.ProcessingResult("MULTIPLE_FACES", 1),
    pipeline.ProcessingResult("INFERENCE_FAILED", -1),
    pipeline.ProcessingResult("INFERENCE_FAILED", 1025),
    pipeline.ProcessingResult("INFERENCE_FAILED", True),
    pipeline.ProcessingResult("OK", 1, np.full(128, np.nan, np.float32)),
    pipeline.ProcessingResult("OK", 1, np.full(128, np.inf, np.float32)),
    pipeline.ProcessingResult("OK", 1, np.ones(127, np.float32)),
    pipeline.ProcessingResult("OK", 1, np.zeros(128, np.float32)),
    pipeline.ProcessingResult("OK", 0, np.ones(128, np.float32)),
    pipeline.ProcessingResult("NO_FACE", 0, np.ones(128, np.float32)),
])
def test_invalid_internal_results_cannot_write_success_or_partial_frame(result):
    output = io.BytesIO()
    with pytest.raises(ipc.ProtocolError):
        ipc.write_response(output, SECRET, ipc.Request(REQUEST_ID, "a" * 64, ""), result)
    assert output.getvalue() == b""


def test_runtime_exception_does_not_escape_response(monkeypatch, tmp_path):
    def fail(model_dir):
        raise RuntimeError("sensitive native details")
    monkeypatch.setattr(ipc, "Pipeline", fail)
    output = io.BytesIO()
    ipc.serve_once(io.BytesIO(frame(request_object())), output, SECRET, tmp_path)
    result = response(output)
    assert result["status"] == "INFERENCE_FAILED"
    assert b"sensitive native details" not in output.getvalue()

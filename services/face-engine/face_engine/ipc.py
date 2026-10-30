"""One authenticated request per process over bounded binary standard streams."""

from __future__ import annotations

import argparse
import base64
import binascii
import hashlib
import hmac
import json
import os
import re
import struct
import sys
import uuid
from dataclasses import dataclass, field
from pathlib import Path
from typing import BinaryIO

from .pipeline import (
    FAILURE_STATUSES,
    MAX_FACE_COUNT,
    MAX_JPEG_BYTES,
    InvalidImage,
    ModelUnavailable,
    Pipeline,
    ProcessingResult,
    _validated_vector,
    jpeg_dimensions,
    manifest,
)

VERSION = 1
MAX_REQUEST_BYTES = 1_400_000
MAX_RESPONSE_BYTES = 16_384
_REQUEST_KEYS = {"version", "request_id", "artifact_sha256", "jpeg_base64"}
_HEX_DIGEST = re.compile(r"[0-9a-f]{64}\Z")


class ProtocolError(ValueError):
    def __init__(self) -> None:
        super().__init__("INVALID_PROTOCOL")


@dataclass(frozen=True)
class Request:
    request_id: str
    artifact_sha256: str
    jpeg_base64: str = field(repr=False)


def _read_exact(stream: BinaryIO, size: int) -> bytes:
    data = bytearray()
    while len(data) < size:
        part = stream.read(size - len(data))
        if not part:
            raise ProtocolError()
        data.extend(part)
    return bytes(data)


def _unique_object(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result = {}
    for key, value in pairs:
        if key in result:
            raise ProtocolError()
        result[key] = value
    return result


def _reject_constant(value: str) -> None:
    raise ProtocolError()


def read_request(stream: BinaryIO, secret: bytes) -> Request:
    """Authenticate before JSON/base64 parsing; require EOF after the one frame."""
    if not isinstance(secret, bytes) or len(secret) != 32:
        raise ProtocolError()
    size = struct.unpack(">I", _read_exact(stream, 4))[0]
    if not 1 <= size <= MAX_REQUEST_BYTES:
        raise ProtocolError()
    authentication = _read_exact(stream, 32)
    payload = _read_exact(stream, size)
    if not hmac.compare_digest(authentication, hmac.digest(secret, payload, "sha256")):
        raise ProtocolError()
    if stream.read(1) != b"":
        raise ProtocolError()
    try:
        value = json.loads(payload.decode("utf-8"), object_pairs_hook=_unique_object,
                           parse_constant=_reject_constant)
    except (ValueError, UnicodeError, RecursionError):
        raise ProtocolError() from None
    if (not isinstance(value, dict) or set(value) != _REQUEST_KEYS
            or type(value["version"]) is not int or value["version"] != VERSION
            or not isinstance(value["request_id"], str)
            or not isinstance(value["artifact_sha256"], str)
            or not _HEX_DIGEST.fullmatch(value["artifact_sha256"])
            or not isinstance(value["jpeg_base64"], str)
            or len(value["jpeg_base64"]) > 4 * ((MAX_JPEG_BYTES + 2) // 3)):
        raise ProtocolError()
    try:
        request_id = uuid.UUID(value["request_id"])
    except (ValueError, AttributeError):
        raise ProtocolError() from None
    if request_id.version != 4 or str(request_id) != value["request_id"]:
        raise ProtocolError()
    return Request(value["request_id"], value["artifact_sha256"], value["jpeg_base64"])


def _process(request: Request, model_dir: Path) -> ProcessingResult:
    try:
        jpeg = base64.b64decode(request.jpeg_base64, validate=True)
        # Reject noncanonical encodings (including nonzero pad bits and excess padding).
        if base64.b64encode(jpeg).decode("ascii") != request.jpeg_base64:
            return ProcessingResult("INVALID_IMAGE")
        if not hmac.compare_digest(hashlib.sha256(jpeg).hexdigest(), request.artifact_sha256):
            return ProcessingResult("INVALID_IMAGE")
        jpeg_dimensions(jpeg)
    except (ValueError, binascii.Error, InvalidImage):
        return ProcessingResult("INVALID_IMAGE")
    try:
        return Pipeline(model_dir).process(jpeg)
    except ModelUnavailable:
        return ProcessingResult("MODEL_UNAVAILABLE")
    except Exception:
        return ProcessingResult("INFERENCE_FAILED")


def write_response(stream: BinaryIO, secret: bytes, request: Request,
                   result: ProcessingResult) -> None:
    """Write only a strict, finite, request-bound response and its authentication."""
    if not isinstance(secret, bytes) or len(secret) != 32:
        raise ProtocolError()
    if (type(result.face_count) is not int or not 0 <= result.face_count <= MAX_FACE_COUNT
            or result.status not in FAILURE_STATUSES | {"OK"}):
        raise ProtocolError()
    if result.status == "OK":
        if result.face_count != 1:
            raise ProtocolError()
        try:
            embedding = _validated_vector(result.embedding).tolist()
        except ValueError:
            raise ProtocolError() from None
    else:
        if (result.embedding.size != 0
                or (result.status == "NO_FACE" and result.face_count != 0)
                or (result.status == "MULTIPLE_FACES" and result.face_count < 2)):
            raise ProtocolError()
        embedding = []
    response = {
        "version": VERSION,
        "request_id": request.request_id,
        "artifact_sha256": request.artifact_sha256,
        "status": result.status,
        "face_count": result.face_count,
        "embedding": embedding,
        "manifest": manifest(),
    }
    payload = json.dumps(response, allow_nan=False, separators=(",", ":")).encode("utf-8")
    if len(payload) > MAX_RESPONSE_BYTES:
        raise ProtocolError()
    stream.write(struct.pack(">I", len(payload)) + hmac.digest(secret, payload, "sha256") + payload)
    stream.flush()


def serve_once(input_stream: BinaryIO, output_stream: BinaryIO,
               secret: bytes, model_dir: Path) -> None:
    request = read_request(input_stream, secret)
    write_response(output_stream, secret, request, _process(request, model_dir))


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Synthetic-only local CV IPC")
    parser.add_argument("--model-dir", required=True, type=Path)
    arguments = parser.parse_args(argv)
    secret_hex = os.environ.pop("FACELINK_CV_SECRET", "")
    if not _HEX_DIGEST.fullmatch(secret_hex) or not arguments.model_dir.is_absolute():
        return 1
    try:
        serve_once(sys.stdin.buffer, sys.stdout.buffer, bytes.fromhex(secret_hex),
                   arguments.model_dir)
    except Exception:
        # No traceback, payload, vector, key, path, or native exception is exposed.
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

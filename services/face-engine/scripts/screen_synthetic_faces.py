"""FL-ENR04-R1: six original procedural mannequin masks, never human photos.

SPDX-License-Identifier: CC0-1.0
This dedication applies only to this original fixture generator and its generated
fixtures, not FaceLink's other code, dependencies, or model weights. No imported
asset, human scan, identity dataset, pretrained generator, or network is used.
The renderer is a 2.5D parametric height-field, not a scanned/learned human mesh.
"""
from __future__ import annotations

import argparse
import base64
import ctypes
import hashlib
import hmac
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import time
import uuid

import cv2
import numpy as np

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from face_engine.pipeline import Pipeline, _valid_detection, decode_jpeg, manifest, similarity
from face_engine.quality import nilai_frame
from face_engine.config import DETECTOR_SCORE_THRESHOLD

VERSION = "procedural-mannequin-heightfield-v1"
CANDIDATES = 6


def render(index: int) -> bytes:
    """All coordinates and colors are invented; index selects six fixed styles."""
    if not 0 <= index < CANDIDATES:
        raise ValueError("candidate outside fixed experiment")
    size = 512
    yy, xx = np.mgrid[:size, :size].astype(np.float64)
    x, y = (xx - 256) / 200, (yy - 246) / 210
    width = .83 + .035 * (index % 3)
    outline = 1 - (x / (width * (1 - .13 * np.maximum(y, 0)))) ** 2 - (y / 1.04) ** 2
    mask = outline > 0
    z = .55 * np.sqrt(np.maximum(outline, 0))
    def bump(cx, cy, sx, sy, height):
        return height * np.exp(-((x - cx) / sx) ** 2 - ((y - cy) / sy) ** 2)
    for sign in (-1, 1):
        z += bump(sign * .34, -.23, .23, .14, -.065)
        z += bump(sign * .34, -.42, .26, .09, .045)
        z += bump(sign * .42, .12, .28, .29, .055)
    z += bump(0, -.03, .115, .40, .16) + bump(0, .19, .16, .14, .19)
    z += bump(0, .44, .30, .085, .045) + bump(0, .76, .32, .16, .04)
    gy, gx = np.gradient(z, 1 / 210, 1 / 200)
    normal = np.stack((-gx, -gy, np.ones_like(z)), axis=2)
    normal /= np.linalg.norm(normal, axis=2, keepdims=True)
    light = np.array([-.35 + .12 * (index % 3), -.40, .85])
    light /= np.linalg.norm(light)
    shade = .45 + .55 * np.maximum(normal @ light, 0)
    skin = np.array([(162, 187, 214), (151, 176, 202), (175, 193, 209)][index % 3])
    texture = np.random.default_rng(4400 + index).normal(0, .8, (size, size, 1))
    image = np.full((size, size, 3), (76, 81, 86), np.uint8)
    surface = np.clip(shade[..., None] * skin + texture, 0, 255).astype(np.uint8)
    image[mask] = surface[mask]
    def point(a, b):
        return (round(256 + a * 200), round(246 + b * 210))
    # Parametric ears, sclera/iris/pupil, lids and brows; no image texture assets.
    for sign in (-1, 1):
        cv2.ellipse(image, point(sign * .83, .01), (17, 41), 0, 0, 360, (137, 164, 188), -1, cv2.LINE_AA)
        center = point(sign * (.32 + .015 * (index % 2)), -.23)
        ew, eh = 35 + index % 3, 13 + index % 2
        cx, cy = center
        almond = np.array([(cx-ew,cy), (cx-ew//2,cy-eh), (cx+ew//2,cy-eh), (cx+ew,cy), (cx+ew//2,cy+eh), (cx-ew//2,cy+eh)], np.int32)
        cv2.fillConvexPoly(image, almond, (215, 217, 220), cv2.LINE_AA)
        cv2.circle(image, center, 12, (64, 85, 92), -1, cv2.LINE_AA)
        cv2.circle(image, center, 6, (18, 20, 22), -1, cv2.LINE_AA)
        cv2.circle(image, (cx-3,cy-4), 2, (247, 247, 247), -1, cv2.LINE_AA)
        cv2.polylines(image, [almond[:4]], False, (63, 65, 72), 2, cv2.LINE_AA)
        brow = np.array([(cx-ew-5,cy-28), (cx-10,cy-34), (cx+14,cy-33), (cx+ew,cy-26)], np.int32)
        cv2.polylines(image, [brow], False, (51, 57, 66), 6 if index < 3 else 4, cv2.LINE_AA)
        cv2.ellipse(image, point(sign*.09,.235), (8,4), sign*12, 0, 360, (72, 91, 113), -1, cv2.LINE_AA)
    upper = np.array([point(-.27,.45),point(-.13,.405),point(0,.425),point(.12,.405),point(.27,.45),point(0,.47)], np.int32)
    lower = np.array([point(-.27,.45),point(0,.47),point(.27,.45),point(.13,.515),point(-.13,.515)], np.int32)
    cv2.fillConvexPoly(image, upper, (100, 111, 156), cv2.LINE_AA)
    cv2.fillConvexPoly(image, lower, (122, 133, 175), cv2.LINE_AA)
    cv2.line(image, point(-.25,.45),point(.25,.45),(61,73,104),2,cv2.LINE_AA)
    if index >= 3:
        hair = mask & (y < -.69 + .065 * np.cos(x * 4))
        image[hair] = np.clip(surface[hair] * .24, 0, 255).astype(np.uint8)
    ok, encoded = cv2.imencode(".jpg", image, [cv2.IMWRITE_JPEG_QUALITY, 90])
    if not ok:
        raise RuntimeError("fixture encoding failed")
    return encoded.tobytes()


def ipc(jpeg: bytes) -> dict:
    secret = os.urandom(32)
    request_id = str(uuid.uuid4())
    digest = hashlib.sha256(jpeg).hexdigest()
    payload = json.dumps(dict(version=1, request_id=request_id, artifact_sha256=digest,
                              jpeg_base64=base64.b64encode(jpeg).decode()), separators=(",", ":")).encode()
    frame = struct.pack(">I",len(payload)) + hmac.digest(secret,payload,"sha256") + payload
    env = {k: v for k,v in os.environ.items() if k.upper() in {"SYSTEMROOT","WINDIR"}}
    env.update(FACELINK_CV_SECRET=secret.hex(), PYTHONNOUSERSITE="1", PYTHONDONTWRITEBYTECODE="1", OMP_NUM_THREADS="1", OPENBLAS_NUM_THREADS="1", MKL_NUM_THREADS="1")
    start = time.perf_counter()
    result = subprocess.run([sys.executable,"-m","face_engine.ipc","--model-dir",str(ROOT/"models")], cwd=ROOT, env=env, input=frame, capture_output=True, timeout=11)
    elapsed = (time.perf_counter()-start)*1000
    if result.returncode != 0 or result.stderr or not 36 <= len(result.stdout) <= 16420:
        raise RuntimeError("screening IPC process/output rejected")
    output = result.stdout
    if struct.unpack(">I",output[:4])[0] != len(output)-36:
        raise RuntimeError("screening IPC framing rejected")
    if not hmac.compare_digest(output[4:36],hmac.digest(secret,output[36:],"sha256")):
        raise RuntimeError("screening IPC authentication rejected")
    response = json.loads(output[36:])
    if (response["version"] != 1 or response["request_id"] != request_id
            or response["artifact_sha256"] != digest or response["manifest"] != manifest()):
        raise RuntimeError("screening IPC request/model binding rejected")
    # No output bytes, features, or credentials are returned to evidence.
    return dict(status=response["status"], face_count=response["face_count"], authenticated=True,
                round_trip_ms=round(elapsed,3), process_exit=result.returncode, reaped=True)


def peak_memory_mib():
    if os.name != "nt":
        return None
    class Counters(ctypes.Structure):
        _fields_ = [("cb",ctypes.c_ulong),("faults",ctypes.c_ulong)] + [(name,ctypes.c_size_t) for name in ("peak","working","pp","p","pnp","np","page","peakpage")]
    counters = Counters(); counters.cb=ctypes.sizeof(counters)
    handle=ctypes.c_void_p(ctypes.windll.kernel32.GetCurrentProcess())
    if ctypes.windll.psapi.GetProcessMemoryInfo(handle,ctypes.byref(counters),counters.cb):
        return round(counters.peak/1048576,3)
    return None


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output",required=True,type=Path)
    args=parser.parse_args()
    args.output.mkdir(parents=True,exist_ok=True)
    # Never overwrite existing evidence; reruns use another explicit directory.
    if any(args.output.iterdir()):
        raise ValueError("output directory must be empty")
    start=time.perf_counter(); engine=Pipeline(ROOT/"models"); init_ms=(time.perf_counter()-start)*1000
    rows=[]
    for index in range(CANDIDATES):
        jpeg=render(index); name=f"{VERSION}-{index+1:02}.jpg"
        (args.output/name).write_bytes(jpeg)
        t=time.perf_counter(); image=decode_jpeg(jpeg); decoded=time.perf_counter()
        faces=engine._detector.deteksi(image); detected=time.perf_counter()
        quality=nilai_frame(image,faces)
        row=dict(file=name,sha256=hashlib.sha256(jpeg).hexdigest(),bytes=len(jpeg),dimensions=[512,512],face_count=len(faces),
                 confidence=[round(f.skor,6) for f in faces],landmarks_valid=all(_valid_detection(f,512,512) for f in faces) if faces else False,
                 quality_status="OK" if quality.diterima else quality.kode,quality_metrics=quality.rincian,
                 decode_ms=round((decoded-t)*1000,3),yunet_ms=round((detected-decoded)*1000,3),alignment_ms=None,sface_ms=None)
        if len(faces)==1 and row["landmarks_valid"] and quality.diterima:
            a=time.perf_counter(); crop=engine._embedder._pengenal.alignCrop(image,faces[0].baris); aligned=time.perf_counter()
            feature=engine._embedder._pengenal.feature(crop); extracted=time.perf_counter()
            row.update(alignment_ms=round((aligned-a)*1000,3),sface_ms=round((extracted-aligned)*1000,3),crop_shape=list(crop.shape))
            feature.fill(0); crop.fill(0)
        t=time.perf_counter(); processed=engine.process(jpeg); row["pipeline_ms"]=round((time.perf_counter()-t)*1000,3)
        row["pipeline_status"]=processed.status
        if processed.status=="OK":
            row.update(embedding_dimension=int(processed.embedding.size),embedding_dtype=str(processed.embedding.dtype),
                       embedding_finite=bool(np.isfinite(processed.embedding).all()),self_similarity=similarity(processed.embedding,processed.embedding,manifest(),manifest()))
            processed.embedding.fill(0)
        row["ipc"]=ipc(jpeg)
        rows.append(row)
    report=dict(version=VERSION,license="CC0-1.0 (fixture generator/output only)",provenance="Original local equations and drawing primitives; no images, scans, trained generator, identity dataset, network or external assets",generator_sha256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                runtime=dict(python=sys.version.split()[0],opencv=cv2.__version__,numpy=np.__version__),manifest=manifest(),detection_threshold=DETECTOR_SCORE_THRESHOLD,model_init_ms=round(init_ms,3),peak_working_set_mib=peak_memory_mib(),candidate_limit=CANDIDATES,candidates=rows,
                real_model_positive=any(r["pipeline_status"]=="OK" and r["ipc"]["status"]=="OK" for r in rows))
    (args.output/"screening.json").write_text(json.dumps(report,indent=2)+"\n",encoding="utf-8")
    print(json.dumps(report,indent=2))


if __name__ == "__main__":
    main()

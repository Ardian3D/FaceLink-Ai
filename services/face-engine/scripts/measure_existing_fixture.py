"""Measure only a pinned existing procedural fixture; no candidate generation/search."""
from __future__ import annotations
import argparse
import hashlib
import json
from pathlib import Path
import sys
import time
import numpy as np

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from face_engine.pipeline import Pipeline, decode_jpeg, _valid_detection, manifest
from face_engine.quality import nilai_frame
from scripts.screen_synthetic_faces import ipc, peak_memory_mib


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixtures', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    if args.output.exists():
        raise ValueError('never overwrite evidence')
    original = json.loads((args.fixtures / 'screening.json').read_text())
    entry = original['candidates'][0]
    jpeg = (args.fixtures / entry['file']).read_bytes()
    if hashlib.sha256(jpeg).hexdigest() != entry['sha256']:
        raise ValueError('fixture pin mismatch')
    started = time.perf_counter()
    engine = Pipeline(ROOT / 'models')
    initialized = time.perf_counter()
    frame = decode_jpeg(jpeg)
    decoded = time.perf_counter()
    faces = engine._detector.deteksi(frame)
    detected = time.perf_counter()
    quality = nilai_frame(frame, faces)
    if len(faces) != 1 or not quality.diterima or not _valid_detection(faces[0], frame.shape[1], frame.shape[0]):
        raise RuntimeError('existing fixture did not pass unchanged real detector/quality gate')
    aligned_start = time.perf_counter()
    crop = engine._embedder._pengenal.alignCrop(frame, faces[0].baris)
    aligned = time.perf_counter()
    feature = engine._embedder._pengenal.feature(crop)
    extracted = time.perf_counter()
    crop_shape = list(crop.shape)
    feature.fill(0)
    crop.fill(0)
    begin = time.perf_counter()
    result = engine.process(jpeg)
    processed = time.perf_counter()
    if result.status != 'OK':
        raise RuntimeError('real pipeline did not succeed')
    vector = result.embedding
    metadata = dict(dimension=int(vector.size), dtype=str(vector.dtype), finite=bool(np.isfinite(vector).all()), unit_normalized=abs(float(np.linalg.norm(vector.astype(np.float64)))-1)<1e-5)
    vector.fill(0)
    output = dict(task='FL-ENR04-R2', fixture_version=original['version'], fixture_sha256=entry['sha256'], new_unique_candidates=0, dimensions=list(frame.shape[:2][::-1]), face_count=len(faces), confidence=round(faces[0].skor,6), landmarks_valid=True, quality_status='OK', aligned_shape=crop_shape, embedding=metadata, manifest=manifest(), timings_ms=dict(model_init=round((initialized-started)*1000,3),decode=round((decoded-initialized)*1000,3),yunet=round((detected-decoded)*1000,3),alignment=round((aligned-aligned_start)*1000,3),sface=round((extracted-aligned)*1000,3),pipeline=round((processed-begin)*1000,3)), ipc=ipc(jpeg), peak_working_set_mib=peak_memory_mib(), memory_zeroization_claimed=False)
    args.output.parent.mkdir(parents=True,exist_ok=True)
    args.output.write_text(json.dumps(output,indent=2)+'\n')
    print(json.dumps(output,indent=2))


if __name__ == '__main__':
    main()

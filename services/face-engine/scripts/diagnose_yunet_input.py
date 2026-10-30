"""FL-ENR04-R2: compare existing synthetic fixture input with direct OpenCV API.

No new face candidates, no threshold override, no box/landmark injection.
"""
from __future__ import annotations
import argparse
import hashlib
import json
from pathlib import Path
import sys
import cv2
import numpy as np

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from face_engine.config import DETECTOR_NMS_THRESHOLD, DETECTOR_SCORE_THRESHOLD, DETECTOR_TOP_K, YUNET_FILENAME
from face_engine.detector import ukuran_masukan
from face_engine.pipeline import Pipeline, decode_jpeg


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixtures', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    if args.output.exists():
        raise ValueError('never overwrite diagnostic evidence')
    metadata = json.loads((args.fixtures/'screening.json').read_text())
    engine = Pipeline(ROOT/'models')
    rows=[]
    for entry in metadata['candidates']:
        jpeg=(args.fixtures/entry['file']).read_bytes()
        if hashlib.sha256(jpeg).hexdigest()!=entry['sha256']:
            raise ValueError('fixture pin mismatch')
        frame=decode_jpeg(jpeg)
        ordinary=cv2.imdecode(np.frombuffer(jpeg,np.uint8),cv2.IMREAD_COLOR)
        height,width=frame.shape[:2]
        model_size=ukuran_masukan(width,height)
        model_input=cv2.resize(frame,model_size,interpolation=cv2.INTER_AREA) if model_size!=(width,height) else frame
        wrapped=engine._detector.deteksi(frame)
        cases=[]
        # Same pinned model and existing score/NMS/topK throughout. Native-size
        # and channel-reversed calls diagnose transforms, not new face fixtures.
        for label,image in [('same_wrapper_input_bgr',model_input),('native_size_bgr',frame),('channel_reversed_diagnostic_only',frame[:,:,::-1].copy())]:
            h,w=image.shape[:2]
            direct=cv2.FaceDetectorYN.create(str(ROOT/'models'/YUNET_FILENAME),'',(w,h),DETECTOR_SCORE_THRESHOLD,DETECTOR_NMS_THRESHOLD,DETECTOR_TOP_K)
            direct.setInputSize((w,h))
            _,faces=direct.detect(image)
            cases.append(dict(case=label,width=w,height=h,set_input_size=list(direct.getInputSize()),face_count=0 if faces is None else len(faces),confidence=[] if faces is None else [round(float(x[14]),6) for x in faces]))
        rows.append(dict(file=entry['file'],sha256=entry['sha256'],decoded_shape=list(frame.shape),dtype=str(frame.dtype),channel_contract='BGR uint8',ordinary_decode_matches_ignore_orientation=bool(np.array_equal(frame,ordinary)),app1_present=b'Exif\0\0' in jpeg,wrapper_model_size=list(model_size),relative_aspect_error=abs((model_size[0]/model_size[1])/(width/height)-1),wrapper_face_count=len(wrapped),direct_calls=cases))
    out=dict(task='FL-ENR04-R2',threshold=DETECTOR_SCORE_THRESHOLD,nms=DETECTOR_NMS_THRESHOLD,top_k=DETECTOR_TOP_K,existing_fixture_version=metadata['version'],native_preprocessing='cv2.imdecode BGR uint8; explicit width,height setInputSize; no blobFromImage/swapRB/crop',rows=rows)
    args.output.parent.mkdir(parents=True,exist_ok=True)
    args.output.write_text(json.dumps(out,indent=2)+'\n')
    print(json.dumps(out,indent=2))


if __name__=='__main__':
    main()

"""R2 native-resolution regression using the pinned wholly procedural R1 fixture."""
import numpy as np
import pytest
from face_engine.detector import ukuran_masukan
from face_engine.pipeline import Pipeline
from scripts.screen_synthetic_faces import render


@pytest.mark.parametrize('width,height', [(512,512), (640,480), (321,257)])
def test_small_frames_preserve_native_dimensions(width, height):
    # Upsampling changes the apparent scale seen by YuNet and loses valid detections.
    assert ukuran_masukan(width,height)==(width,height)


@pytest.mark.model
def test_real_procedural_face_extracts_at_native_scale(konfigurasi):
    engine=Pipeline(konfigurasi.model_dir)
    result=engine.process(render(0))
    assert result.status=='OK'
    assert result.face_count==1
    assert result.embedding.shape==(128,)
    assert result.embedding.dtype==np.float32
    assert np.isfinite(result.embedding).all()
    assert abs(float(np.linalg.norm(result.embedding.astype(np.float64)))-1)<1e-5
    result.embedding.fill(0)

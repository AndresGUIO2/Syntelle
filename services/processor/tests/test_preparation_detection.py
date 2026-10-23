import numpy as np
from PIL import Image
import pytest

from app.preparation.detection import PreparationDetector
from app.preparation.faces import PreparationError


def test_luminance_fallback_preserves_frame_and_original_pixels(monkeypatch):
    calls=[];geometry=object()
    class Detector:
        def __init__(self,_):pass
        def detect(self,image):
            calls.append(np.asarray(image).copy())
            if len(calls)==1:raise PreparationError('No face detected. Choose a clear frontal portrait.')
            return geometry
        def close(self):pass
    monkeypatch.setattr('app.preparation.detection.FaceDetector',Detector)
    original=Image.new('RGB',(30,20),(30,100,190));before=original.tobytes()
    with PreparationDetector(None) as detector:
        assert detector.detect(original) is geometry
        assert detector.last_view=='luminance-rgb'
    assert original.tobytes()==before and calls[1].shape==calls[0].shape
    assert np.array_equal(calls[1][...,0],calls[1][...,1])


def test_luminance_cannot_bypass_multiple_face_rejection(monkeypatch):
    calls=[]
    class Detector:
        def __init__(self,_):pass
        def detect(self,image):calls.append(1);raise PreparationError('Choose a portrait containing exactly one face.')
        def close(self):pass
    monkeypatch.setattr('app.preparation.detection.FaceDetector',Detector)
    with PreparationDetector(None) as detector:
        with pytest.raises(PreparationError,match='exactly one face'):detector.detect(Image.new('RGB',(20,20)))
    assert len(calls)==1

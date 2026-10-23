"""Bounded color-independent detection; never change original image bytes."""
from PIL import ImageOps

from .faces import FaceDetector, PreparationError


class PreparationDetector:
    def __init__(self,model):
        self.detector=FaceDetector(model);self.last_view=None

    def detect(self,image):
        self.last_view='original-rgb'
        try:return self.detector.detect(image)
        except PreparationError as error:
            # Multiple faces and malformed geometry remain failures. A second
            # color representation must not hide extra people or alter framing.
            if not str(error).startswith('No face detected.'):raise
        self.last_view='luminance-rgb'
        return self.detector.detect(ImageOps.grayscale(image).convert('RGB'))

    def __enter__(self):return self
    def __exit__(self,*_):self.detector.close()

from dataclasses import dataclass
from hashlib import sha256
from pathlib import Path
from urllib.request import urlopen

import numpy as np
from PIL import Image

MODEL_URL = "https://storage.googleapis.com/mediapipe-models/face_landmarker/face_landmarker/float16/1/face_landmarker.task"
MODEL_SHA256 = "64184e229b263107bc2b804c6625db1341ff2bb731874b0bcc2fe6544e0bc9ff"


class PreparationError(Exception):
    pass


def ensure_model(path: Path) -> None:
    if not path.exists():
        path.parent.mkdir(parents=True, exist_ok=True)
        with urlopen(MODEL_URL, timeout=60) as response:
            data = response.read(10_000_001)
        if sha256(data).hexdigest() != MODEL_SHA256:
            raise PreparationError("Downloaded face model failed its checksum check.")
        path.write_bytes(data)
    if sha256(path.read_bytes()).hexdigest() != MODEL_SHA256:
        raise PreparationError("Face model checksum mismatch; use the pinned model bundle.")


@dataclass
class FaceGeometry:
    points: np.ndarray
    blendshapes: dict[str, float]
    matrix: list[list[float]]

    def as_dict(self) -> dict:
        return {"landmarks_px": self.points.tolist(), "blendshapes": self.blendshapes, "transformation_matrix": self.matrix}


class FaceDetector:
    def __init__(self, model_path: Path):
        import mediapipe as mp

        ensure_model(model_path)
        self.mp = mp
        options = mp.tasks.vision.FaceLandmarkerOptions(
            base_options=mp.tasks.BaseOptions(model_asset_path=str(model_path)),
            running_mode=mp.tasks.vision.RunningMode.IMAGE,
            num_faces=2,
            output_face_blendshapes=True,
            output_facial_transformation_matrixes=True,
        )
        self.landmarker = mp.tasks.vision.FaceLandmarker.create_from_options(options)

    def detect(self, image: Image.Image) -> FaceGeometry:
        pixels = np.ascontiguousarray(image.convert("RGB"))
        result = self.landmarker.detect(self.mp.Image(image_format=self.mp.ImageFormat.SRGB, data=pixels))
        if not result.face_landmarks:
            raise PreparationError("No face detected. Choose a clear frontal portrait.")
        if len(result.face_landmarks) != 1:
            raise PreparationError("Choose a portrait containing exactly one face.")
        points = np.array([[p.x * image.width, p.y * image.height, p.z * image.width] for p in result.face_landmarks[0]])
        if points.shape != (478, 3) or not np.isfinite(points).all():
            raise PreparationError("Incomplete facial landmarks.")
        return FaceGeometry(
            points=points,
            blendshapes={item.category_name: float(item.score) for item in result.face_blendshapes[0]},
            matrix=result.facial_transformation_matrixes[0].tolist(),
        )

    def close(self) -> None:
        self.landmarker.close()

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()

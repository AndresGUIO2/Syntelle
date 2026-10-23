from dataclasses import dataclass
from pathlib import Path

import cv2
import numpy as np
from PIL import Image, ImageDraw

from .faces import FaceGeometry, PreparationError

# Eye corners, brow/forehead, temples and nose bridge; exclude lips and jaw.
STABLE_ANCHORS = (33, 133, 263, 362, 168, 6, 197, 195, 10, 70, 300, 127, 356)


def lip_indices() -> list[int]:
    from mediapipe.tasks.python.vision import FaceLandmarksConnections

    return sorted({index for edge in FaceLandmarksConnections.FACE_LANDMARKS_LIPS for index in (edge.start, edge.end)})


def fit_similarity(source: np.ndarray, target: np.ndarray) -> tuple[np.ndarray, float]:
    """Fit rotation, uniform scale and translation without deforming expression."""
    source_center, target_center = source.mean(axis=0), target.mean(axis=0)
    src, dst = source - source_center, target - target_center
    variance = float(np.sum(src * src))
    if variance < 1e-8:
        raise PreparationError("Facial anchors cannot be aligned.")
    u, singular, vt = np.linalg.svd(src.T @ dst)
    correction = np.eye(2)
    correction[-1, -1] = np.sign(np.linalg.det(u @ vt))
    rotation = u @ correction @ vt
    scale = float(np.sum(singular * correction.diagonal()) / variance)
    linear = scale * rotation
    translation = target_center - source_center @ linear
    transform = np.column_stack((linear.T, translation))
    predicted = source @ linear + translation
    residual = float(np.sqrt(np.mean(np.sum((predicted - target) ** 2, axis=1))))
    return transform, residual


def transform_points(points: np.ndarray, transform: np.ndarray) -> np.ndarray:
    return points[:, :2] @ transform[:, :2].T + transform[:, 2]


def align_variant(base: Image.Image, variant: Image.Image, base_face: FaceGeometry, variant_face: FaceGeometry):
    anchors = np.array(STABLE_ANCHORS)
    transform, residual = fit_similarity(variant_face.points[anchors, :2], base_face.points[anchors, :2])
    eye_distance = float(np.linalg.norm(base_face.points[33, :2] - base_face.points[263, :2]))
    scale = float(np.linalg.norm(transform[:, 0]))
    angle = float(np.degrees(np.arctan2(transform[1, 0], transform[0, 0])))
    if eye_distance < 20 or residual > eye_distance * 0.035 or not 0.90 <= scale <= 1.10 or abs(angle) > 5:
        raise PreparationError("Generated face changed too much for reliable local alignment.")
    pixels = cv2.warpAffine(
        np.asarray(variant.convert("RGB")), transform, base.size,
        flags=cv2.INTER_LANCZOS4, borderMode=cv2.BORDER_REFLECT_101,
    )
    return Image.fromarray(pixels), transform_points(variant_face.points, transform), {
        "transform": transform.tolist(), "anchor_rmse_px": residual, "scale": scale, "rotation_degrees": angle,
    }


def make_mouth_mask(size: tuple[int, int], base_points: np.ndarray, variant_points: np.ndarray) -> Image.Image:
    """Build a bounded feathered mask around the union of both lip contours."""
    indices = lip_indices()
    lip_points = np.concatenate((base_points[indices, :2], variant_points[indices, :2]))
    width = float(np.linalg.norm(base_points[61, :2] - base_points[291, :2]))
    if width < 10 or not np.isfinite(lip_points).all():
        raise PreparationError("Mouth landmarks are not usable.")
    dilation = max(2, round(width * 0.12))
    feather = max(1.0, width * 0.025)
    padding = dilation + int(np.ceil(feather * 4)) + 2
    left, top = np.floor(lip_points.min(axis=0) - padding).astype(int)
    right, bottom = np.ceil(lip_points.max(axis=0) + padding).astype(int)
    # Keep the mask below the nose and above the lower chin.
    nose_floor = max(base_points[2, 1], variant_points[2, 1])
    top = max(0, int(top), int(np.ceil(nose_floor + width * 0.07)))
    bottom = min(size[1], int(bottom), int(base_points[152, 1] - width * 0.08))
    left, right = max(0, int(left)), min(size[0], int(right))
    if right <= left or bottom <= top:
        raise PreparationError("Mouth mask lies outside the portrait.")
    local_points = np.rint(lip_points - [left, top]).astype(np.int32)
    hull = cv2.convexHull(local_points)
    local = np.zeros((bottom - top, right - left), dtype=np.uint8)
    cv2.fillConvexPoly(local, hull, 255)
    kernel = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (2 * dilation + 1, 2 * dilation + 1))
    local = cv2.dilate(local, kernel)
    local = cv2.GaussianBlur(local, (0, 0), feather, borderType=cv2.BORDER_CONSTANT)
    local[[0, -1], :] = 0
    local[:, [0, -1]] = 0
    mask = Image.new("L", size, 0)
    mask.paste(Image.fromarray(local), (left, top))
    return mask


@dataclass
class MouthComposition:
    image: Image.Image
    overlay: Image.Image
    mask: Image.Image
    bounds: tuple[int, int, int, int]
    metrics: dict


def export_lossless_pair(image: Image.Image, directory: Path, name: str) -> None:
    image.save(directory / f"{name}.png")
    image.save(directory / f"{name}.webp", format="WEBP", lossless=True, exact=True, method=6)
    with Image.open(directory / f"{name}.webp") as decoded:
        identical = np.array_equal(np.asarray(decoded.convert("RGBA")), np.asarray(image.convert("RGBA")))
    if not identical:
        raise PreparationError("Lossless WebP export changed image pixels.")


def export_overlay(result: MouthComposition, directory: Path, name: str = "mouth-a") -> None:
    export_lossless_pair(result.overlay, directory, name)
    result.metrics["lossless_webp_rgba_identical"] = True


def compose_mouth(base: Image.Image, variant: Image.Image, base_face: FaceGeometry, variant_face: FaceGeometry, viseme: str = "A") -> MouthComposition:
    base = base.convert("RGB")
    aligned, points, alignment = align_variant(base, variant, base_face, variant_face)
    mouth_width = float(np.linalg.norm(base_face.points[61, :2] - base_face.points[291, :2]))
    base_opening = float(np.linalg.norm(base_face.points[13, :2] - base_face.points[14, :2]) / mouth_width)
    opening = float(np.linalg.norm(points[13] - points[14]) / mouth_width)
    if viseme == "A" and opening < max(0.035, base_opening + 0.02):
        raise PreparationError("The generated A viseme does not visibly open the mouth.")
    if viseme == "MBP" and opening > 0.035:
        raise PreparationError("The generated MBP pose does not close the lips.")
    if viseme in {"EI", "O", "U", "CHSHR"} and opening < 0.015:
        raise PreparationError(f"The generated {viseme} pose has no usable mouth opening.")
    mask = make_mouth_mask(base.size, base_face.points, points)
    bounds = mask.getbbox()
    if bounds is None:
        raise PreparationError("Mouth mask is empty.")
    composed = Image.composite(aligned, base, mask)
    original_pixels, result_pixels = np.asarray(base), np.asarray(composed)
    outside = np.asarray(mask) == 0
    changed_outside = int(np.count_nonzero(np.any(original_pixels[outside] != result_pixels[outside], axis=1)))
    if changed_outside:
        raise PreparationError("Compositing changed pixels outside the mouth mask.")
    overlay = aligned.crop(bounds).convert("RGBA")
    overlay.putalpha(mask.crop(bounds))
    return MouthComposition(composed, overlay, mask, bounds, {
        **alignment, "base_opening_ratio": base_opening, "variant_opening_ratio": opening,
        "changed_pixels_outside_mask": changed_outside,
        "mask_pixels": int(np.count_nonzero(np.asarray(mask))),
    })


def draw_regions(image: Image.Image, face: FaceGeometry) -> Image.Image:
    from mediapipe.tasks.python.vision import FaceLandmarksConnections

    drawing = image.copy().convert("RGB")
    draw = ImageDraw.Draw(drawing)
    for connections, color in [
        (FaceLandmarksConnections.FACE_LANDMARKS_LIPS, "#BBA3F5"),
        (FaceLandmarksConnections.FACE_LANDMARKS_LEFT_EYE, "#397DAC"),
        (FaceLandmarksConnections.FACE_LANDMARKS_RIGHT_EYE, "#397DAC"),
    ]:
        for edge in connections:
            draw.line([tuple(face.points[edge.start, :2]), tuple(face.points[edge.end, :2])], fill=color, width=3)
    for index in STABLE_ANCHORS:
        x, y = face.points[index, :2]
        draw.ellipse((x - 4, y - 4, x + 4, y + 4), fill="#FFB74D")
    return drawing

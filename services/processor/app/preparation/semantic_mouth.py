"""Source-bound semantic oral masks. MediaPipe is never a semantic boundary."""
from io import BytesIO
from typing import Annotated, Literal

import cv2
import numpy as np
from PIL import Image, ImageDraw
from pydantic import BaseModel, ConfigDict, Field, model_validator

from app.gemini_contour_trial import polygon_is_simple
from .faces import PreparationError

LABELS = ('outer_lips', 'aperture', 'upper_teeth', 'lower_teeth', 'tongue')
COLORS = ((90, 220, 140), (250, 200, 60), (40, 180, 250), (220, 120, 235), (255, 130, 70))
Coordinate = Annotated[float, Field(ge=0, le=1000, allow_inf_nan=False)]


class OralRegion(BaseModel):
    model_config = ConfigDict(extra='forbid')
    label: Literal['outer_lips', 'aperture', 'upper_teeth', 'lower_teeth', 'tongue']
    visible: bool
    polygons: list[Annotated[list[tuple[Coordinate, Coordinate]], Field(min_length=3, max_length=96)]]

    @model_validator(mode='after')
    def visibility(self):
        if self.visible != bool(self.polygons):
            raise ValueError('Empty regions must be explicitly absent.')
        return self


class OralContours(BaseModel):
    model_config = ConfigDict(extra='forbid')
    regions: Annotated[list[OralRegion], Field(min_length=5, max_length=5)]
    notes: str

    @model_validator(mode='after')
    def labels(self):
        if {r.label for r in self.regions} != set(LABELS):
            raise ValueError('Every semantic label must occur exactly once.')
        return self


def png(image):
    stream = BytesIO(); image.save(stream, format='PNG'); return stream.getvalue()


def views(image):
    clean = image.resize((image.width * 4, image.height * 4), Image.Resampling.LANCZOS)
    grid = clean.copy(); draw = ImageDraw.Draw(grid)
    for n in range(0, 1001, 100):
        x = min(grid.width - 1, round(n * grid.width / 1000))
        y = min(grid.height - 1, round(n * grid.height / 1000))
        draw.line((x, 0, x, grid.height), fill='#66aabb', width=1)
        draw.line((0, y, grid.width, y), fill='#66aabb', width=1)
        draw.text((x + 2, 2), str(n), fill='white', font_size=18, stroke_width=1, stroke_fill='black')
        draw.text((2, y + 2), str(n), fill='white', font_size=18, stroke_width=1, stroke_fill='black')
    return {'clean.png': png(clean), 'grid.png': png(grid)}


def rasterize(contours, image):
    """Supersampled contour fill; preserve raw proposals and bound edge snapping."""
    masks = {}; refinements = {}
    gray = cv2.cvtColor(np.asarray(image.convert('RGB')), cv2.COLOR_RGB2GRAY)
    gradient = cv2.magnitude(cv2.Sobel(gray, cv2.CV_32F, 1, 0), cv2.Sobel(gray, cv2.CV_32F, 0, 1))
    for region in contours.regions:
        mask = Image.new('L', (image.width * 4, image.height * 4)); draw = ImageDraw.Draw(mask)
        adjustments = []
        for polygon in region.polygons:
            points = np.asarray(polygon) * [image.width / 1000, image.height / 1000]
            if not polygon_is_simple(points):
                raise PreparationError(f'{region.label}: invalid contour topology.')
            # Snap at most one native pixel, only when a distinctly stronger edge
            # exists. This cannot grow an aperture into a hand-chosen opening.
            refined = points.copy()
            for i, point in enumerate(points):
                x, y = np.rint(point).astype(int)
                if not (1 <= x < image.width - 1 and 1 <= y < image.height - 1):
                    continue
                patch = gradient[y-1:y+2, x-1:x+2]
                yy, xx = np.unravel_index(patch.argmax(), patch.shape)
                delta = np.array([xx-1, yy-1], dtype=float)
                if np.linalg.norm(delta) <= 1 and patch[yy, xx] > max(30, gradient[y, x] * 1.5):
                    refined[i] += delta
            if not polygon_is_simple(refined):
                refined = points
            adjustments.extend(np.linalg.norm(refined-points, axis=1).tolist())
            draw.polygon([tuple(v) for v in refined * 4], fill=255)
        masks[region.label] = mask.resize(image.size, Image.Resampling.LANCZOS)
        refinements[region.label] = {'max_native_px': max(adjustments, default=0), 'vertices': len(adjustments)}
    return masks, refinements


def validate_masks(masks, pose):
    core = {name: np.asarray(mask) >= 128 for name, mask in masks.items()}
    issues = []
    if not core['outer_lips'].any(): issues.append('Missing outer lip boundary.')
    allowed = cv2.dilate(core['aperture'].astype(np.uint8), np.ones((3, 3), np.uint8)) > 0
    for label in ('upper_teeth', 'lower_teeth', 'tongue'):
        n = int((core[label] & ~allowed).sum())
        if n > max(3, core[label].sum() * .04): issues.append(f'{label}: {n} pixels outside observed opening.')
    if (core['upper_teeth'] & core['lower_teeth']).sum() > 3:
        issues.append('Upper/lower tooth ownership overlaps.')
    if pose in ('Rest', 'MBP') and any(core[k].any() for k in LABELS[1:]):
        issues.append('Closed pose contains an exposed oral region.')
    if pose not in ('Rest', 'MBP') and not core['aperture'].any():
        issues.append('Open pose has no observed opening.')
    if (core['aperture'] & ~cv2.dilate(core['outer_lips'].astype(np.uint8), np.ones((3,3),np.uint8)).astype(bool)).sum() > 3:
        issues.append('Opening exceeds outer lip boundary.')
    return {'valid': not issues, 'issues': issues, 'pixels': {k: int(v.sum()) for k, v in core.items()}}


def overlay(image, masks):
    color = np.asarray(image.convert('RGB')).astype(float)
    for name, rgb in zip(LABELS, COLORS):
        # Draw boundaries for nested outer/aperture regions, filled dental masks.
        mask = np.asarray(masks[name])
        if name in ('outer_lips', 'aperture'):
            mask = cv2.morphologyEx(mask, cv2.MORPH_GRADIENT, np.ones((3,3),np.uint8))
        alpha = mask[..., None] / 255 * .55
        color = color * (1-alpha) + np.asarray(rgb) * alpha
    return Image.fromarray(np.rint(color).astype(np.uint8))

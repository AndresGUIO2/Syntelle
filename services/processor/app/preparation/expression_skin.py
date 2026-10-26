"""Deform source skin without blending a second portrait's appearance."""
import cv2
import numpy as np

from app.preparation.faces import PreparationError


def source_skin_frame(foreground, field, crop, amount):
    x, y, width, height = (crop[key] for key in ('x', 'y', 'width', 'height'))
    if (foreground.ndim != 3 or foreground.shape[2] != 4 or
            field.ndim != 3 or field.shape[2] < 2 or not np.isfinite(field).all() or
            not np.isfinite(amount) or not 0 <= amount <= 1 or
            min(x, y) < 0 or min(width, height) <= 0 or
            x + width > foreground.shape[1] or y + height > foreground.shape[0]):
        raise PreparationError('Source skin requires finite bound geometry and RGBA pixels.')
    if amount == 0:
        return foreground[y:y+height, x:x+width].copy()
    yy, xx = np.indices((height, width), np.float32)
    sx, sy = xx.copy(), yy.copy()
    for _ in range(5):
        delta = cv2.remap(field[..., :2].astype(np.float32),
                          ((sx+.5)*field.shape[1]/width-.5).astype(np.float32),
                          ((sy+.5)*field.shape[0]/height-.5).astype(np.float32),
                          cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT)
        sx, sy = xx-delta[..., 0]*amount, yy-delta[..., 1]*amount
    return cv2.remap(foreground, (sx+x).astype(np.float32), (sy+y).astype(np.float32), cv2.INTER_LINEAR,
                     borderMode=cv2.BORDER_REFLECT_101)

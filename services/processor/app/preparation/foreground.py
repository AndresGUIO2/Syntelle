"""Automatic semantic trimaps and source-resolution foreground matting.

Semantic confidence supplies seeds, never fractional pixel opacity. No character
names, manually drawn polygons or background color keys belong in this module.
"""
from hashlib import sha256
from pathlib import Path

import cv2
import numpy as np
from pymatting import estimate_alpha_cf, estimate_foreground_ml

from .faces import PreparationError

VERSION = 'semantic-native-hair-matting/5'
SETTINGS = {
    'pymatting': '1.1.16', 'foreground_confidence': .98,
    'background_confidence': .10, 'foreground_erosion_fraction': .004,
    'unknown_expansion_fraction': .018, 'epsilon': 1e-7,
    'solver_max_iterations': 2000, 'solver_relative_tolerance': 1e-7,
    'foreground_estimation': 'multilevel', 'source_resolution': True,
    'interior_minimum_confidence': .2, 'interior_margin_fraction': .012,
}


def cache_settings():
    from .hair_matting import source_binding
    return {**SETTINGS, 'version': VERSION,
            'hair_refinement':source_binding(),
            'implementation_sha256': sha256(Path(__file__).read_bytes()).hexdigest()}


def semantic_trimap(probability, nose=None, hair=None):
    """Retain a wide uncertain band, including low-confidence fine strands."""
    p = np.asarray(probability, dtype=np.float64)
    if p.ndim != 2 or not np.isfinite(p).all() or p.min() < 0 or p.max() > 1:
        raise PreparationError('Invalid semantic foreground confidence.')
    scale = min(p.shape)
    def kernel(radius):
        return cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (2*radius+1,)*2)
    seed = (p >= .5).astype(np.uint8)
    if nose is not None:
        _, labels = cv2.connectedComponents(seed)
        x, y = np.rint(nose).astype(int)
        owner = labels[np.clip(y, 0, p.shape[0]-1), np.clip(x, 0, p.shape[1]-1)]
        if owner == 0:
            raise PreparationError('Subject segmentation does not own the detected face.')
        seed = (labels == owner).astype(np.uint8)
    expansion = max(3, round(scale*SETTINGS['unknown_expansion_fraction']))
    support = cv2.dilate(seed, kernel(expansion)) > 0
    # Only definitely interior semantic pixels may be locked opaque. The broad
    # support is NOT applied to the solved alpha afterward: wisps remain soft.
    erosion = max(1, round(scale*SETTINGS['foreground_erosion_fraction']))
    fg = cv2.erode(((p >= SETTINGS['foreground_confidence']) & (seed > 0)).astype(np.uint8), kernel(erosion)) > 0
    # A low-confidence patch enclosed by the silhouette is not a translucent
    # strand. Keep deep interior hair/clothing opaque; estimate alpha only near
    # the actual silhouette and high-confidence background holes.
    flooded = cv2.copyMakeBorder(seed,1,1,1,1,cv2.BORDER_CONSTANT,value=0)
    cv2.floodFill(flooded,None,(0,0),2)
    filled = (flooded[1:-1,1:-1] != 2).astype(np.uint8)
    support = cv2.dilate(filled,kernel(expansion)) > 0
    margin = max(3,round(scale*SETTINGS['interior_margin_fraction']))
    deep = cv2.erode(filled,kernel(margin)) > 0
    fg |= deep & (p > SETTINGS['interior_minimum_confidence'])
    bg = (p <= SETTINGS['background_confidence']) & ~support
    bg |= (filled > 0) & (cv2.erode((p <= .01).astype(np.uint8),kernel(erosion)) > 0)
    if nose is not None:
        # Disconnected furniture is background only beyond its uncertain rim.
        bg |= ~support & (seed == 0)
    if fg.sum() < 20 or bg.sum() < 20:
        raise PreparationError('Segmentation has insufficient confident trimap seeds.')
    trimap = np.full(p.shape, .5)
    trimap[fg] = 1
    trimap[bg] = 0
    if hair is not None:
        # Coarse semantic certainty is not pixel-level certainty: foreground
        # seeds near hair can contain real background between fine strands.
        from .hair_matting import SETTINGS as hair_settings
        distance=cv2.distanceTransform((trimap>0).astype(np.uint8),cv2.DIST_L2,5)
        uncertain=(trimap>0)&hair&(distance<scale*hair_settings['hair_unknown_band_fraction'])
        trimap[uncertain]=.5
    return trimap


def solve_matte(rgb, trimap):
    image = np.asarray(rgb, dtype=np.float64)/255
    trimap = np.asarray(trimap, dtype=np.float64)
    if image.shape[:2] != trimap.shape or image.shape[2:] != (3,):
        raise PreparationError('Matting image and trimap dimensions differ.')
    try:
        alpha = estimate_alpha_cf(image, trimap,
            laplacian_kwargs={'epsilon': SETTINGS['epsilon']},
            cg_kwargs={'maxiter': SETTINGS['solver_max_iterations'],
                       'rtol': SETTINGS['solver_relative_tolerance']})
        foreground = estimate_foreground_ml(image, alpha)
    except (ValueError, RuntimeError, np.linalg.LinAlgError) as error:
        raise PreparationError(f'Foreground matting failed: {type(error).__name__}.') from error
    if not np.isfinite(alpha).all() or not np.isfinite(foreground).all():
        raise PreparationError('Foreground matting returned nonfinite values.')
    color = np.rint(np.clip(foreground, 0, 1)*255).astype(np.uint8)
    # Multilevel foreground estimation also estimates interior color. Preserve
    # actual source texture exactly wherever exported coverage is fully opaque.
    opaque = np.rint(alpha*255) == 255
    color[opaque] = np.asarray(rgb)[opaque]
    return alpha, color


def prepare_matte(rgb, probability, nose=None, classes=None, models=None, evidence=None):
    hair=None
    if classes is not None:
        resized=np.stack([cv2.resize(p,(rgb.shape[1],rgb.shape[0])) for p in classes])
        hair=(resized[1]>.1)&(resized[1]>resized[2:].max(axis=0))
    trimap = semantic_trimap(probability, nose, hair)
    alpha, color = solve_matte(rgb, trimap)
    if hair is not None:
        from .hair_matting import refine_native
        if models is None: raise PreparationError('Native hair model directory is required.')
        active=hair&(trimap>0)&(trimap<1)
        alpha,record=refine_native(rgb,trimap,alpha,active,models)
        foreground=estimate_foreground_ml(np.asarray(rgb,dtype=np.float64)/255,alpha)
        color=np.rint(np.clip(foreground,0,1)*255).astype(np.uint8)
        opaque=np.rint(alpha*255)==255;color[opaque]=rgb[opaque]
        if evidence is not None:
            from PIL import Image
            import json
            Image.fromarray(active.astype(np.uint8)*255).save(evidence/'hair-uncertain.png')
            (evidence/'hair-refinement.json').write_text(json.dumps(record,indent=2))
    return alpha, color, trimap

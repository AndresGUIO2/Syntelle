"""Observed, opaque dental rows from a source-bound expressive reference.

Only segmentation uses colour. Dental RGB is never normalized or repainted.
The curved occlusal seam is measured in the image, independently of portrait IDs.
"""
from __future__ import annotations

import cv2
import numpy as np
from PIL import Image

from .faces import PreparationError
from .oral import cavity_mask

VERSION = 'curved-reference-dentition/1'


def _largest(mask: np.ndarray, connected: bool = False) -> np.ndarray:
    count, labels, statistics, _ = cv2.connectedComponentsWithStats(mask.astype(np.uint8), 8)
    if count < 2:
        return np.zeros_like(mask, dtype=bool)
    largest = 1 + int(np.argmax(statistics[1:, cv2.CC_STAT_AREA]))
    if connected:
        return labels == largest
    # Adjacent crowns can be separated by dark interproximal pixels. Keep all
    # components of meaningful size, rather than discarding individual teeth.
    minimum = max(2, int(statistics[largest, cv2.CC_STAT_AREA] * .008))
    keep = np.flatnonzero(statistics[:, cv2.CC_STAT_AREA] >= minimum)
    return np.isin(labels, keep[keep != 0])


def _curved_seam(gray: np.ndarray, aperture: np.ndarray, enamel: np.ndarray, width: float):
    """Minimum-cost connected dark path, with independent row-support evidence."""
    columns = np.flatnonzero(aperture.any(0))
    if len(columns) < 12:
        raise PreparationError('The reference has no usable observed oral aperture.')
    h, w = gray.shape
    x0, x1 = int(columns[0]), int(columns[-1])
    costs = np.full((h, x1-x0+1), 8., dtype=np.float64)
    contrast = np.zeros_like(costs)
    supported = np.zeros_like(costs, dtype=bool)
    for i, x in enumerate(range(x0, x1+1)):
        ys = np.flatnonzero(aperture[:, x])
        if len(ys) < 5:
            continue
        top, bottom = int(ys[0]), int(ys[-1])
        for y in range(top+2, bottom-1):
            a, b = gray[top:y, x], gray[y+1:bottom+1, x]
            upper, lower = float(a.max()), float(b.max())
            peak = min(upper, lower)
            gap = float(gray[y, x])
            ratio = gap/max(peak, 1.)
            two_rows = enamel[top:y, x].sum() >= 2 and enamel[y+1:bottom+1, x].sum() >= 2
            # A darker slit with enamel on each side is required for confidence.
            # Unsupported edge columns still participate in smooth path fitting.
            costs[y, i] = ratio + (0. if two_rows else .65)
            contrast[y, i] = max(0., 1-ratio)
            supported[y, i] = two_rows and ratio <= .45
    step = max(1, round(width*.012))
    parent = np.zeros(costs.shape, np.int32)
    previous = costs[:, 0].copy()
    for i in range(1, costs.shape[1]):
        current = np.full(h, np.inf)
        for dy in range(-step, step+1):
            src = np.arange(h)-dy
            valid = (src >= 0) & (src < h)
            proposal = np.full(h, np.inf)
            proposal[valid] = previous[src[valid]] + abs(dy)*.075
            choose = proposal < current
            current[choose] = proposal[choose]
            parent[choose, i] = src[choose]
        previous = current + costs[:, i]
    path = np.zeros(costs.shape[1], np.int32)
    path[-1] = int(previous.argmin())
    for i in range(len(path)-1, 0, -1):
        path[i-1] = parent[path[i], i]
    result = np.interp(np.arange(w), np.arange(x0,x1+1), path)
    flags = supported[path, np.arange(len(path))]
    observed = contrast[path, np.arange(len(path))][flags]
    return result, {
        'columns': int(len(path)), 'supported_columns': int(flags.sum()),
        'supported_width_ratio': float(flags.sum()/width),
        'median_local_separation': float(np.median(observed)) if len(observed) else 0.,
        'maximum_step_px': int(step), 'path': [[int(x), int(y)] for x,y in zip(range(x0,x1+1),path)],
    }


def extract_reference_rows(image: Image.Image, points: np.ndarray):
    """Return binary observed masks, source-exact RGBA, and inspectable evidence.

    Thin lower rows need a curved slit rather than a horizontal histogram cut.
    Dark crown pixels are retained inside the measured opening using a learned
    chromaticity boundary between confident enamel and the observed cavity.
    """
    points = np.asarray(points, dtype=float)
    if points.ndim != 2 or len(points) < 468 or points.shape[1] < 2 or not np.isfinite(points).all():
        raise PreparationError('Finite cached facial landmarks are required for reference dentition.')
    raw = np.asarray(image.convert('RGB'))
    width = float(np.linalg.norm(points[61,:2]-points[291,:2]))
    if width < 20:
        raise PreparationError('The observed dental reference is too small.')
    aperture = np.asarray(cavity_mask(image.size, points)) > 0
    gray = cv2.cvtColor(raw, cv2.COLOR_RGB2GRAY).astype(float)
    rgb = raw.astype(float)
    chroma = rgb[...,0]/np.maximum(rgb.sum(2), 1)
    # Initial seeds retain the earlier conservative neutral-enamel rules.
    red, green, blue = rgb.transpose(2,0,1)
    confident = aperture & (green/(red+1) >= .698) & (blue/(red+1) >= .558) & (gray >= 51.25)
    if confident.sum() < 20:
        raise PreparationError('No confident enamel seeds in the expressive reference.')
    cavity = aperture & (gray <= np.percentile(gray[aperture], 25))
    if cavity.sum() < 4:
        raise PreparationError('No independently observed dark cavity for dental ownership.')
    enamel_red = float(np.percentile(chroma[confident],90))
    cavity_red = float(np.median(chroma[cavity]))
    if cavity_red-enamel_red < .07:
        raise PreparationError('Enamel and cavity colours are ambiguous; preserve for review.')
    chroma_boundary = (enamel_red+cavity_red)/2
    # The fitted chromaticity boundary includes shadowed observed enamel; it
    # never changes dental colour or creates pixels outside the oral aperture.
    support = aperture & (chroma < chroma_boundary) & (gray >= 25)
    seam, evidence = _curved_seam(gray, aperture, confident, width)
    yy = np.arange(image.height)[:,None]
    domains = {'upper': aperture & (yy < seam[None]), 'lower': aperture & (yy > seam[None])}
    masks, fragments = {}, {}
    for row, domain in domains.items():
        # A disconnected apparent lower island is often the bottom of an upper
        # lateral crown beside the curved slit. It cannot become a second row.
        mask = support & domain
        if row == 'lower':
            radius = max(1,round(width*.006))
            mask = cv2.morphologyEx(mask.astype(np.uint8),cv2.MORPH_OPEN,
                                   cv2.getStructuringElement(cv2.MORPH_ELLIPSE,(radius*2+1,)*2)) > 0
        mask = _largest(mask, connected=row == 'lower')
        rgba = np.dstack((raw.copy(),mask.astype(np.uint8)*255))
        rgba[~mask,:3] = 0
        masks[row] = Image.fromarray(mask.astype(np.uint8)*255)
        fragments[row] = Image.fromarray(rgba)
    reasons = []
    rows = {}
    for row, mask_image in masks.items():
        mask = np.asarray(mask_image)>0
        columns = int(mask.any(0).sum())
        rows[row] = {'pixels':int(mask.sum()),'observed_columns':columns,'width_ratio':columns/width,
                     'rgb_source_exact': bool(np.array_equal(np.asarray(fragments[row])[mask,:3],raw[mask])),
                     'alpha_opaque':True}
        if mask.sum() < width*width*.001 or columns < width*.3:
            reasons.append(f'{row}: insufficient observed crown coverage.')
    if evidence['supported_width_ratio'] < .3:
        reasons.append('Insufficient independently supported curved dental gap.')
    if evidence['median_local_separation'] < .55:
        reasons.append('Curved dental rows cannot be separated confidently.')
    if np.any((np.asarray(masks['upper'])>0)&(np.asarray(masks['lower'])>0)):
        reasons.append('Dental row ownership overlaps.')
    report = {'version':VERSION,'ready':not reasons,'reasons':reasons,'rows':rows,'seam':evidence,
              'chromaticity':{'enamel_red_fraction':enamel_red,'cavity_red_fraction':cavity_red,
                              'boundary':chroma_boundary},
              'manual_masks':False,'paid_calls':0,'recolouring':False,'tooth_warping':False,
              'limitation':'Observed 2D crown material only; hidden or lateral crowns are not reconstructed.'}
    return masks, fragments, report


def mask_overlay(image: Image.Image, masks: dict[str, Image.Image]) -> Image.Image:
    raw = np.asarray(image.convert('RGB')).astype(float)
    for row,colour in [('upper',(40,190,245)),('lower',(225,115,225))]:
        alpha = (np.asarray(masks[row])/255*.45)[...,None]
        raw = raw*(1-alpha)+np.asarray(colour)*alpha
    return Image.fromarray(np.rint(raw).astype(np.uint8))

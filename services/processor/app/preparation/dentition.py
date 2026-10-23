"""Deterministic, recorded dental-plate matting and one-time frontal calibration."""
from hashlib import sha256
import cv2
import numpy as np
from PIL import Image
from .faces import PreparationError


def prepare_rows(raw, config, size, annotation_offset=(0, 0)):
    """Calibrate once; the resulting canonical rows never deform at playback."""
    rows = {}
    evidence = {}
    for name in ('upper', 'lower'):
        settings = config['rows'][name]
        crop = raw.convert('RGB').crop(tuple(settings['source_box']))
        rgb = np.asarray(crop, dtype=np.float32)
        channel = rgb.max(axis=2)
        alpha = np.clip((channel-config['matte']['black_level']) / config['matte']['ramp'], 0, 1)
        support = channel > config['matte']['edge_threshold']
        columns = np.flatnonzero(support.any(axis=0))
        if len(columns) < crop.width // 2:
            raise PreparationError('Dental row does not cover the reviewed plate region.')
        edge = np.zeros(crop.width, dtype=np.float32)
        for x in columns:
            pixels = np.flatnonzero(support[:, x])
            edge[x] = pixels[-1] if name == 'upper' else pixels[0]
        edge = np.interp(np.arange(crop.width), columns, edge[columns]).astype(np.float32)
        edge = cv2.GaussianBlur(edge[None], (0, 0), settings['edge_smoothing_source_px'])[0]
        scale = settings['width'] / crop.width
        center = settings['center_x'] + annotation_offset[0]
        edge_y = settings['cutting_edge_y'] + annotation_offset[1]
        y, x = np.mgrid[:size[1], :size[0]].astype(np.float32)
        source_x = (x-center) / scale + (crop.width-1)/2
        relative = (x-center)/(settings['width']/2)
        target_edge = edge_y + settings['edge_curve_px'] * relative**2
        source_edge = np.interp(source_x, np.arange(crop.width), edge).astype(np.float32)
        source_y = (y-target_edge) / scale + source_edge
        # Separate JPEG's black matte, then resample premultiplied colors to avoid fringes.
        rgb *= settings['exposure_gain']
        premultiplied = rgb * alpha[..., None]
        mapped_rgb = cv2.remap(premultiplied, source_x, source_y, cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT)
        mapped_alpha = cv2.remap(alpha, source_x, source_y, cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT)
        straight = np.divide(mapped_rgb, mapped_alpha[..., None], out=np.zeros_like(mapped_rgb), where=mapped_alpha[..., None] > 0)
        rgba = np.dstack((np.clip(straight, 0, 255), np.rint(mapped_alpha*255))).astype(np.uint8)
        rows[name] = Image.fromarray(rgba)
        evidence[name] = {'source_box': settings['source_box'], 'uniform_scale': scale,
                          'source_edge_profile': edge.round(4).tolist(), 'placement': settings,
                          'rgba_sha256': sha256(rgba.tobytes()).hexdigest()}
    return rows['upper'], rows['lower'], evidence

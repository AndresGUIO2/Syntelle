"""Prepare local aperture measurements without changing enamel or lip coverage."""
from hashlib import sha256

import numpy as np
from PIL import Image

from .faces import PreparationError

VERSION = 'visible-aperture-lighting/1'


def prepare_lighting(package, directory, reference_gap):
    """Mask B stores measured column opening/reference opening, in [0,255].

    R remains the edit mask and G remains the signed aperture distance. Keeping
    the measurement in their UV domain makes it follow the same articulation.
    """
    if not np.isfinite(reference_gap) or reference_gap <= 1:
        raise PreparationError('A measurable registered reference opening is required.')
    measurements = {}
    for name, pose in package['poses'].items():
        if not pose.get('aperture_sdf'):
            raise PreparationError('Visible aperture lighting requires semantic aperture masks.')
        path = directory/pose['mask']
        pixels = np.asarray(Image.open(path).convert('RGBA')).copy()
        opening = pixels[:, :, 1] > 128
        # Count the actual exposed span, including subpixel coverage at its edge.
        coverage = np.clip((pixels[:, :, 1].astype(float)-128)/8+.5, 0, 1)
        gaps = coverage.sum(axis=0)
        pixels[:, :, 2] = np.rint(np.clip(gaps/reference_gap, 0, 1)*255).astype(np.uint8)[None, :]
        Image.fromarray(pixels).save(path, lossless=True)
        measurements[name] = {'maximum_gap_px': float(gaps.max()),
                              'open_columns': int(opening.any(axis=0).sum()),
                              'mask_sha256': sha256(path.read_bytes()).hexdigest()}
    package.setdefault('oral', {})['visible_aperture'] = {
        'version': VERSION, 'reference_gap_px': float(reference_gap),
        'minimum_irradiance': .28, 'full_light_ratio': .65,
        'contact_width_px': float(reference_gap)*.12,
        'measurement': 'mask-b-column-coverage/1',
    }
    return measurements

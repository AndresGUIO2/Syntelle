"""Source-bound motion inspection and continuous rigid dental registration."""
from hashlib import sha256
from pathlib import Path
import json

import numpy as np
import cv2
from PIL import Image

from .archive import encoded
from .faces import PreparationError
from .projection_rig import validate


def inspect_ranges(motion, directory):
    p = motion['projection']; g = p['geometry']
    raw = (directory/g['file']).read_bytes()
    if sha256(raw).hexdigest() != g['sha256']:
        raise PreparationError('Projection geometry checksum mismatch.')
    vertices = np.frombuffer(raw, '<f4', count=g['vertices']*5).reshape(-1, 5)
    indices = np.frombuffer(raw, '<u2', offset=g['vertices']*20).reshape(-1, 3)
    rows = []
    for axis, values in enumerate(((3, 4.5, 6, 8, 10, 12), (4, 5, 6), (1.2, 2))):
        for value in values:
            bounds = [0., 0., 0.]; bounds[axis] = value
            report = validate(vertices, indices, p['camera'], p['pivot'], bounds,
                              motion['anatomy']['face_height'])
            rows.append({'axis': ('yaw', 'pitch', 'roll')[axis], 'degrees': value,
                         'geometry': report, 'visual_acceptance': False})
    return {'version': 'parallax-range-inspection/1', 'source_sha256': motion['original_sha256'],
            'attempts': rows, 'appearance_qualified': False}


def oral_transition(expressions, oral, source):
    reference = expressions.get('reference_laughter')
    if reference is None or reference['source_sha256'] != source or oral['source_sha256'] != source:
        raise PreparationError('Oral transition requires matching source-bound resources.')
    # Existing canonical placements are the evidence for teeth hidden by closed
    # lips. Landmarks alone cannot reconstruct that hidden dental geometry.
    rest = oral['poses']['Rest']
    upper = np.asarray(rest.get('upper_translation', oral.get('upper_translation')), float)
    lower = np.asarray(rest['lower_translation'], float)
    if upper.shape != (2,) or lower.shape != (2,) or not np.isfinite([upper, lower]).all():
        raise PreparationError('Missing canonical resting dental registration.')
    weights = np.linspace(0, 1, 25)
    # Source-derived offsets join the exact reference placement at full laughter.
    # Each arch remains rigid; only lip geometry reveals its opaque pixels.
    transition = dict(version='oral-transition/1', source_sha256=source,
                      oral_build_id=oral['build_id'], upper=((1-weights[:, None])*upper).tolist(),
                      lower=((1-weights[:, None])*lower).tolist(),
                      calibration='existing-canonical-registration', local_contact=True,
                      hidden_dentition_reconstructed=False)
    reference['oral']['transition'] = transition
    return transition


def rebind(value, before, after):
    if isinstance(value, dict):
        for key, child in value.items():
            if key == 'motion_build_id':
                if child != before:
                    raise PreparationError('Nested resource has a different motion binding.')
                value[key] = after
            else:
                rebind(child, before, after)
    elif isinstance(value, list):
        for child in value:
            rebind(child, before, after)


def validate_dental_transition(expressions, directory, oral):
    reference=expressions['reference_laughter'];t=reference['oral']['transition']
    upper=np.asarray(Image.open(directory/reference['oral']['observed_upper']['file']).convert('RGBA'))[:,:,3]
    lower=np.asarray(Image.open(directory/reference['oral']['observed_lower']['file']).convert('RGBA'))[:,:,3]
    h,w=upper.shape;rows=[]
    for i,(up,lo) in enumerate(zip(t['upper'],t['lower'])):
        aperture=np.zeros((h,w),np.uint8)
        cv2.fillPoly(aperture,[np.rint(reference['oral']['inner'][i]).astype(np.int32)],255)
        a=cv2.warpAffine(upper,np.array([[1,0,up[0]],[0,1,up[1]]],float),(w,h))>127
        b=cv2.warpAffine(lower,np.array([[1,0,lo[0]],[0,1,lo[1]]],float),(w,h))>127
        overlap=int((a&b&(aperture>0)).sum())
        if overlap:raise PreparationError('Dental rows overlap inside the recovering aperture; inspect without installing.')
        rows.append(dict(step=i,visible_upper_pixels=int((a&(aperture>0)).sum()),visible_lower_pixels=int((b&(aperture>0)).sum()),overlap_pixels=overlap))
    return dict(version='oral-transition-validation/1',states=rows,valid=True,canonical_materials_unchanged=True)


def write_json(path: Path, value):
    path.write_bytes(encoded(value))

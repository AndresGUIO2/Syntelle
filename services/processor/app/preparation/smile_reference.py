"""Register a received closed smile and prepare a geometry-aligned facial material.

The reference is normalized into the neutral facial domain before interpolation.
This prevents dissolving between two lip edges. Speech keeps its independent oral
materials: this first companion is a closed-mouth expression, not smile visemes.
"""
import cv2
import numpy as np
from PIL import Image, ImageDraw
from scipy.interpolate import LinearNDInterpolator

from .eyes import align_eyes
from .expressions import field_from_image, validate, render
from .faces import PreparationError

VERSION = 'registered-closed-smile/1'


def prepare(original, generated, points, generated_points, previous, previous_field, output):
    aligned, target, registration = align_eyes(original, generated, points, generated_points,
                                              original.width/generated.width)
    crop = previous['crop']
    x0, y0, width, height = [crop[k] for k in ('x', 'y', 'width', 'height')]
    field = field_from_image(previous_field, previous['units_per_byte'])
    h, w = field.shape[:2]
    yy, xx = np.indices((h, w), float)
    xy = np.stack([x0+(xx+.5)*width/w, y0+(yy+.5)*height/h], -1)
    mouth_width = float(np.linalg.norm(points[61, :2]-points[291, :2]))
    center = (points[61, :2]+points[291, :2])/2
    # A common lower-face domain includes complete lips and cheek attachments.
    # No central lip pin or manually drawn character mask is used.
    radius = ((xy[..., 0]-center[0])/(mouth_width*.98))**2+((xy[..., 1]-center[1])/(mouth_width*.78))**2
    mask = np.clip((1-radius)/.48, 0, 1)
    mask = mask*mask*(3-2*mask)
    nose = np.clip((xy[..., 1]-points[2, 1]+mouth_width*.03)/(mouth_width*.18), 0, 1)
    mask *= nose*nose*(3-2*nose)
    displacement = LinearNDInterpolator(points[:468, :2], target[:468, :2]-points[:468, :2], fill_value=0)(xy)
    displacement *= mask[..., None]
    field[..., :2] = displacement
    field[[0, -1], :, :2] = 0
    field[:, [0, -1], :2] = 0
    for reduction in (1., .85, .7225, .614125):
        trial = field.copy(); trial[..., :2] *= reduction
        scale = max(.001, float(abs(trial).max())/120)
        packed = np.uint8(np.clip(np.rint(trial/scale)+128, 0, 255))
        decoded = (packed.astype(float)-128)*scale
        validation = validate(decoded, crop)
        if validation['valid']:
            break
    if not validation['valid']:
        raise PreparationError('Registered smile failed the shared non-inversion/inverse gate.')
    y, x = np.indices((height, width), np.float32)
    fx = (x+.5)*w/width-.5; fy = (y+.5)*h/height-.5
    # The raw observed mapping, rather than its safety-reduced rendering field,
    # locates the received lips in neutral material coordinates.
    delta = cv2.remap(displacement.astype(np.float32), fx, fy, cv2.INTER_LINEAR)
    color = cv2.remap(np.asarray(aligned), x+x0+delta[..., 0], y+y0+delta[..., 1],
                      cv2.INTER_LINEAR, borderMode=cv2.BORDER_REFLECT_101)
    alpha = cv2.remap(mask.astype(np.float32), fx, fy, cv2.INTER_LINEAR)
    rgba = np.dstack([color, np.uint8(np.rint(alpha*255))])
    Image.fromarray(rgba).save(output/'smile-material.png')
    Image.fromarray(rgba).save(output/'smile-material.webp', lossless=True, exact=True, method=6)
    Image.fromarray(packed).save(output/'field.png')
    np.save(output/'field.npy', decoded)
    aligned.save(output/'aligned-reference.png')
    neutral = np.asarray(original.convert('RGB')).copy()
    normalized = neutral.copy()
    patch = normalized[y0:y0+height, x0:x0+width]
    patch[:] = np.rint(patch*(1-alpha[..., None])+color*alpha[..., None]).astype(np.uint8)
    panels = []
    for amount in (0, .5, 1):
        blend = np.rint(neutral*(1-amount)+normalized*amount).astype(np.uint8)
        result = render(Image.fromarray(blend), decoded, crop, amount, 0)
        result.save(output/f'preview-{amount:g}.png')
        panels.append(result.crop((x0, y0, x0+width, y0+height)))
    sheet = Image.new('RGB', (600*3, round(height*600/width)+24), '#222126')
    draw = ImageDraw.Draw(sheet)
    for i, panel in enumerate(panels):
        draw.text((i*600+8, 5), ('Original', '50% registered smile', 'Registered smile')[i], fill='white')
        sheet.paste(panel.resize((600, sheet.height-24)), (i*600, 24))
    sheet.save(output/'inspection.png')
    return {**previous, 'version': 'local-expressions/2', 'units_per_byte': scale,
            'validation': validation, 'smile': {'version': VERSION, 'closed_mouth_only': True,
                'registration': registration, 'amplitude_reduction': reduction,
                'material': {'file': 'smile-material.png'}, 'reference_landmarks': target.tolist(),
                'generated_landmarks': generated_points.tolist(),
                'manual_masks': False, 'requests_at_runtime': 0}}

"""Small source-preserving expression fields derived from cached face anatomy."""
from itertools import product

import cv2
import numpy as np
from PIL import Image, ImageDraw

from .eyes import EYE_RINGS
from .faces import PreparationError

VERSION = 'local-expressions/1'
SETTINGS = {'version': VERSION, 'maximum_field_side': 512, 'smile_corner_lift': .055,
            'smile_corner_width': .03, 'brow_lift_width_fraction': .12,
            'minimum_jacobian': .45, 'maximum_gradient': 1.5,
            'maximum_inverse_residual_px': .2, 'inverse_iterations': 5}
BROWS = ((70, 63, 105, 66, 107, 55, 65, 52, 53, 46),
         (336, 296, 334, 293, 300, 276, 283, 282, 295, 285))


def bump(x, y, center, radius):
    r = ((x-center[0])/radius[0])**2+((y-center[1])/radius[1])**2
    return np.maximum(0, 1-r)**3


def field_from_image(image, scale):
    values = np.asarray(image.convert('RGB')).astype(float)
    return (values-128)*scale


def validate(field, crop, *, inverse_iterations=None, brow_attenuation=0.):
    """Validate the actual quantized, bilinearly sampled field's local Jacobian."""
    iterations = SETTINGS['inverse_iterations'] if inverse_iterations is None else inverse_iterations
    height, width = field.shape[:2]
    dx = crop['width']/width; dy = crop['height']/height
    minimum = 1.
    maximum_gradient = 0.
    for smile, brows in product(np.linspace(0, 1, 5), repeat=2):
        brows *= 1-brow_attenuation*smile
        u = field[..., 0]*smile
        v = field[..., 1]*smile+field[..., 2]*brows
        ux = np.diff(u, axis=1)/dx; vx = np.diff(v, axis=1)/dx
        uy = np.diff(u, axis=0)/dy; vy = np.diff(v, axis=0)/dy
        # Bilinear derivatives are affine along each cell edge. Check all corners.
        for vertical, horizontal in product((0, 1), repeat=2):
            a = ux[vertical:height-1+vertical]; c = vx[vertical:height-1+vertical]
            b = uy[:, horizontal:width-1+horizontal]; d = vy[:, horizontal:width-1+horizontal]
            minimum = min(minimum, float(((1+a)*(1+d)-b*c).min()))
            maximum_gradient = max(maximum_gradient, float(np.maximum(abs(a)+abs(b), abs(c)+abs(d)).max()))
    gy, gx = np.indices((height, width), np.float32)
    residual = 0.
    for smile, brows in ((1., 0.), (0., 1.), (1., 1.)):
        brows *= 1-brow_attenuation*smile
        u = field[..., 0]*smile; v = field[..., 1]*smile+field[..., 2]*brows
        target_x = gx+u/dx; target_y = gy+v/dy
        source_x, source_y = target_x.copy(), target_y.copy()
        for _ in range(iterations):
            sampled = cv2.remap(field.astype(np.float32), source_x.astype(np.float32), source_y.astype(np.float32),
                                cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT)
            source_x = target_x-sampled[..., 0]*smile/dx
            source_y = target_y-(sampled[..., 1]*smile+sampled[..., 2]*brows)/dy
        residual = max(residual, float(np.hypot((source_x-gx)*dx, (source_y-gy)*dy).max()))
    return {'valid': minimum >= SETTINGS['minimum_jacobian'] and maximum_gradient < SETTINGS['maximum_gradient'] and residual <= SETTINGS['maximum_inverse_residual_px'],
            'minimum_jacobian': minimum, 'maximum_gradient': maximum_gradient,
            'maximum_inverse_residual_px': residual,
            'configurations': 25, 'neutral_exact': True}


def render(image, field, crop, smile, brows):
    rgb = np.asarray(image.convert('RGB'))
    output = rgb.copy()
    x0, y0, width, height = [crop[k] for k in ('x', 'y', 'width', 'height')]
    y, x = np.indices((height, width), np.float32)
    source_x, source_y = x.copy(), y.copy()
    for _ in range(SETTINGS['inverse_iterations']):
        fx = (source_x+.5)*field.shape[1]/width-.5
        fy = (source_y+.5)*field.shape[0]/height-.5
        delta = cv2.remap(field.astype(np.float32), fx, fy, cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT)
        source_x = x-delta[..., 0]*smile
        source_y = y-delta[..., 1]*smile-delta[..., 2]*brows
    patch = cv2.remap(rgb, source_x+x0, source_y+y0, cv2.INTER_LINEAR, borderMode=cv2.BORDER_REFLECT_101)
    output[y0:y0+height, x0:x0+width] = patch
    return Image.fromarray(output)


def prepare(image, landmarks, package, oral, upper, lower, output):
    points = np.asarray(landmarks, float)[:, :2]
    if points.shape != (478, 2) or not np.isfinite(points).all():
        raise PreparationError('Expressions require finite refined face landmarks.')
    face_width = float(np.linalg.norm(points[234]-points[454]))
    mouth_width = float(np.linalg.norm(points[61]-points[291]))
    if mouth_width < 16:
        raise PreparationError('The mouth is too small for controlled expression preparation.')
    selected = points[np.r_[list(range(468))]]
    low = np.maximum(0, np.floor(selected.min(0)-face_width*.06)).astype(int)
    high = np.minimum(image.size, np.ceil(selected.max(0)+face_width*.06)).astype(int)
    crop = dict(zip(('x', 'y', 'width', 'height'), map(int, [*low, *(high-low)])))
    side = min(SETTINGS['maximum_field_side'], max(crop['width'], crop['height']))
    w = max(2, round(crop['width']*side/max(crop['width'], crop['height'])))
    h = max(2, round(crop['height']*side/max(crop['width'], crop['height'])))
    yy, xx = np.indices((h, w), float)
    x = low[0]+(xx+.5)*crop['width']/w
    y = low[1]+(yy+.5)*crop['height']/h
    field = np.zeros((h, w, 3), float)
    center = (points[61]+points[291])/2
    # Pin the shared dental material through every recorded rigid row translation.
    guard = np.zeros((image.height, image.width), np.uint8)
    oral_crop = oral['crop']
    for pose in oral['poses'].values():
        for row, material in (('upper', upper), ('lower', lower)):
            alpha = np.asarray(material.convert('RGBA'))[..., 3]
            shift = pose.get(row+'_translation', [0, 0])
            matrix = np.float32([[1, 0, oral_crop['x']+shift[0]], [0, 1, oral_crop['y']+shift[1]]])
            guard = np.maximum(guard, cv2.warpAffine(alpha, matrix, image.size))
    # Eyes and lids remain under their independent blink/gaze compositor.
    for ring in EYE_RINGS:
        boundary = np.rint(points[list(ring)]).astype(np.int32)
        eye_mask = np.zeros_like(guard); cv2.fillPoly(eye_mask, [boundary], 255)
        radius = max(1, round(np.ptp(points[list(ring), 0])*.07))
        eye_mask = cv2.dilate(eye_mask, cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (radius*2+1,)*2))
        guard = np.maximum(guard, eye_mask)
    # Keep the tongue, central cavity, incisal contact and closed lip seam stable.
    cv2.ellipse(guard, tuple(np.rint(center).astype(int)),
                (round(mouth_width*.33), round(mouth_width*.35)), 0, 0, 360, 255, -1)
    # Account for both interpolation footprints so downsampling cannot leak a
    # subpixel deformation into protected pupils, enamel or the contact seam.
    padding = int(np.ceil(max(crop['width']/w, crop['height']/h)*2))+1
    padded = cv2.dilate(np.uint8(guard > 0), cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (padding*2+1,)*2))
    distance = cv2.distanceTransform(np.uint8(padded == 0), cv2.DIST_L2, 5)
    protections = []
    for fraction in (.12, .3):
        protect = np.clip(distance/max(2, mouth_width*fraction), 0, 1)
        protect = protect*protect*(3-2*protect)
        protections.append(cv2.remap(protect, x.astype(np.float32), y.astype(np.float32), cv2.INTER_LINEAR))
    for corner in (points[61], points[291]):
        side_sign = -1 if corner[0] < center[0] else 1
        weight = bump(x, y, corner, [mouth_width*.48, mouth_width*.52])
        cheek = bump(x, y, corner+[side_sign*mouth_width*.23, -mouth_width*.3], [mouth_width*.55, mouth_width*.65])
        field[..., 0] += side_sign*(weight+.22*cheek)*mouth_width*SETTINGS['smile_corner_width']
        field[..., 1] -= (weight+.3*cheek)*mouth_width*SETTINGS['smile_corner_lift']
    for brow in BROWS:
        p = points[list(brow)]
        width = float(np.ptp(p[:, 0]))
        # A broad plateau carries the complete brow; compact support leaves hair fixed.
        weight = np.zeros((h, w))
        for anchor in p:
            weight = np.maximum(weight, bump(x, y, anchor, [width*.55, width*.7]))
        field[..., 2] -= weight*width*SETTINGS['brow_lift_width_fraction']
    field[..., :2] *= protections[0][..., None]
    field[..., 2] *= protections[1]
    edge = np.minimum.reduce([x-low[0], high[0]-x, y-low[1], high[1]-y])
    taper = np.clip(edge/(face_width*.1), 0, 1)
    field *= (taper*taper*(3-2*taper))[..., None]
    # Zero the outer field ring so CLAMP_TO_EDGE cannot move material outside its domain.
    field[[0, -1]] = 0; field[:, [0, -1]] = 0
    reductions = []
    for channels in ((0, 1), (2,)):
        reduction = 1.
        for _ in range(12):
            trial = np.zeros_like(field)
            trial[..., list(channels)] = field[..., list(channels)]*reduction
            if validate(trial, crop)['valid']:
                break
            reduction *= .85
        field[..., list(channels)] *= reduction
        reductions.append(reduction)
    reduction = 1.
    for _ in range(12):
        scaled = field*reduction
        scale = max(.001, float(abs(scaled).max())/120)
        encoded = np.uint8(np.clip(np.rint(scaled/scale)+128, 0, 255))
        decoded = (encoded.astype(float)-128)*scale
        validation = validate(decoded, crop)
        if validation['valid']:
            break
        reduction *= .85
    if not validation['valid']:
        raise PreparationError('Expression deformation cannot satisfy the shared topology gate.')
    Image.fromarray(encoded).save(output/'field.png')
    Image.fromarray(guard).save(output/'protected.png')
    np.save(output/'field.npy', decoded)
    # CPU previews share the inverse-field convention used by the browser shader.
    previews = [image.convert('RGB'), render(image, decoded, crop, 1, 0),
                render(image, decoded, crop, 0, 1), render(image, decoded, crop, 1, 1)]
    face = (crop['x'], crop['y'], crop['x']+crop['width'], crop['y']+crop['height'])
    panel_width = min(600, crop['width']); panel_height = round(panel_width*crop['height']/crop['width'])
    sheet = Image.new('RGB', (panel_width*2, (panel_height+24)*2), '#242229')
    draw = ImageDraw.Draw(sheet)
    for i, preview in enumerate(previews):
        px, py = i%2*panel_width, i//2*(panel_height+24)
        draw.text((px+8, py+5), ('Neutral', 'Soft smile', 'Raised eyebrows', 'Combined')[i], fill='white')
        sheet.paste(preview.crop(face).resize((panel_width, panel_height)), (px, py+24))
    sheet.save(output/'inspection.png')
    return {'version': VERSION, 'source_sha256': package['original_sha256'],
            'motion_build_id': package['build_id'], 'oral_build_id': oral['build_id'],
            'crop': crop, 'field_size': [w, h], 'units_per_byte': scale,
            'amplitude_reduction': {'smile': reductions[0]*reduction, 'brows': reductions[1]*reduction},
            'inverse_iterations': SETTINGS['inverse_iterations'],
            'validation': validation, 'new_provider_requests': 0}

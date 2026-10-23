"""Source-supported iris translation; no hidden iris synthesis or manual masks."""
from hashlib import sha256
import json

import cv2
import numpy as np
from PIL import Image, ImageDraw
from scipy.sparse import coo_matrix
from scipy.sparse.linalg import spsolve

from .eyes import EYE_RINGS
from .faces import PreparationError

VERSION = 'observed-iris/1'
COMPOSITOR_VERSION = 'supported-iris-over/2'
SETTINGS = {'version': VERSION, 'horizontal_fraction': .05, 'vertical_fraction': .02,
            'aperture_inset_px': .6, 'protected_boundary_fraction': .02, 'edge_search_fraction': .12}


def harmonic_fill(rgb, domain, unknown):
    """Neumann outside the aperture, observed sclera as Dirichlet boundary."""
    coords = np.argwhere(unknown)
    if not len(coords):
        return rgb.copy()
    index = np.full(domain.shape, -1, np.int32)
    index[unknown] = np.arange(len(coords))
    rows, cols, values = [], [], []
    rhs = np.zeros((len(coords), 3))
    for i, (y, x) in enumerate(coords):
        degree = 0
        for dy, dx in ((-1, 0), (1, 0), (0, -1), (0, 1)):
            yy, xx = y+dy, x+dx
            if not (0 <= yy < domain.shape[0] and 0 <= xx < domain.shape[1] and domain[yy, xx]):
                continue
            degree += 1
            j = index[yy, xx]
            if j >= 0:
                rows.append(i); cols.append(j); values.append(-1.)
            else:
                rhs[i] += rgb[yy, xx]
        rows.append(i); cols.append(i); values.append(float(degree))
    if not np.any(domain & ~unknown):
        raise PreparationError('No observed sclera supports iris removal.')
    matrix = coo_matrix((values, (rows, cols)), shape=(len(coords), len(coords))).tocsr()
    result = rgb.copy().astype(float)
    result[unknown] = spsolve(matrix, rhs)
    if not np.isfinite(result).all():
        raise PreparationError('Sclera reconstruction is not constrained.')
    return np.uint8(np.clip(np.rint(result), 0, 255))


def translate(array, dx, dy):
    return cv2.warpAffine(array, np.float32([[1, 0, dx], [0, 1, dy]]),
                          (array.shape[1], array.shape[0]), flags=cv2.INTER_LINEAR,
                          borderMode=cv2.BORDER_CONSTANT)


def render(rgb, plate, masks, offsets, centers):
    result = rgb.astype(float).copy()
    yy, xx = np.indices(rgb.shape[:2])
    for i, (dx, dy) in enumerate(offsets):
        if dx == 0 and dy == 0:
            continue
        side = (xx < (centers[0][0]+centers[1][0])/2) == (i == 0)
        aperture, iris = masks[..., 0]/255., masks[..., 1]/255.
        aperture = aperture*side
        clean = plate.astype(float)
        known = iris*(masks[..., 2]/255.)
        alpha = translate(known, dx, dy)
        covered = (rgb.astype(float)-plate*(1-iris[..., None]))*(masks[..., 2:3]/255.)
        shifted = translate(covered, dx, dy)
        expected = translate(iris, dx, dy)
        missing = np.clip(expected-alpha, 0, 1)
        color = shifted+rgb*missing[..., None]+clean*np.maximum(0, 1-alpha-missing)[..., None]
        result = result*(1-aperture[..., None])+color*aperture[..., None]
    return np.uint8(np.clip(np.rint(result), 0, 255))


def prepare(image, landmarks, package, output):
    crop = package['eyes']['crop']
    x, y, w, h = (crop[k] for k in ('x', 'y', 'width', 'height'))
    rgb = np.asarray(image.convert('RGB').crop((x, y, x+w, y+h)))
    points = np.asarray(landmarks, float)[:, :2]-[x, y]
    if len(points) != 478:
        raise PreparationError('Refined MediaPipe iris landmarks are required.')
    yy, xx = np.indices((h, w), dtype=float)
    masks = np.zeros((h, w, 4), np.uint8)
    plate = rgb.copy()
    eyes = []
    # Rings are matched by horizontal proximity, independent of character or handedness.
    iris_groups = (points[469:473], points[474:478])
    for ring in EYE_RINGS:
        boundary = points[list(ring)]
        center_hint = boundary.mean(0)
        iris_points = min(iris_groups, key=lambda p: np.linalg.norm(p.mean(0)-center_hint))
        center = iris_points.mean(0)
        radii = np.ptp(iris_points, axis=0)/2
        if min(radii) < 1.5:
            raise PreparationError('Iris resolution is insufficient for safe gaze.')
        # Refine the limbal radius from visible side arcs only: eyelids must not pull the fit.
        gray = cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY).astype(float)
        gx = cv2.Sobel(gray, cv2.CV_64F, 1, 0)
        candidates = np.linspace(.88, 1.12, 25)
        scores = []
        for scale in candidates:
            samples = []
            for angle in np.r_[np.linspace(-.35, .35, 15), np.linspace(np.pi-.35, np.pi+.35, 15)]:
                px, py = np.rint(center+radii*scale*[np.cos(angle), np.sin(angle)]).astype(int)
                if 0 <= px < w and 0 <= py < h:
                    samples.append(abs(gx[py, px]))
            scores.append(np.mean(samples) if samples else 0)
        # A weak edge leaves the anatomical estimate unchanged.
        scale = float(candidates[int(np.argmax(scores))]) if max(scores) > 15 else 1.
        radii *= scale
        poly = np.zeros((h*4, w*4), np.uint8)
        cv2.fillPoly(poly, [np.rint(boundary*4).astype(np.int32)], 255)
        aperture = cv2.resize(poly, (w, h), interpolation=cv2.INTER_AREA)/255.
        source_visibility = (aperture >= .5).astype(float)
        distance = cv2.distanceTransform(np.uint8(aperture > .5), cv2.DIST_L2, 5)
        aperture *= np.clip(distance-SETTINGS['aperture_inset_px'], 0, 1)
        radius = np.sqrt(((xx-center[0])/radii[0])**2+((yy-center[1])/radii[1])**2)
        ellipse = np.clip((1-radius)*min(radii)+.5, 0, 1)
        observed = ellipse*source_visibility
        domain = aperture > 0
        unknown = (ellipse > 0) & domain
        plate_part = harmonic_fill(rgb, domain, unknown)
        plate[unknown] = plate_part[unknown]
        masks[..., 0] = np.maximum(masks[..., 0], np.rint(aperture*255).astype(np.uint8))
        # Full ellipse is stored separately from the original visibility. It is never completed.
        masks[..., 1] = np.maximum(masks[..., 1], np.rint(ellipse*255).astype(np.uint8))
        masks[..., 2] = np.maximum(masks[..., 2], np.rint(source_visibility*255).astype(np.uint8))
        width = float(np.ptp(boundary[:, 0]))
        bounds = []
        reductions = []
        for axis, fraction in ((0, .05), (1, .02)):
            requested = width*fraction
            directions = []
            for sign in (-1, 1):
                valid = 0.
                for step in np.linspace(0, requested, 65)[1:]:
                    dx, dy = (sign*step, 0) if axis == 0 else (0, sign*step)
                    needed = translate(ellipse, dx, dy)
                    supported = translate(observed, dx, dy)
                    missing = float((np.maximum(0, needed-supported)*aperture).sum())
                    # Missing boundary material is protected in both compositors, never filled.
                    # Bound the stationary sliver so an unsupported shift cannot pass as gaze.
                    if missing > float(observed.sum())*SETTINGS['protected_boundary_fraction']:
                        break
                    valid = float(step)
                directions.append(valid)
            bounds.append(directions)
            if min(directions) < requested:
                reductions.append({'axis': axis, 'requested_px': requested, 'safe_negative_positive_px': directions})
        eyes.append({'center': center.tolist(), 'radii': radii.tolist(), 'width_px': width,
                     'bounds_px': bounds, 'edge_radius_scale': scale, 'reductions': reductions})
    eyes.sort(key=lambda e: e['center'][0])
    masks[..., 3] = 255
    if not all(max(e['bounds_px'][0]) >= .25 for e in eyes):
        raise PreparationError('Observed iris coverage cannot support a quarter-pixel gaze; original eyes retained.')
    # Check the complete combined envelope, not just the four independent axes.
    for eye in eyes:
        # Protect unsupported pixels dynamically; lower global bounds if too much
        # of the visible iris would have to stay stationary beneath the lash rim.
        center, radii = np.asarray(eye['center']), np.asarray(eye['radii'])
        ellipse = np.clip((1-np.sqrt(((xx-center[0])/radii[0])**2+((yy-center[1])/radii[1])**2))*min(radii)+.5, 0, 1)
        visible = ellipse*(masks[..., 2]/255.)
        for _ in range(20):
            worst = 0.
            for angle in np.linspace(0, 2*np.pi, 25):
                direction = [np.cos(angle), np.sin(angle)]
                dx, dy = [direction[axis]*eye['bounds_px'][axis][int(direction[axis] > 0)] for axis in (0, 1)]
                missing = np.maximum(0, translate(ellipse, dx, dy)-translate(visible, dx, dy))*(masks[..., 0]/255.)
                worst = max(worst, float(missing.sum()/max(1, visible.sum())))
            if worst <= SETTINGS['protected_boundary_fraction']:
                break
            eye['bounds_px'] = [[v*.9 for v in axis] for axis in eye['bounds_px']]
        eye['maximum_protected_boundary_fraction'] = worst
    for name, array in [('sclera.png', plate), ('ownership.png', masks), ('source.png', rgb)]:
        Image.fromarray(array).save(output/name)
    result = {'version': VERSION, 'source_sha256': package['original_sha256'],
              'motion_build_id': package['build_id'], 'crop': crop, 'eyes': eyes,
              'assets': {name: {'file': name, 'sha256': sha256((output/name).read_bytes()).hexdigest()}
                         for name in ('sclera.png', 'ownership.png')},
              'validation': {'valid': True, 'hidden_iris_generated': False, 'neutral_source_exact': True}}
    # Inspect native pixels rather than an upscaled generative preview.
    frames = [Image.fromarray(rgb)]
    for axis, sign in ((0, -1), (0, 1), (1, -1), (1, 1)):
        offsets = [[0., 0.] for _ in eyes]
        for i, eye in enumerate(eyes):
            offsets[i][axis] = sign*eye['bounds_px'][axis][int(sign > 0)]
        frames.append(Image.fromarray(render(rgb, plate, masks, offsets, [e['center'] for e in eyes])))
    sheet = Image.new('RGB', (w, (h+20)*len(frames)), '#222222')
    draw = ImageDraw.Draw(sheet)
    for i, frame in enumerate(frames):
        sheet.paste(frame, (0, i*(h+20)+20))
        draw.text((4, i*(h+20)+3), ('Original', 'Left bound', 'Right bound', 'Up bound', 'Down bound')[i], fill='white')
    sheet.save(output/'inspection.png')
    overlay = rgb.copy()
    overlay[masks[..., 0] > 128] = (overlay[masks[..., 0] > 128]*.6+[0, 100, 100]).astype(np.uint8)
    Image.fromarray(overlay).save(output/'overlay.png')
    (output/'gaze.json').write_text(json.dumps(result, indent=2)+'\n')
    return result

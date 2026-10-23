"""Automatic small-turn neck skinning and source-bound volumetric breathing."""
from itertools import product

import cv2
import numpy as np
from scipy.ndimage import gaussian_filter1d

from .faces import PreparationError
from .motion_geometry import smooth
from .projection_rig import sample, unproject

VERSION = 'neck-volume/3'
SETTINGS = {'upper_rotation_fraction': .85, 'lower_rotation_fraction': .05,
            'axis_depth_diameter_fraction': .45,
            'breath_forward_gain': 5., 'breath_lateral_gain': .35,
            'minimum_area_ratio': .25}


def rotate(p, angles):
    yaw, pitch, roll = np.radians(angles)
    cy, sy, cp, sp, cr, sr = np.cos(yaw), np.sin(yaw), np.cos(pitch), np.sin(pitch), np.cos(roll), np.sin(roll)
    a = np.column_stack((cy*p[:, 0]+sy*p[:, 2], p[:, 1], -sy*p[:, 0]+cy*p[:, 2]))
    b = np.column_stack((a[:, 0], cp*a[:, 1]-sp*a[:, 2], sp*a[:, 1]+cp*a[:, 2]))
    return np.column_stack((cr*b[:, 0]-sr*b[:, 1], sr*b[:, 0]+cr*b[:, 1], b[:, 2]))


def project(vertices, attributes, camera, rig, angles, breath=0, mode='full'):
    world = unproject(vertices, camera)
    original = world.copy()
    for j, (pivot, fraction) in enumerate(zip(rig['pivots'], rig['rotation_fractions'])):
        local = original-pivot
        world += attributes[:, j, None]*(rotate(local, np.asarray(angles)*fraction)-local)
    if mode == 'full':
        forward = -breath*attributes[:, 5]*rig['breath_forward_gain']
        shift = np.column_stack((-breath*attributes[:, 3]*rig['breath_lateral_gain'],
                                  breath*attributes[:, 4]))
        source_q = camera['distance']-original[:, 2]
        world[:, :2] += shift*source_q[:, None]/camera['focal']
        world[:, :2] -= forward[:, None]*(np.asarray(rig['chest_center_px'])-camera['center'])/camera['focal']
        world[:, 2] += forward
    q = camera['distance']-world[:, 2]
    return world[:, :2]*camera['focal']/q[:, None]+camera['center'], q


def validate(vertices, attributes, indices, camera, rig, bounds, face_height):
    t = indices.reshape(-1, 3)
    original = vertices[t, :2]*camera['size']
    ab, ac = original[:, 1]-original[:, 0], original[:, 2]-original[:, 0]
    reference = ab[:, 0]*ac[:, 1]-ab[:, 1]*ac[:, 0]
    minimum, clipped = float('inf'), 0
    for angles in product([-1., 0., 1.], repeat=3):
        for breath in [0., -face_height*.006]:
            xy, q = project(vertices, attributes, camera, rig, np.asarray(angles)*bounds, breath)
            triangles = xy[t]
            ab, ac = triangles[:, 1]-triangles[:, 0], triangles[:, 2]-triangles[:, 0]
            minimum = min(minimum, float(((ab[:, 0]*ac[:, 1]-ab[:, 1]*ac[:, 0])/reference).min()))
            clipped += int(((q <= camera['near']) | (q >= camera['far'])).sum())
    return {'valid': bool(np.isfinite(minimum) and minimum > SETTINGS['minimum_area_ratio'] and not clipped),
            'configurations': 54, 'minimum_area_ratio': minimum, 'clipped_vertices': clipped}


def prepare(package, vertices, indices, landmarks, classes, foreground):
    projection = package['projection']
    camera = projection['camera']
    w, h = camera['size']
    if foreground.size != (w, h):
        raise PreparationError('Foreground and neck package dimensions differ.')
    points = np.asarray(landmarks)
    chin = points[152, 1]
    fh, fw = package['anatomy']['face_height'], package['anatomy']['face_width']
    cx = float((points[234, 0]+points[454, 0])/2)
    x0, x1 = max(0, round(cx-fw*.18)), min(w, round(cx+fw*.18))
    clothes = cv2.resize(classes[4], (w, h))
    skin = cv2.resize(classes[2]+classes[3], (w, h))
    profile = gaussian_filter1d(np.mean(clothes[:, x0:x1]-skin[:, x0:x1], axis=1), max(1., fh*.015))
    lo, hi = round(chin+fh*.16), min(h-1, round(chin+fh*.60))
    candidates = np.flatnonzero(profile[lo:hi] > .18)
    base_y = float(lo+candidates[0]) if len(candidates) else float(min(h-1, chin+fh*.38))
    observed = bool(len(candidates))
    xy = vertices[:, :2]*[w, h]
    t = np.clip((xy[:, 1]-chin)/max(base_y-chin, 1), 0, 2)
    central = 1-smooth(fw*.42, fw*.85, abs(xy[:, 0]-cx))
    # Preserve strong coupling near the jaw, then distribute most of the twist
    # through the lower visible neck. The clothing attachment stays near rest.
    head = 1-smooth(.08, .65, t)
    lower = smooth(.30, 1., t)*(1-smooth(1., 1.7, t))
    upper = np.maximum(0., 1-head-smooth(.30, 1., t))
    # Outside the neck, retain the parent's head/hair attachment rather than
    # dragging long hair and accessories with a rectangular neck band.
    head = head*central+vertices[:, 3]*(1-central)
    upper *= central
    lower *= central
    total = np.maximum(1., head+upper+lower)
    head, upper, lower = head/total, upper/total, lower/total
    face = xy[:, 1] <= chin
    head[face], upper[face], lower[face] = 1., 0., 0.
    # A little broad shoulder follow fades into the stable lower torso.
    shoulder = (1-central)*smooth(1., 1.15, t)*(1-smooth(1.15, 2., t))*.08
    lower += shoulder*(1-head-upper-lower)
    material = sample(cv2.resize(classes[2]+classes[4]+classes[5], (w, h)), xy)
    chest_y = min(h-fh*.15, base_y+fh*.5)
    shoulder_start = base_y-fh*.35*(1-central)
    support = smooth(shoulder_start, shoulder_start+fh*.28, xy[:, 1])*(1-smooth(h-fh*.13, h, xy[:, 1]))
    support *= np.clip(material, 0, 1)
    support[face] = 0
    radial = np.clip((xy[:, 0]-cx)/(fw*.9), -1, 1)
    attributes = np.column_stack((head, upper, lower, support*radial, support, support)).astype('<f4')
    # Both neck pivots use the cached source depth, automatically localized.
    pivots = [projection['pivot']]
    sections = []
    alpha = np.asarray(foreground.convert('RGBA'))[:, :, 3]/255
    for y in [chin+(base_y-chin)*.22, base_y]:
        nearest = int(np.argmin(np.sum((xy-[cx, y])**2, axis=1)))
        surface_z = float(vertices[nearest, 2])
        row = min(h-1, max(0, round(y)))
        band = max(1, round(fh*.025))
        profile = np.mean(skin[max(0, row-band):min(h, row+band+1)] *
                          alpha[max(0, row-band):min(h, row+band+1)], axis=0)
        left, right = max(0, round(cx-fw*.45)), min(w, round(cx+fw*.45))
        candidates = np.flatnonzero(profile[left:right] > .30)+left
        observed_width = float(candidates[-1]-candidates[0]) if len(candidates) > 1 else fw*.50
        diameter = float(np.clip(observed_width, fw*.28, fw*.75))
        # The depth map describes the visible front skin. A rotation axis on
        # that surface produces almost no yaw displacement. Estimate an inner
        # axis from the visible neck section, without inventing hidden pixels.
        radius = diameter*SETTINGS['axis_depth_diameter_fraction']*(camera['distance']-surface_z)/camera['focal']
        z = surface_z-radius
        q = camera['distance']-z
        pivots.append([float((cx-camera['center'][0])*q/camera['focal']),
                       float((y-camera['center'][1])*q/camera['focal']), z])
        sections.append({'y': float(y), 'observed_skin_width_px': observed_width,
                         'used_diameter_px': diameter, 'estimated_axis_inset': radius,
                         'surface_z': surface_z, 'axis_z': z,
                         'skin_section_observed': bool(len(candidates) > 1)})
    rig = {'version': VERSION, 'pivots': pivots,
           'axis_estimation': {'method': 'visible-neck-section/1', 'sections': sections,
                               'assumed_depth_diameter_fraction': SETTINGS['axis_depth_diameter_fraction'],
                               'measured_anatomical_axis': False},
           'rotation_fractions': [1., SETTINGS['upper_rotation_fraction'], SETTINGS['lower_rotation_fraction']],
           'chest_center_px': [cx, float(chest_y)],
           'breath_forward_gain': SETTINGS['breath_forward_gain'],
           'breath_lateral_gain': SETTINGS['breath_lateral_gain'],
           'localization': {'chin_y': float(chin), 'base_y': base_y,
                            'clothing_transition_observed': observed, 'manual_overrides': False},
           'attributes': {'stride': 6, 'order': ['head', 'upper_neck', 'lower_neck', 'lateral', 'lift', 'forward']}}
    locations = np.array([[cx, chin+(base_y-chin)*s] for s in [.05, .25, .50, .75, 1.]])
    grid = projection['grid']
    grid_xy = locations/[w, h]*grid
    fields = attributes[:, :3].reshape(grid+1, grid+1, 3)
    strengths = sum(sample(fields[:, :, j], grid_xy)*fraction
                    for j, fraction in enumerate(rig['rotation_fractions']))
    rig['effective_yaw_profile'] = [{'neck_fraction': s, 'head_rotation_fraction': float(f)}
                                  for s, f in zip([.05, .25, .50, .75, 1.], strengths)]
    if np.any(np.diff(strengths) > .02) or strengths[0]-strengths[-1] < .7:
        raise PreparationError('Upper/lower neck twist profile is not sufficiently differentiated.')
    bounds = np.array([projection['bounds'][key] for key in ['yaw', 'pitch', 'roll']])
    rig['validation'] = validate(vertices, attributes, indices, camera, rig, bounds, fh)
    if not rig['validation']['valid']:
        raise PreparationError('Articulated neck/breath envelope inverts; preserve the failed preparation.')
    return rig, attributes

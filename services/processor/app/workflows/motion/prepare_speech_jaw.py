"""Prepare a subtle, skin-only speech jaw from cached source geometry."""
import argparse
import json
import shutil
from hashlib import sha256
from itertools import product
from pathlib import Path

import numpy as np
from scipy.spatial import cKDTree

from app.core.paths import ROOT
from app.preparation.faces import PreparationError
from app.workflows.motion.prepare_mandible import smooth


def convex_hull(points):
    ordered = sorted(set(map(tuple, np.asarray(points, dtype=float))))
    if len(ordered) < 3:
        raise PreparationError('Calibrated oral support is incomplete.')
    def cross(a, b, c):
        return (b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0])
    lower, upper = [], []
    for point in ordered:
        while len(lower) >= 2 and cross(lower[-2], lower[-1], point) <= 0:
            lower.pop()
        lower.append(point)
    for point in reversed(ordered):
        while len(upper) >= 2 and cross(upper[-2], upper[-1], point) <= 0:
            upper.pop()
        upper.append(point)
    return np.asarray(lower[:-1]+upper[:-1], dtype=float)


def calibrated_oral_support(manifest):
    """Convex support of the actual calibrated lip endpoints, not an edit crop.

    Existing oral interpolation, smile and normalized emotion weights are
    affine in their endpoint coordinates. The convex hull includes their
    extrema and smile-plus-emotion combinations without provider analysis.
    """
    oral = manifest['packages']['oral']
    expressions = manifest['packages']['expressions']
    smile = expressions.get('joint_articulation', {}).get('pairs', {})
    emotions = expressions.get('emotion_rig', {}).get('oral', {}).get('states', {})
    points = []
    for name, pair in sorted(oral['pairs'].items()):
        for phase in ('rest', 'open'):
            # The existing native/WebGL source vertex buffer is float32.
            # Bind to those exact loaded coordinates before adding the cached
            # double-precision expression deltas, matching native sampling.
            base = np.asarray(pair['mesh'][phase], dtype=np.float32).astype(float)[:20]
            zeros = np.zeros_like(base)
            delta = np.asarray(smile.get(name, {}).get(phase+'_delta', zeros), dtype=float)[:20]
            for amount in (0., 1.):
                vertex = base+delta*amount
                points.append(vertex)
                for state in sorted(emotions):
                    emotion = np.asarray(emotions[state].get(name, {}).get(phase+'_delta', zeros), dtype=float)[:20]
                    points.append(vertex+emotion)
    polygon = convex_hull(np.concatenate(points))
    return polygon+np.array([oral['crop']['x'], oral['crop']['y']])


def triangles_overlapping_support(xy, triangles, polygon, guard):
    """Conservative separating-axis test, including the filtering footprint."""
    points = xy[triangles]
    low, high = polygon.min(axis=0)-guard, polygon.max(axis=0)+guard
    candidates = np.flatnonzero(np.all(points.max(axis=1) >= low, axis=1)
                                & np.all(points.min(axis=1) <= high, axis=1))
    selected = points[candidates]
    overlaps = np.ones(len(candidates), dtype=bool)
    for a, b in zip(polygon, np.roll(polygon, -1, axis=0)):
        axis = np.array([-(b-a)[1], (b-a)[0]])
        padding = guard*np.linalg.norm(axis)
        projected = selected@axis
        support = polygon@axis
        overlaps &= ((projected.max(axis=1) >= support.min()-padding-1e-7)
                     & (projected.min(axis=1) <= support.max()+padding+1e-7))
    for index in range(3):
        edge = selected[:, (index+1) % 3]-selected[:, index]
        axes = np.column_stack((-edge[:, 1], edge[:, 0]))
        padding = guard*np.linalg.norm(axes, axis=1)
        projected = np.einsum('nvc,nc->nv', selected, axes)
        support = polygon@axes.T
        overlaps &= ((projected.max(axis=1) >= support.min(axis=0)-padding-1e-7)
                     & (projected.min(axis=1) <= support.max(axis=0)+padding+1e-7))
    result = np.zeros(len(triangles), dtype=bool)
    result[candidates] = overlaps
    return result


def polygon_vertical_bottom(polygon, x):
    values = []
    for a, b in zip(polygon, np.roll(polygon, -1, axis=0)):
        if (a[0] <= x < b[0]) or (b[0] <= x < a[0]):
            values.append(a[1]+(x-a[0])*(b[1]-a[1])/(b[0]-a[0]))
    if not values:
        raise PreparationError('Calibrated lips do not cross the measured chin midline.')
    return max(values)


def prepare(source: Path, landmarks: Path, output: Path):
    if output.exists():
        raise PreparationError('Preserve previous evidence; choose a new output directory.')
    manifest = json.loads((source / 'manifest.json').read_bytes())
    motion, oral = manifest['packages']['motion'], manifest['packages']['oral']
    if motion.get('mandible') or manifest['packages']['expressions']['reference_laughter'].get('presentation'):
        raise PreparationError('Use the preserved pre-articulation package.')
    points = np.asarray(json.loads(landmarks.read_bytes())['landmarks_px'], dtype=float)
    if points.shape != (478, 3) or not np.isfinite(points).all():
        raise PreparationError('Complete source-bound cached MediaPipe points required.')
    chin = points[152, :2]
    face_height = chin[1] - points[10, 1]
    if not np.allclose([chin[1], face_height], [motion['anatomy']['chin_y'], motion['anatomy']['face_height']], atol=.001):
        raise PreparationError('Landmarks belong to another source.')
    width, height = motion['canvas']['width'], motion['canvas']['height']
    geometry = motion['projection']['geometry']
    data = (source / manifest['geometry']['file']).read_bytes()
    vertices = np.frombuffer(data[:geometry['vertices'] * 20], '<f4').reshape(-1, 5).astype(float)
    triangles = np.frombuffer(data[geometry['vertices'] * 20:geometry['vertices'] * 20 + geometry['triangles'] * 6], '<u2').reshape(-1, 3)
    xy = vertices[:, :2] * [width, height]
    z = vertices[:, 2]
    camera = motion['projection']['camera']
    center, focal, distance = np.asarray(camera['center']), camera['focal'], camera['distance']
    world = np.column_stack(((xy - center) * (distance - z)[:, None] / focal, z))
    face_width = np.linalg.norm(points[234, :2] - points[454, :2])
    hinge_xy = (points[234, :2] + points[454, :2]) * .5
    near = lambda p: int(np.argmin(np.sum((xy - p) ** 2, axis=1)))
    hinge_z = min(z[near(hinge_xy)], z[near(chin)]) - face_width * .75
    hinge = np.array([*((hinge_xy - center) * (distance - hinge_z) / focal), hinge_z])
    oval = points[[234,93,132,58,172,136,150,149,176,148,152,377,400,378,379,365,397,288,361,323,454], :2]
    oval = oval[np.argsort(oval[:, 0])]
    x, y = xy[:, 0], xy[:, 1]
    jaw_boundary = np.interp(x, oval[:, 0], oval[:, 1])
    crop = oral['crop']
    support = calibrated_oral_support(manifest)
    upper_limit = float(support[:, 1].min())
    neck_limit = chin[1] + face_height * .08
    weights = ((1 - smooth(face_width * .32, face_width * .50, np.abs(x - chin[0])))
               * smooth(upper_limit - face_height * .02, chin[1] - face_height * .08, y)
               * (1 - smooth(jaw_boundary + face_height * .005, jaw_boundary + face_height * .08, y)))
    weights[y >= neck_limit] = 0
    weights[y <= upper_limit] = 0
    weights[vertices[:, 3] <= 0] = 0
    # Keep the actual lips/enamel source frame fixed. Skin elsewhere in the
    # broad edit rectangle must join the jaw rather than form a frozen island.
    guard = max(2., face_height * 2.6 / 720 * 2)
    overlaps = triangles_overlapping_support(xy, triangles, support, guard)
    protected_vertices = np.unique(triangles[overlaps])
    if len(protected_vertices) == 0:
        raise PreparationError('Calibrated oral support does not overlap the source mesh.')
    skin_distance = cKDTree(xy[protected_vertices]).query(xy)[0]
    edge_ids = np.concatenate((triangles[:, [0,1]], triangles[:, [1,2]], triangles[:, [2,0]]))
    mesh_step = float(np.median(np.linalg.norm(xy[edge_ids[:, 0]]-xy[edge_ids[:, 1]], axis=1)))
    transition = min(face_height*.10, float(skin_distance[near(chin)]))
    if transition <= max(guard*2, mesh_step*1.5):
        raise PreparationError('Insufficient source skin for a continuous jaw transition.')
    weights *= smooth(0, transition, skin_distance)
    weights[protected_vertices] = 0

    def project(angle, opening=1, head=(0, 0, 0), breath=0):
        d = world - hinge
        co, si = np.cos(-angle * opening), np.sin(-angle * opening)
        rotated = np.column_stack((d[:, 0], co * d[:, 1] - si * d[:, 2], si * d[:, 1] + co * d[:, 2])) + hinge
        moved = world + (rotated - world) * weights[:, None]
        yaw, pitch, roll = np.deg2rad(head)
        cy, sy, cp, sp, cr, sr = np.cos(yaw), np.sin(yaw), np.cos(pitch), np.sin(pitch), np.cos(roll), np.sin(roll)
        d = moved - np.asarray(motion['projection']['pivot'])
        ax, az = cy * d[:, 0] + sy * d[:, 2], -sy * d[:, 0] + cy * d[:, 2]
        by, bz = cp * d[:, 1] - sp * az, sp * d[:, 1] + cp * az
        moved += (np.column_stack((cr * ax - sr * by, sr * ax + cr * by, bz)) - d) * vertices[:, 3, None]
        projected = moved[:, :2] * focal / (distance - moved[:, 2, None]) + center
        projected[:, 1] += vertices[:, 4] * breath * face_height * .006
        projected[:, 0] += vertices[:, 4] * (xy[:, 0] - camera['pivot_px'][0]) * breath * face_height * .006 / height * .25
        return projected

    chin_index = near(chin)
    low, high = 0., .08
    target = face_height * .0075
    for _ in range(40):
        middle = (low + high) * .5
        if project(middle)[chin_index, 1] - xy[chin_index, 1] > target:
            high = middle
        else:
            low = middle
    angle = low
    def areas(positions):
        p = positions[triangles]
        a, b = p[:, 1] - p[:, 0], p[:, 2] - p[:, 0]
        return a[:, 0] * b[:, 1] - a[:, 1] * b[:, 0]
    area0 = areas(xy)
    bounds = motion['projection']['bounds']
    attempts = []
    for _ in range(16):
        minimum = 1.
        for opening in (0, .25, .5, .75, 1):
            for direction in product((-1, 0, 1), repeat=3):
                head = np.asarray(direction) * [bounds['yaw'], bounds['pitch'], bounds['roll']]
                for breath in (-1, 0, 1):
                    minimum = min(minimum, float(np.min(areas(project(angle, opening, head, breath)) / area0)))
        attempts.append({'angle': angle, 'minimum_area_ratio': minimum})
        if minimum > .25:
            break
        angle *= .9
    if minimum <= .25:
        raise PreparationError('Subtle jaw fails the existing geometry gate.')
    delta = project(angle) - xy
    neck = y >= neck_limit
    assert np.max(weights[protected_vertices]) == 0 and np.max(weights[neck]) == 0
    lower_support = polygon_vertical_bottom(support, chin[0])
    bridge = []
    for fraction in (.5, .65, .8):
        index = near([chin[0], lower_support+(chin[1]-lower_support)*fraction])
        bridge.append({'fraction': fraction, 'source': xy[index].tolist(), 'weight': float(weights[index]),
                       'displacement_px': delta[index].tolist()})
    if any(row['weight'] <= .001 for row in bridge) or any(b['weight']+1e-7 < a['weight'] for a,b in zip(bridge,bridge[1:])):
        raise PreparationError('The central lower-face bridge is disconnected or reverses its transition.')
    lower = {pose: float(np.asarray(oral['pairs']['Rest-' + pose]['mesh']['open'])[15, 1]) for pose in oral['poses'] if pose != 'Rest'}
    rest = float(np.asarray(oral['pairs']['Rest-A']['mesh']['rest'])[15, 1])
    maximum = max(max(value - rest, 0) for value in lower.values())
    opening = {pose: float(np.clip((value - rest) / maximum, 0, 1)) for pose, value in lower.items()}
    opening['Rest'] = 0.
    measured = float(delta[chin_index, 1] / face_height)
    rig = {'version': 'speech-jaw/2', 'source_sha256': motion['original_sha256'], 'motion_build_id': motion['build_id'],
           'oral_build_id': oral['build_id'], 'geometry_sha256': geometry['sha256'], 'hinge': hinge.tolist(),
           'chin': chin.tolist(), 'maximum_angle_radians': angle, 'weights': weights.tolist(), 'pose_opening': opening,
           'protected_oral_crop': crop, 'lower_neck_y': neck_limit, 'maximum_chin_fraction': .0075,
           'oral_support': {'version': 'calibrated-outer-lip-support/1', 'polygon': support.tolist(),
                            'sampling_guard_source_px': guard, 'transition_width_source_px': transition},
           'validation': {'valid': True, 'minimum_area_ratio': minimum, 'chin_fraction': measured,
                          'chin_travel_720_canvas_px': float(delta[chin_index, 1] * 720 / height),
                          'maximum_neck_weight': float(np.max(weights[neck])), 'maximum_oral_weight': float(np.max(weights[protected_vertices])),
                          'center_bridge': bridge},
           'provenance': {'landmarks_sha256': sha256(landmarks.read_bytes()).hexdigest(), 'points': 478,
                          'estimated_hinge': True, 'manual_masks': False, 'jaw_targets': 'measured-lower-outer-lip-travel',
                          'oral_protection': 'calibrated-lip-support-triangles-with-smooth-skin-transition', 'sampling_guard_source_px': guard,
                          'scope': 'speech-only lower-face skin; oral materials and laughter unchanged'}}
    output.mkdir(parents=True)
    shutil.copytree(source, output / 'package')
    encoded = json.dumps(rig, separators=(',', ':')).encode()
    (output / 'speech-jaw.json').write_bytes(encoded)
    motion['speech_jaw'] = rig
    manifest['identity_files'].append({'path': 'speech-jaw/2', 'sha256': sha256(encoded).hexdigest(), 'bytes': len(encoded)})
    manifest['package_hash'] = sha256(json.dumps(manifest['identity_files'], separators=(',', ':'), ensure_ascii=False).encode()).hexdigest()
    (output / 'package/manifest.json').write_text(json.dumps(manifest, indent=2))
    report = {'package_hash': manifest['package_hash'], 'validation': rig['validation'], 'pose_opening': opening,
              'attempts': attempts, 'textures_unchanged': True, 'oral_unchanged': True, 'provider_requests': 0}
    (output / 'report.json').write_text(json.dumps(report, indent=2))
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--landmarks', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(prepare(args.source, args.landmarks, args.output)))


if __name__ == '__main__':
    main()

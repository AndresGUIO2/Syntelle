"""Replace joint-smile skin appearance with source-only deformation offline."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np
from PIL import Image

from app.preparation.archive import encoded
from app.preparation.expression_skin import source_skin_frame
from app.preparation.faces import PreparationError
from app.preparation.joint_expressions import prepare_field, prepare_pairs
from app.workflows.rendering.export_native import resource_paths, validate_bindings


def prepare(bundle, landmarks, output):
    if output.exists():
        raise PreparationError('Preserve previous preparations; choose a fresh output directory.')
    folders, specification = resource_paths(bundle, None)
    names = {'motion': 'package.json', 'oral': 'package.json',
             'gaze': 'gaze.json', 'expressions': 'expressions.json'}
    packages = {key: json.loads((folders[key]/name).read_bytes()) for key, name in names.items()}
    validate_bindings(packages, specification)
    ex, oral, motion = (packages[key] for key in ('expressions', 'oral', 'motion'))
    points = np.asarray(json.loads(landmarks.read_bytes())['landmarks_px'], float)
    if points.shape != (478, 3) or not np.isfinite(points).all():
        raise PreparationError('Complete finite cached landmarks are required.')
    measured = (points[152, 1], np.linalg.norm(points[234, :2]-points[454, :2]),
                points[152, 1]-points[10, 1])
    anatomy = motion['anatomy']
    if not np.allclose(measured, [anatomy[key] for key in ('chin_y', 'face_width', 'face_height')], atol=1e-3, rtol=0):
        raise PreparationError('Cached landmarks differ from source-bound anatomy.')
    shutil.copytree(folders['expressions'], output/'expressions')
    shutil.copy2(bundle, output/'input-bundle.json')
    shutil.copy2(landmarks, output/'landmarks.json')
    source_root = Path(__file__).resolve().parents[2]
    for relative in ('preparation/expression_skin.py', 'preparation/joint_expressions.py',
                     'preparation/expressions.py', 'workflows/motion/prepare_expression_skin.py'):
        target = output/'sources'/relative
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source_root/relative, target)
    field_dir = output/'reconstructed-field'
    field_dir.mkdir()
    # The existing registered landmarks reconstruct the same joint field; its
    # prepared lip deltas must agree before any appearance resource is changed.
    prepare_field(points, np.asarray(ex['smile']['reference_landmarks']), ex,
                  Image.open(folders['expressions']/ex['field']['file']), field_dir)
    field = np.load(field_dir/'field.npy')
    joint = ex['joint_articulation']
    rebuilt = prepare_pairs(oral, field, ex['crop'],
                            np.asarray(ex['smile']['reference_landmarks']), points)
    for name, previous in joint['pairs'].items():
        for key in ('rest_delta', 'open_delta'):
            if not np.allclose(previous[key], rebuilt['pairs'][name][key], atol=1e-7, rtol=0):
                raise PreparationError('Reconstructed skin field differs from prepared lip geometry.')
    source = folders['motion_assets']/motion['foreground']
    foreground = np.asarray(Image.open(source).convert('RGBA'))
    frames = []
    for index, amount in enumerate(np.linspace(0, 1, 25)):
        name = f'source-smile-skin-{index:02d}.webp'
        path = output/'expressions'/name
        Image.fromarray(source_skin_frame(foreground, field, ex['crop'], amount)).save(
            path, lossless=True, exact=True, method=6)
        frames.append({'file': name, 'sha256': sha256(path.read_bytes()).hexdigest()})
    joint['skin_frames'] = frames
    joint['skin_appearance'] = {'version': 'source-skin-deformation/1',
                              'source_foreground_sha256': sha256(source.read_bytes()).hexdigest(),
                              'field_sha256': sha256((field_dir/'field.npy').read_bytes()).hexdigest(),
                              'joint_geometry_verified': True, 'reference_appearance_blend': False}
    (output/'expressions/expressions.json').write_bytes(encoded(ex))
    result = dict(specification)
    result['expressions'] = (output/'expressions').resolve().as_posix()
    (output/'resource-bundle.json').write_bytes(encoded(result))
    (output/'preparation.json').write_bytes(encoded({
        'version': 'source-skin-deformation/1', 'input_bundle_sha256': sha256(bundle.read_bytes()).hexdigest(),
        'landmarks_sha256': sha256(landmarks.read_bytes()).hexdigest(),
        'skin': joint['skin_appearance'], 'geometry_changed': False,
        'provider_requests': 0, 'owner_acceptance': False}))
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('bundle', 'landmarks', 'output'):
        parser.add_argument('--'+name, type=Path, required=True)
    args = parser.parse_args()
    prepare(args.bundle.resolve(), args.landmarks.resolve(), args.output.resolve())
    print(json.dumps({'prepared': str(args.output), 'provider_requests': 0}))

"""Validate bounded head gesture ranges using existing character geometry, offline."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np

from app.core.paths import ROOT
from app.preparation.archive import encoded
from app.preparation.faces import PreparationError
from app.preparation.projection_rig import validate
from app.workflows.rendering.export_native import resource_paths, validate_bindings


def rebind_motion(value, previous, current):
    if isinstance(value, dict):
        for key, child in value.items():
            if key == 'motion_build_id':
                if child != previous:
                    raise PreparationError('Nested expression belongs to a different motion package.')
                value[key] = current
            else:
                rebind_motion(child, previous, current)
    elif isinstance(value, list):
        for child in value:
            rebind_motion(child, previous, current)


def prepare(bundle: Path, output: Path, pitch: float = 4., yaw=None, roll=None) -> dict:
    if not np.isfinite(pitch) or not 1.2 <= pitch <= 4:
        raise PreparationError('Requested pitch must be within the bounded 1.2–4 degree experiment.')
    if output.exists():
        raise PreparationError('Preserve previous evidence; choose a fresh output directory.')
    for value,maximum in ((yaw,4.5),(roll,3.)):
        if value is not None and (not np.isfinite(value) or not 0<value<=maximum):
            raise PreparationError('Requested yaw/roll exceeds the bounded gesture experiment.')
    folders, specification = resource_paths(bundle, None)
    filenames = {'oral': 'package.json', 'motion': 'package.json',
                 'gaze': 'gaze.json', 'expressions': 'expressions.json'}
    packages = {key: json.loads((folders[key]/file).read_bytes()) for key, file in filenames.items()}
    validate_bindings(packages, specification)
    motion = packages['motion']
    projection = motion['projection']
    geometry = projection['geometry']
    binary = (folders['motion']/geometry['file']).read_bytes()
    if sha256(binary).hexdigest() != geometry['sha256']:
        raise PreparationError('Projection geometry checksum mismatch.')
    vertices = np.frombuffer(binary, '<f4', count=geometry['vertices']*5).reshape(-1, 5)
    indices = np.frombuffer(binary, '<u2', count=geometry['triangles']*3,
                            offset=geometry['vertices']*5*4).reshape(-1, 3)
    bounds = [projection['bounds']['yaw'] if yaw is None else yaw,
              pitch,projection['bounds']['roll'] if roll is None else roll]
    report = validate(vertices, indices, projection['camera'], projection['pivot'], bounds,
                      motion['anatomy']['face_height'])
    output.mkdir(parents=True)
    (output/'input-bundle.json').write_bytes(bundle.read_bytes())
    (output/'geometry-review.json').write_bytes(encoded(report))
    if not report['valid']:
        raise PreparationError('Gesture range failed geometry; evidence preserved without fallback or activation.')
    previous = motion['build_id']
    projection['bounds']['pitch'] = pitch
    projection['bounds']['yaw'] = bounds[0]
    projection['bounds']['roll'] = bounds[2]
    projection['validation'] = report
    projection['validation_attempts'] = [report]
    version='repeated-nod-envelope/1' if yaw is None and roll is None else 'head-gesture-envelope/2'
    motion['gesture_envelope'] = {'version': version, 'parent_build_id': previous,
                                  'geometry_sha256': geometry['sha256'], 'idle_unchanged': True,
                                  'owner_acceptance': False, 'paid_requests': 0}
    motion.pop('build_id')
    motion['build_id'] = sha256(encoded(motion)).hexdigest()
    result = dict(specification)
    for key in ('motion', 'gaze', 'expressions'):
        destination = output/key
        shutil.copytree(folders[key], destination)
        package = packages[key]
        if key != 'motion':
            rebind_motion(package, previous, motion['build_id'])
        (destination/filenames[key]).write_bytes(encoded(package))
        result[key] = destination.resolve().as_posix()
    (output/'resource-bundle.json').write_bytes(encoded(result))
    return {'bundle': str(output/'resource-bundle.json'), 'validation': report,
            'paid_requests': 0, 'owner_acceptance': False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--pitch', type=float, default=4.)
    parser.add_argument('--yaw',type=float)
    parser.add_argument('--roll',type=float)
    args = parser.parse_args()
    print(json.dumps(prepare(args.bundle.resolve(), args.output.resolve(), args.pitch,args.yaw,args.roll)))


if __name__ == '__main__':
    main()

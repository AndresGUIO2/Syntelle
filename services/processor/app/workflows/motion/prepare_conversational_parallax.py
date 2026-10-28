"""Prepare an isolated generic conversational-motion candidate, without providers."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.preparation.archive import encoded
from app.preparation.faces import PreparationError
from app.preparation.conversational_parallax import inspect_ranges, oral_transition, rebind, write_json, validate_dental_transition
from app.workflows.rendering.export_native import resource_paths, validate_bindings


def prepare(bundle, output, inspect_only=False):
    if output.exists():
        raise PreparationError('Preserve previous candidates; choose a fresh output directory.')
    folders, specification = resource_paths(bundle, None)
    names = {'oral': 'package.json', 'motion': 'package.json', 'gaze': 'gaze.json', 'expressions': 'expressions.json'}
    packages = {key: json.loads((folders[key]/name).read_bytes()) for key, name in names.items()}
    validate_bindings(packages, specification)
    output.mkdir(parents=True)
    (output/'input-bundle.json').write_bytes(bundle.read_bytes())
    report = inspect_ranges(packages['motion'], folders['motion'])
    write_json(output/'range-inspection.json', report)
    if inspect_only:
        return report
    oral_transition(packages['expressions'], packages['oral'], specification['source_sha256'])
    write_json(output/'dental-review.json',validate_dental_transition(packages['expressions'],folders['expressions'],packages['oral']))
    motion = packages['motion']; previous = motion.pop('build_id')
    motion['conversational_motion'] = dict(version='conversational-motion/1',
        transition_ms=[1800,3600], hold_ms=[350,1000], gesture_window_seconds=20,
        maximum_cycles=8, minimum_gesture_gap_seconds=2.2, recovery_ms=240)
    from app.preparation import conversational_parallax
    motion['conversational_motion']['preparation_sources']={p.name:sha256(p.read_bytes()).hexdigest() for p in (Path(__file__),Path(conversational_parallax.__file__))}
    motion['build_id'] = sha256(encoded(motion)).hexdigest()
    result = dict(specification)
    for key in ('motion', 'gaze', 'expressions'):
        shutil.copytree(folders[key], output/key)
        if key != 'motion':
            rebind(packages[key], previous, motion['build_id'])
        write_json(output/key/names[key], packages[key])
        result[key] = (output/key).as_posix()
    write_json(output/'resource-bundle.json', result)
    write_json(output/'preparation.json', dict(version='conversational-parallax/1', source_sha256=specification['source_sha256'],
               parent_motion=previous, motion_build_id=motion['build_id'], paid_calls=0, manual_overrides=False,
               owner_acceptance=False, arbitrary_uploads_qualified=False))
    return dict(bundle=str(output/'resource-bundle.json'), source_sha256=specification['source_sha256'], paid_calls=0)


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--inspect-only', action='store_true')
    args=parser.parse_args()
    print(json.dumps(prepare(args.bundle.resolve(), args.output.resolve(), args.inspect_only)))

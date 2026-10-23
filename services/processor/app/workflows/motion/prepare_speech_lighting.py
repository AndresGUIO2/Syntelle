"""Add reference-derived speech contact lighting to an existing character bundle."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.preparation.archive import encoded
from app.preparation.aperture_lighting import prepare_lighting
from app.preparation.faces import PreparationError
from app.workflows.rendering.export_native import resource_paths, validate_bindings


def rebind(value, previous, current, binding='oral_build_id'):
    if isinstance(value, dict):
        for key, child in value.items():
            if key == binding:
                if child != previous:
                    raise PreparationError('Nested oral binding mismatch.')
                value[key] = current
            else:
                rebind(child, previous, current, binding)
    elif isinstance(value, list):
        for child in value:
            rebind(child, previous, current, binding)


def prepare(bundle, output):
    if output.exists():
        raise PreparationError('Preserve previous evidence; use a fresh output directory.')
    folders, specification = resource_paths(bundle, None)
    filenames = {'oral':'package.json', 'motion':'package.json', 'gaze':'gaze.json', 'expressions':'expressions.json'}
    packages = {key:json.loads((folders[key]/name).read_bytes()) for key,name in filenames.items()}
    validate_bindings(packages, specification)
    reference = packages['expressions'].get('reference_laughter', {})
    if reference.get('version') != 'reference-laughter/1' or not reference.get('validation', {}).get('valid'):
        raise PreparationError('A validated source-bound laughing reference is required.')
    inner = reference['oral']['inner'][-1]
    reference_gap = inner[15][1]-inner[5][1]
    result = dict(specification)
    for key in filenames:
        shutil.copytree(folders[key], output/key)
        result[key] = (output/key).resolve().as_posix()
    previous = packages['oral']['build_id']
    measurements = prepare_lighting(packages['oral'], output/'oral', reference_gap)
    packages['oral'].pop('build_id')
    current = sha256(encoded(packages['oral'])).hexdigest()
    packages['oral']['build_id'] = current
    for key in filenames:
        if key != 'oral':
            rebind(packages[key], previous, current)
    prior_motion = packages['motion'].pop('build_id')
    packages['motion']['build_id'] = sha256(encoded(packages['motion'])).hexdigest()
    for key in ('gaze','expressions'):
        rebind(packages[key], prior_motion, packages['motion']['build_id'], 'motion_build_id')
    for key, name in filenames.items():
        (output/key/name).write_bytes(encoded(packages[key]))
    # Only mask B may change. All appearance images, including the manual smile
    # and canonical enamel, are preserved byte for byte.
    masks = {p['mask'] for p in packages['oral']['poses'].values()}
    unchanged = {}
    for key in filenames:
        for path in folders[key].rglob('*'):
            if path.suffix.lower() not in ('.png','.jpg','.webp') or key == 'oral' and path.relative_to(folders[key]).as_posix() in masks:
                continue
            identity = sha256(path.read_bytes()).hexdigest()
            if identity != sha256((output/key/path.relative_to(folders[key])).read_bytes()).hexdigest():
                raise PreparationError('Appearance bytes changed during lighting preparation.')
            unchanged[key+'/'+path.relative_to(folders[key]).as_posix()] = identity
    (output/'input-bundle.json').write_bytes(bundle.read_bytes())
    (output/'resource-bundle.json').write_bytes(encoded(result))
    report = {'version':'speech-aperture-lighting-preparation/1', 'source_sha256':specification['source_sha256'],
              'parent_oral_build_id':previous, 'oral_build_id':current,
              'lighting':packages['oral']['oral']['visible_aperture'], 'poses':measurements,
              'unchanged_images':unchanged, 'paid_provider_calls':0, 'owner_acceptance':False}
    (output/'preparation.json').write_bytes(encoded(report))
    return report


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    report = prepare(args.bundle.resolve(), args.output.resolve())
    print(json.dumps({'lighting':report['lighting'], 'unchanged_images':len(report['unchanged_images']), 'paid_provider_calls':0}))

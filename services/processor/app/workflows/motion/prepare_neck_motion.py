"""Extend cached RGB-D packages without models, providers or character overrides."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
import time

import numpy as np
from PIL import Image

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.preparation.neck_rig import prepare, SETTINGS, VERSION
from app.workflows.motion.prepare_projection_motion import verified_cache
from app.workflows.visemes.viseme_trial import write_report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, default=ROOT/'docs/characters/neck-inputs.json')
    parser.add_argument('--output', type=Path, default=ROOT/'.tools/neck-motion-19/prepared-current')
    parser.add_argument('--experiment', default='neck-motion-19')
    args = parser.parse_args()
    if not args.experiment.replace('-', '').isalnum():
        raise PreparationError('Invalid experiment identifier.')
    db = Archive()
    sources = [Path(__file__), ROOT/'services/processor/app/preparation/neck_rig.py']
    algorithms = {p.name: sha256(p.read_bytes()).hexdigest() for p in sources}
    for item in json.loads(args.manifest.read_text())['characters']:
        name = item['id']
        if not name.replace('-', '').isalnum():
            raise PreparationError('Invalid character identifier.')
        source = (ROOT/item['prepared']).resolve()
        inputs = {p.name: sha256(p.read_bytes()).hexdigest() for p in [source/'package.json', source/'projection.bin',
            source/'inputs/landmarks.json', source/'inputs/classes.npz', source/'inputs/foreground.webp']}
        binding = sha256(encoded({'inputs': inputs, 'settings': SETTINGS, 'algorithms': algorithms})).hexdigest()
        out = args.output/name
        out.mkdir(parents=True, exist_ok=True)
        if not verified_cache(out, binding):
            started = time.perf_counter()
            shutil.copytree(source, out/'parent', dirs_exist_ok=True)
            for p in sources:
                shutil.copyfile(p, out/p.name)
            write_report(out/'input-record.json', {'binding': binding, 'inputs': inputs, 'algorithms': algorithms})
            write_report(out/'settings.json', SETTINGS)
            db.archive_tree(args.experiment+'/'+name+'/inputs', out)
            package = json.loads((source/'package.json').read_text())
            g = package['projection']['geometry']
            raw = (source/'projection.bin').read_bytes()
            if sha256(raw).hexdigest() != g['sha256']:
                raise PreparationError('Parent projection geometry differs.')
            vertices = np.frombuffer(raw, '<f4', count=g['vertices']*5).reshape(-1, 5)
            indices = np.frombuffer(raw, '<u2', count=g['triangles']*3, offset=g['vertices']*20)
            try:
                rig, attributes = prepare(package, vertices, indices,
                    json.loads((source/'inputs/landmarks.json').read_text())['landmarks_px'],
                    np.load(source/'inputs/classes.npz')['probabilities'],
                    Image.open(source/'inputs/foreground.webp'))
                offset = (len(raw)+3)//4*4
                rig['attributes']['offset_bytes'] = offset
                rig['parent_build_id'] = package['build_id']
                binary = raw+bytes(offset-len(raw))+attributes.tobytes()
                (out/'projection.bin').write_bytes(binary)
                package['projection']['articulation'] = rig
                package['projection']['geometry']['sha256'] = sha256(binary).hexdigest()
                package['projection']['preparation_binding'] = binding
                package.pop('build_id')
                package['build_id'] = sha256(encoded(package)).hexdigest()
                write_report(out/'package.json', package)
                write_report(out/'record.json', {'binding': binding, 'version': VERSION, 'seconds': time.perf_counter()-started,
                    'inputs': inputs, 'algorithms': algorithms, 'new_provider_requests': 0})
            except PreparationError as failure:
                write_report(out/'failure.json', {'error': str(failure), 'binding': binding})
                db.archive_tree(args.experiment+'/'+name+'/failed', out)
                raise
            print(name, db.archive_tree(args.experiment+'/'+name, out), flush=True)
        else:
            print(name, 'matching cached preparation', flush=True)
        target = ROOT/'apps/web/src/experiments/motion/neck'/name
        target.mkdir(parents=True, exist_ok=True)
        for file in ['package.json', 'projection.bin']:
            shutil.copyfile(out/file, target/file)


if __name__ == '__main__':
    main()

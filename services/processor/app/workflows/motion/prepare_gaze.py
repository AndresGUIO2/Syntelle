"""Prepare source-supported gaze from immutable portraits and cached landmarks."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
import time
import platform
import cv2
import numpy
import scipy

from PIL import Image
from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.preparation.gaze import prepare, SETTINGS


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, default=ROOT/'docs/characters/gaze-inputs.json')
    parser.add_argument('--output', type=Path, default=ROOT/'.tools/attention-motion-24/prepared')
    parser.add_argument('--cases', nargs='+', required=True)
    parser.add_argument('--install-preview', action='store_true')
    args = parser.parse_args()
    db = Archive()
    manifest = json.loads(args.manifest.read_text())
    items = {item['id']: item for item in manifest['characters']}
    for name in args.cases:
        if name not in items or not name.replace('-', '').isalnum():
            raise PreparationError('Unknown character identifier.')
        item = items[name]
        paths = {key: ROOT/item[key] for key in ('original', 'motion', 'landmarks')}
        sources = [Path(__file__), ROOT/'services/processor/app/preparation/gaze.py', ROOT/'services/processor/app/preparation/eyes.py']
        inputs = {key: sha256(path.read_bytes()).hexdigest() for key, path in paths.items()}
        algorithms = {path.name: sha256(path.read_bytes()).hexdigest() for path in sources}
        versions = {'python': platform.python_version(), 'opencv': cv2.__version__, 'numpy': numpy.__version__, 'scipy': scipy.__version__, 'pillow': Image.__version__}
        binding = sha256(encoded({'inputs': inputs, 'settings': SETTINGS, 'algorithms': algorithms, 'versions': versions})).hexdigest()
        out = args.output/name/binding[:16]
        out.mkdir(parents=True, exist_ok=True)
        if not (out/'record.json').exists():
            if (out/'failure.json').exists():
                raise PreparationError('Matching preparation previously failed; preserve it and change the algorithm.')
            started = time.perf_counter()
            for key, path in paths.items():
                shutil.copyfile(path, out/(key+path.suffix))
            for path in sources:
                shutil.copyfile(path, out/path.name)
            (out/'input-record.json').write_bytes(encoded({'binding': binding, 'inputs': inputs, 'algorithms': algorithms, 'settings': SETTINGS, 'versions': versions}))
            db.archive_tree('attention-motion-24/'+name+'/inputs', out)
            try:
                package = json.loads(paths['motion'].read_text())
                if package['original_sha256'] != inputs['original']:
                    raise PreparationError('Original image does not match its frozen package.')
                prepare(Image.open(paths['original']), json.loads(paths['landmarks'].read_text())['landmarks_px'], package, out)
                files = {p.name: sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()}
                (out/'record.json').write_bytes(encoded({'binding': binding, 'files': files, 'seconds': time.perf_counter()-started, 'provider_requests': 0}))
            except Exception as error:
                (out/'failure.json').write_bytes(encoded({'binding': binding, 'error': str(error)}))
                db.archive_tree('attention-motion-24/'+name+'/failed', out)
                raise
        else:
            record = json.loads((out/'record.json').read_text())
            if any(sha256((out/file).read_bytes()).hexdigest() != digest for file, digest in record['files'].items()):
                raise PreparationError('Cached gaze evidence differs from its recorded hashes.')
        print(name, db.archive_tree('attention-motion-24/'+name, out), flush=True)
        if args.install_preview:
            target = ROOT/'apps/web/src/experiments/motion/gaze'/name
            target.mkdir(parents=True, exist_ok=True)
            for file in ('gaze.json', 'sclera.png', 'ownership.png'):
                shutil.copyfile(out/file, target/file)


if __name__ == '__main__':
    main()

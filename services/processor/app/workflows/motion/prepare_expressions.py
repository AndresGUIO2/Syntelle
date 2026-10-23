"""Prepare automatic local smile/brow fields without image generation."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
import time
import platform
import cv2
import numpy

from PIL import Image
from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.preparation.expressions import SETTINGS, prepare


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, default=ROOT/'docs/characters/expression-inputs.json')
    parser.add_argument('--output', type=Path, default=ROOT/'.tools/expressions-25/prepared')
    parser.add_argument('--cases', nargs='+', required=True)
    parser.add_argument('--install-preview', action='store_true')
    args = parser.parse_args()
    db = Archive()
    items = {item['id']: item for item in json.loads(args.manifest.read_text())['characters']}
    for name in args.cases:
        if name not in items or not name.replace('-', '').isalnum():
            raise PreparationError('Unknown character identifier.')
        item = items[name]
        oral_file = ROOT/item['oral']; oral_root = oral_file.parent
        oral = json.loads(oral_file.read_text())
        paths = {key: ROOT/item[key] for key in ('original', 'motion', 'landmarks')}
        paths.update({'oral': oral_file, 'upper': oral_root/oral['reference']['upper'], 'lower': oral_root/oral['reference']['lower']})
        sources = [Path(__file__), ROOT/'services/processor/app/preparation/expressions.py', ROOT/'services/processor/app/preparation/eyes.py']
        inputs = {key: sha256(path.read_bytes()).hexdigest() for key, path in paths.items()}
        algorithms = {p.name: sha256(p.read_bytes()).hexdigest() for p in sources}
        versions = {'python': platform.python_version(), 'opencv': cv2.__version__, 'numpy': numpy.__version__, 'pillow': Image.__version__}
        binding = sha256(encoded({'inputs': inputs, 'settings': SETTINGS, 'algorithms': algorithms, 'versions': versions})).hexdigest()
        out = args.output/name/binding[:16]; out.mkdir(parents=True, exist_ok=True)
        if not (out/'record.json').exists():
            if (out/'failure.json').exists():
                raise PreparationError('This preparation failed; preserve it and correct the common algorithm.')
            start = time.perf_counter()
            for key, path in paths.items():
                shutil.copyfile(path, out/(key+path.suffix))
            for path in sources:
                shutil.copyfile(path, out/path.name)
            (out/'input-record.json').write_bytes(encoded({'binding': binding, 'inputs': inputs, 'algorithms': algorithms, 'settings': SETTINGS, 'versions': versions}))
            db.archive_tree('expressions-25/'+name+'/inputs', out)
            try:
                package = json.loads(paths['motion'].read_text())
                if inputs['original'] != package['original_sha256'] or package['oral_build_id'] != oral['build_id']:
                    raise PreparationError('Original or oral package differs from frozen motion inputs.')
                result = prepare(Image.open(paths['original']), json.loads(paths['landmarks'].read_text())['landmarks_px'], package,
                                 oral, Image.open(paths['upper']), Image.open(paths['lower']), out)
                result['field'] = {'file': 'field.png', 'sha256': sha256((out/'field.png').read_bytes()).hexdigest()}
                (out/'expressions.json').write_bytes(encoded(result))
                files = {p.name: sha256(p.read_bytes()).hexdigest() for p in out.iterdir() if p.is_file()}
                (out/'record.json').write_bytes(encoded({'binding': binding, 'files': files, 'seconds': time.perf_counter()-start, 'provider_requests': 0}))
            except Exception as error:
                (out/'failure.json').write_bytes(encoded({'binding': binding, 'error': str(error)}))
                db.archive_tree('expressions-25/'+name+'/failed', out)
                raise
        else:
            record = json.loads((out/'record.json').read_text())
            if any(sha256((out/file).read_bytes()).hexdigest() != digest for file, digest in record['files'].items()):
                raise PreparationError('Expression cache failed hash validation.')
        print(name, db.archive_tree('expressions-25/'+name, out), flush=True)
        if args.install_preview:
            target = ROOT/'apps/web/src/experiments/motion/expressions'/name; target.mkdir(parents=True, exist_ok=True)
            for file in ('expressions.json', 'field.png'):
                shutil.copyfile(out/file, target/file)


if __name__ == '__main__':
    main()

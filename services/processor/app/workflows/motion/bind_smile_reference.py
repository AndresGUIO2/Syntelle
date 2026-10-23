"""Prepare an inspected source-bound smile companion, reusing existing oral assets."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
import time
import platform

import cv2
import numpy as np
import scipy
from PIL import Image
from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.detection import PreparationDetector
from app.preparation.faces import PreparationError
from app.preparation.smile_reference import prepare, VERSION


def digest(path):
    return sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--expressions', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--install', type=Path)
    args = parser.parse_args()
    config = json.loads(args.config.read_text())
    record = json.loads((args.reference/'run.json').read_text())
    review = json.loads((args.reference/'visual-review.json').read_text())
    raw = args.reference/record['file']
    if record['status'] != 'received' or not review['accepted'] or review['raw_sha256'] != digest(raw):
        raise PreparationError('A received and visually inspected reference is required.')
    paths = {'original': ROOT/config['original'], 'landmarks': ROOT/config['cached_landmarks'],
             'generated': raw, 'previous': args.expressions/'expressions.json',
             'field': args.expressions/'field.png'}
    previous = json.loads(paths['previous'].read_text())
    if previous['source_sha256'] != digest(paths['original']):
        raise PreparationError('Expression reference and original portrait hashes differ.')
    source_paths = [Path(__file__), *[ROOT/'services/processor/app/preparation'/name
        for name in ('smile_reference.py', 'expressions.py', 'eyes.py', 'mouth.py', 'faces.py', 'detection.py')]]
    algorithms = {str(p.resolve().relative_to(ROOT)): digest(p) for p in source_paths}
    versions = {'python': platform.python_version(), 'scipy': scipy.__version__, 'opencv': cv2.__version__,
                'numpy': np.__version__, 'pillow': Image.__version__,
                'mediapipe_model_sha256': digest(ROOT/'.tools/models/face_landmarker.task')}
    inputs = {name: digest(path) for name, path in paths.items()}
    binding = sha256(encoded({'version': VERSION, 'inputs': inputs, 'algorithms': algorithms, 'versions': versions})).hexdigest()
    out = args.output/binding[:16]; out.mkdir(parents=True, exist_ok=True)
    db = Archive()
    if not (out/'record.json').exists():
        start = time.perf_counter()
        for name, path in paths.items():
            shutil.copyfile(path, out/('input-'+name+path.suffix))
        sources = out/'sources'; sources.mkdir(exist_ok=True)
        for path in source_paths:
            shutil.copyfile(path, sources/path.name)
        (out/'input-record.json').write_bytes(encoded({'inputs': inputs, 'algorithms': algorithms, 'versions': versions, 'binding': binding}))
        db.archive_tree('smile-reference-26/preparation-inputs', out)
        with PreparationDetector(ROOT/'.tools/models/face_landmarker.task') as detector:
            generated_face = detector.detect(Image.open(raw))
        (out/'generated-landmarks.json').write_bytes(encoded(generated_face.as_dict()))
        result = prepare(Image.open(paths['original']), Image.open(raw),
                         np.asarray(json.loads(paths['landmarks'].read_text())['landmarks_px']),
                         generated_face.points, previous, Image.open(paths['field']), out)
        result['smile']['reference_sha256'] = digest(raw)
        result['smile']['material']['sha256'] = digest(out/'smile-material.png')
        result['field'] = {'file': 'field.png', 'sha256': digest(out/'field.png')}
        (out/'expressions.json').write_bytes(encoded(result))
        files = {p.name: digest(p) for p in out.iterdir() if p.is_file()}
        (out/'record.json').write_bytes(encoded({'binding': binding, 'seconds': time.perf_counter()-start, 'files': files}))
    record = json.loads((out/'record.json').read_text())
    if any(digest(out/name) != checksum for name, checksum in record['files'].items()):
        raise PreparationError('The prepared smile cache failed checksum validation.')
    print(json.dumps({'prepared': str(out), 'archive': db.archive_tree('smile-reference-26/prepared', out)}), flush=True)
    if args.install:
        # Caller must inspect the composition, independently of topology gates.
        inspection = json.loads((out/'visual-review.json').read_text())
        if not inspection['accepted'] or inspection['package_sha256'] != digest(out/'expressions.json'):
            raise PreparationError('Inspect the registered composition before installation.')
        args.install.mkdir(parents=True, exist_ok=True)
        for name in ('expressions.json', 'field.png', 'smile-material.png'):
            shutil.copyfile(out/name, args.install/name)


if __name__ == '__main__':
    main()

"""Prepare a boundary laugh from cached source-bound references; no providers."""
import argparse
import json
import shutil
from pathlib import Path
import numpy as np
from PIL import Image

from app.core.paths import ROOT
from app.preparation.eyes import align_eyes
from app.preparation.laughter import eye_closure, prepare_boundary, prepare_soft_laughter
from app.workflows.motion.prepare_motion import digest


def run(manifest, expressions, cached_landmarks, output):
    if output.exists():
        raise ValueError('Preserve earlier candidates; choose a new output directory.')
    settings = json.loads(manifest.read_bytes())
    resolve = lambda key: ROOT/settings[key]
    package = json.loads((expressions/'expressions.json').read_bytes())
    original = Image.open(resolve('original')).convert('RGB')
    points = np.asarray(json.loads(resolve('landmarks').read_bytes())['landmarks_px'])
    oral = json.loads(resolve('oral').read_bytes())
    if package['source_sha256'] != digest(resolve('original')) or oral['source_sha256'] != package['source_sha256']:
        raise ValueError('Immutable portrait bindings differ.')
    if package['joint_articulation']['laugh']['reference_sha256'] != digest(resolve('laugh')):
        raise ValueError('Cached laughter reference differs from the prepared binding.')
    generated = np.asarray(json.loads(cached_landmarks.read_bytes())['landmarks_px'])
    aligned, target, registration = align_eyes(original, Image.open(resolve('laugh')).convert('RGB'), points, generated, original.width/Image.open(resolve('laugh')).width)
    shutil.copytree(expressions, output)
    out = output/'laugh'
    out.mkdir(exist_ok=True)
    shutil.copy2(cached_landmarks, out/'landmarks.json')
    registered_dir = resolve('expressions')
    registered = json.loads((registered_dir/'expressions.json').read_bytes())
    if registered['source_sha256'] != package['source_sha256']:
        raise ValueError('Resting expression belongs to another portrait.')
    soft_target, frames, lip = prepare_soft_laughter(oral, points, target, registered, registered_dir,
        Image.open(resolve('foreground')), output)
    report = {'method':'preserved-registered-smile-field/1','validation':registered['validation']}
    joint = prepare_boundary(oral, package['joint_articulation'], points, soft_target)
    joint['lip_material'] = lip
    joint['lip_material']['sha256'] = digest(output/'laugh-lips.png')
    joint.update(registration=registration, reference_sha256=digest(resolve('laugh')), eye_closure=eye_closure(points, target),
                 field_preparation=report, review_status='composition-review-required')
    package['joint_articulation']['laugh'] = joint
    package['joint_articulation']['validation']['boundary_laugh'] = joint['validation']
    for frame in frames: frame['sha256'] = digest(output/frame['file'])
    joint['skin_frames'] = frames
    (output/'expressions.json').write_text(json.dumps(package, indent=2)+'\n', encoding='utf-8')
    (output/'laughter-preparation.json').write_text(json.dumps({'version':'boundary-laughter-preparation/1',
        'inputs':{key:digest(resolve(key)) for key in ('original','landmarks','oral','foreground','laugh')},
        'cached_landmarks_sha256':digest(cached_landmarks), 'provider_calls':0,
        'original_manual_smile_material_sha256':digest(expressions/'smile-material.png'),
        'manual_smile_preserved':digest(output/'smile-material.png')==digest(expressions/'smile-material.png'),
        'owner_acceptance':False, 'review_status':'composition-review-required'},indent=2)+'\n',encoding='utf-8')
    print(json.dumps({'output':str(output), 'field':report, 'oral_reduction':joint['amplitude_reduction'], 'provider_calls':0}))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, required=True)
    parser.add_argument('--expressions', type=Path, required=True)
    parser.add_argument('--cached-landmarks', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    run(args.manifest.resolve(), args.expressions.resolve(), args.cached_landmarks.resolve(), args.output.resolve())

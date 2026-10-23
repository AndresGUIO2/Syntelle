"""Prepare reusable full-face reference laughter from immutable cached inputs."""
from __future__ import annotations

import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np
from PIL import Image

from app.core.paths import ROOT
from app.preparation.eyes import align_eyes
from app.preparation.faces import PreparationError
from app.preparation.reference_laughter import prepare_frames


def digest(path):
    return sha256(path.read_bytes()).hexdigest()


def run(manifest, expressions, cached_landmarks, dentition, output):
    if output.exists():
        raise PreparationError('Preserve previous candidates; choose a fresh output directory.')
    config=json.loads(manifest.read_bytes())
    resolve=lambda key:(ROOT/config[key]).resolve()
    original=resolve('original');identity=digest(original)
    motion=json.loads(resolve('motion').read_bytes())
    existing=json.loads((expressions/'expressions.json').read_bytes())
    oral=json.loads((dentition/'oral/package.json').read_bytes())
    material=json.loads((dentition/'dental-material.json').read_bytes())
    for name,received in [('motion',motion['original_sha256']),('expressions',existing['source_sha256']),('oral',oral['source_sha256'])]:
        if received!=identity:
            raise PreparationError(f'{name} belongs to a different immutable portrait.')
    if material.get('reference_sha256')!=digest(resolve('laugh')):
        raise PreparationError('Dental reference does not match the laughing face.')
    if material.get('reference_landmarks_sha256')!=digest(cached_landmarks):
        raise PreparationError('Dental and facial preparation use different reference landmarks.')
    points=np.asarray(json.loads(resolve('landmarks').read_bytes())['landmarks_px'])[:,:2]
    generated=np.asarray(json.loads(cached_landmarks.read_bytes())['landmarks_px'])[:,:2]
    source=Image.open(original).convert('RGB');reference=Image.open(resolve('laugh')).convert('RGB')
    aligned,target,registration=align_eyes(source,reference,points,generated,source.width/reference.width)
    shutil.copytree(expressions,output)
    aligned.save(output/'registered-reference.png')
    masks=[Image.open(dentition/name).convert('L') for name in ('upper-mask.png','lower-mask.png')]
    extension=prepare_frames(source,Image.open(resolve('foreground')),aligned,points,target,motion,oral,masks,output)
    for row in ('upper','lower'):
        name=f'reference-{row}.webp'
        shutil.copyfile(dentition/f'{row}-fragment.webp',output/name)
        extension['oral']['observed_'+row]={'file':name}
    for entry in [*extension['frames'],extension['oral']['cavity'],extension['oral']['observed_upper'],extension['oral']['observed_lower']]:
        entry['sha256']=digest(output/entry['file'])
    extension.update(source_sha256=identity,reference_sha256=digest(resolve('laugh')),
                     motion_build_id=motion['build_id'],oral_build_id=oral['build_id'],
                     registration=registration,dental_ready=bool(material.get('ready',False) and material.get('speech_validation',{}).get('ready',False)),
                     dental_material_sha256=digest(dentition/'dental-material.json'),
                     review_status='composition-review-required')
    existing['reference_laughter']=extension
    existing['oral_build_id']=oral['build_id']
    (output/'expressions.json').write_text(json.dumps(existing,indent=2)+'\n',encoding='utf-8')
    before={name:digest(expressions/name) for name in ('field.png','smile-material.png')}
    if any(digest(output/name)!=checksum for name,checksum in before.items()):
        raise PreparationError('Manual smile bytes changed during reference preparation.')
    report={'version':'reference-laughter-preparation/1','source_sha256':identity,
            'reference_sha256':digest(resolve('laugh')),'manifest_sha256':digest(manifest),
            'cached_landmarks_sha256':digest(cached_landmarks),'manual_resources':before,
            'paid_provider_calls':0,'owner_acceptance':False,'dental_ready':extension['dental_ready'],
            'validation':extension['validation']}
    (output/'reference-preparation.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')
    return report


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ('manifest','expressions','cached-landmarks','dentition','output'):
        parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args()
    try:
        print(json.dumps(run(args.manifest.resolve(),args.expressions.resolve(),args.cached_landmarks.resolve(),args.dentition.resolve(),args.output.resolve())))
    except PreparationError as error:
        raise SystemExit(str(error)) from None

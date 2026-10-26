"""Prepare emotional controls from immutable cached anatomy, without providers."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np

from app.preparation.archive import encoded
from app.preparation.faces import PreparationError
from app.preparation.emotion_rig import STATES, prepare_emotion_rig
from app.workflows.rendering.export_native import resource_paths, validate_bindings


def prepare(bundle, landmarks, output, *, refine_states=None):
    if output.exists():
        raise PreparationError('Preserve previous preparations; choose a fresh directory.')
    folders, specification = resource_paths(bundle, None)
    names = {'motion':'package.json','oral':'package.json','gaze':'gaze.json','expressions':'expressions.json'}
    packages = {key:json.loads((folders[key]/name).read_bytes()) for key,name in names.items()}
    validate_bindings(packages,specification)
    points = np.asarray(json.loads(landmarks.read_bytes())['landmarks_px'], dtype=float)
    if points.shape != (478, 3) or not np.isfinite(points).all():
        raise PreparationError('Finite cached refined landmarks are required.')
    anatomy = packages['motion']['anatomy']
    measured = (points[152, 1], np.linalg.norm(points[234, :2]-points[454, :2]), points[152, 1]-points[10, 1])
    recorded = (anatomy['chin_y'], anatomy['face_width'], anatomy['face_height'])
    if not np.allclose(measured, recorded, rtol=0, atol=1e-3):
        raise PreparationError('Cached landmarks differ from the source-bound motion anatomy.')
    result = dict(specification)
    shutil.copytree(folders['expressions'],output/'expressions')
    result['expressions'] = (output/'expressions').resolve().as_posix()
    ex = packages['expressions']
    motion = packages['motion']
    shutil.copy2(landmarks,output/'landmarks.json')
    shutil.copy2(bundle,output/'input-bundle.json')
    source_root=Path(__file__).resolve().parents[2]
    for relative in ('preparation/emotion_rig.py','preparation/emotion_articulation.py','preparation/emotion_calibration.py','preparation/expressions.py','workflows/motion/prepare_emotions.py'):
        target=output/'sources'/relative;target.parent.mkdir(parents=True,exist_ok=True)
        shutil.copy2(source_root/relative,target)
    try:
        rig = prepare_emotion_rig(points,(motion['canvas']['width'],motion['canvas']['height']),
                                 packages['oral'],folders['oral'],ex,output/'expressions',refine_states=refine_states)
    except Exception as exc:
        (output/'failure.json').write_bytes(encoded({'error':str(exc),'provider_requests':0,'qualified':False}))
        raise
    ex['emotion_rig'] = rig
    (output/'expressions/expressions.json').write_bytes(encoded(ex))
    (output/'resource-bundle.json').write_bytes(encoded(result))
    (output/'preparation.json').write_bytes(encoded({'version':'emotion-preparation/1','rig':rig,
        'input_bundle_sha256':sha256(bundle.read_bytes()).hexdigest(),
        'landmarks_sha256':sha256(landmarks.read_bytes()).hexdigest(),
        'paid_provider_requests':0,'owner_acceptance':False}))
    return rig


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle',type=Path,required=True)
    parser.add_argument('--landmarks',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--refine-state',action='append',choices=STATES,help='Recalibrate only this state and freeze existing other states; may be repeated.')
    args=parser.parse_args()
    result=prepare(args.bundle.resolve(),args.landmarks.resolve(),args.output.resolve(),refine_states=args.refine_state)
    print(json.dumps({'version':result['version'],'states':result['states'],'provider_requests':0}))

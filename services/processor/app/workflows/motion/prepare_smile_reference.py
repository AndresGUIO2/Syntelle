"""Record and generate one source-bound smile reference, with one reviewed correction."""
import argparse
import base64
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np
from PIL import Image, ImageOps
from dotenv import dotenv_values

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.eyes import face_crop
from app.preparation.faces import PreparationError
from app.preparation.mouth import export_lossless_pair
from app.preparation.recorded_provider import RecordedProvider
from app.workflows.motion.prepare_motion import NETWORK, digest
from app.workflows.visemes.viseme_trial import write_report, save_provider_image
from app.workflows.contours.gemini_contour_trial import stamp

VERSION = 'source-smile-reference/1'


def run(config_path, output, request=False, correction=None):
    config = json.loads(config_path.read_text(encoding='utf-8'))
    if config['status'] != 'owner-authorized' or config['limits'] != {'image': 2, 'analysis': 0}:
        raise PreparationError('A separately authorized two-image smile budget is required.')
    db = Archive()
    db.budget(config['budget_id'], digest(config_path), config['limits'])
    output.mkdir(parents=True, exist_ok=True)
    stage = output/('correction' if correction else 'proposal')
    stage.mkdir(exist_ok=True)
    original = ROOT/config['original']
    landmarks = ROOT/config['cached_landmarks']
    image = ImageOps.exif_transpose(Image.open(original)).convert('RGB')
    points = np.asarray(json.loads(landmarks.read_text())['landmarks_px'])
    box = face_crop(image, points)
    image.crop(box).save(stage/'face-context.png')
    shutil.copyfile(original, stage/('original'+original.suffix))
    shutil.copyfile(landmarks, stage/'landmarks.json')
    shutil.copyfile(config_path, stage/'settings.json')
    prompt = (ROOT/config['prompt']).read_text(encoding='utf-8')
    inputs = [{'type': 'image', 'mime_type': Image.open(original).get_format_mimetype(),
               'data': base64.b64encode(original.read_bytes()).decode()},
              {'type': 'image', 'mime_type': 'image/png',
               'data': base64.b64encode((stage/'face-context.png').read_bytes()).decode()}]
    if correction:
        review = json.loads(correction.read_text())
        previous = json.loads((output/'proposal/run.json').read_text())
        previous_image = output/'proposal'/previous['file']
        if review.get('accepted') is not False or review.get('raw_sha256') != digest(previous_image) or not review.get('feedback'):
            raise PreparationError('Correction requires a rejected, hash-bound visual review.')
        shutil.copyfile(correction, stage/'previous-review.json')
        prompt += '\nThe third image is the previous proposal. Correct these observed defects while keeping the original identity:\n'+review['feedback']
        inputs.append({'type': 'image', 'mime_type': Image.open(previous_image).get_format_mimetype(),
                       'data': base64.b64encode(previous_image.read_bytes()).decode()})
    payload = {'model': config['model'], 'input': [{'type': 'text', 'text': prompt}, *inputs],
               'response_format': config['response_format'], 'store': False}
    binding = sha256(encoded(payload)).hexdigest()
    ledger = stage/'run.json'
    if ledger.exists():
        cached = json.loads(ledger.read_text())
        if cached['status'] != 'received' or cached['binding'] != binding or digest(stage/cached['file']) != cached['raw_sha256']:
            raise PreparationError('This request cannot be rebound or retried. Restore its original records.')
        if not (stage/'reference.png').exists() or not (stage/'reference.webp').exists():
            export_lossless_pair(Image.open(stage/cached['file']).convert('RGB'), stage, 'reference')
        db.archive_tree(config['budget_id']+'/'+stage.name, stage)
        print(json.dumps({'cache': True, 'file': str(stage/cached['file'])}))
        return
    if not request:
        raise PreparationError('No received reference is cached; --request is required.')
    key = dotenv_values(ROOT/'.env').get('GEMINI_API_KEY', '')
    if not key:
        raise PreparationError('Configure the existing local Gemini key.')
    write_report(stage/'request.json', payload)
    (stage/'prompt.txt').write_text(prompt, encoding='utf-8')
    record = {'version': VERSION, 'status': 'started', 'binding': binding, 'started_at': stamp(),
              'original_sha256': digest(original), 'landmarks_sha256': digest(landmarks),
              'context_box': list(box), 'attempts': 1, 'settings': NETWORK,
              'model': config['model'], 'response_format': config['response_format']}
    write_report(ledger, record)
    provider = RecordedProvider(db, config['budget_id'], output, 'image')
    try:
        raw, status, duration = provider(payload, NETWORK, key, stage)
        if not 200 <= status < 300:
            raise PreparationError(f'Provider HTTP {status}; stopped without retry.')
        body = json.loads(raw)
        parts = [body['output_image']] if body.get('output_image') else [p for step in body.get('steps', []) for p in step.get('content', []) if p.get('type') == 'image' and p.get('data')]
        if len(parts) != 1:
            raise PreparationError('Expected exactly one image; preserve the received response.')
        path = save_provider_image(base64.b64decode(parts[0]['data'], validate=True), stage, 'smile')
        export_lossless_pair(Image.open(path).convert('RGB'), stage, 'reference')
        record.update(status='received', file=path.name, raw_sha256=digest(path),
                      usage=body.get('usage', {}), seconds=duration)
    except Exception as error:
        record.update(status='failed', error=str(error).replace(key, '[redacted]')[:500])
        raise
    finally:
        record['finished_at'] = stamp()
        write_report(ledger, record)
        print(json.dumps({'record': record['status'], 'archive': db.archive_tree(config['budget_id']+'/'+stage.name, stage)}), flush=True)
    print(json.dumps({'cache': False, 'file': str(path)}), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--request', action='store_true')
    parser.add_argument('--correction-review', type=Path)
    args = parser.parse_args()
    run(args.config.resolve(), args.output.resolve(), args.request, args.correction_review)

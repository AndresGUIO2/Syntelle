"""Generic, bounded semantic analysis of immutable cached character poses."""
import argparse
import base64
from hashlib import sha256
import json
from pathlib import Path

import cv2
import numpy as np
from PIL import Image, ImageOps

from app.workflows.contours.gemini_contour_loop import capture_round, checked_files
from app.preparation.faces import PreparationError
from app.preparation.mouth import transform_points
from app.preparation.semantic_mouth import OralContours, views, rasterize, validate_masks, overlay, png
from app.workflows.visemes.viseme_trial import ROOT, write_report

VERSION = 'semantic-oral/1'
SETTINGS = {'model': 'gemini-4-argon', 'endpoint': 'https://generativelanguage.googleapis.com/v1beta/interactions',
            'retries': 0, 'follow_redirects': False, 'store': False, 'timeout_seconds': 240,
            'generation_config': {'thinking_level': 'medium', 'temperature': 1, 'max_output_tokens': 24576}}


def digest(path): return sha256(path.read_bytes()).hexdigest()


def load_inputs(path):
    spec = json.loads(path.read_text(encoding='utf-8'))
    def resolve(value): return (path.parent / value).resolve()
    base_path = resolve(spec['original'])
    if digest(base_path) != spec['original_sha256']: raise PreparationError('Original portrait bytes changed.')
    base = ImageOps.exif_transpose(Image.open(base_path)).convert('RGB')
    crop = spec['crop']; x, y, w, h = (crop[k] for k in ('x', 'y', 'width', 'height'))
    if min(x,y) < 0 or w <= 0 or h <= 0 or x+w > base.width or y+h > base.height:
        raise PreparationError('Invalid registered oral coordinate frame.')
    images = {}; points = {}; provenance = {}
    for pose, item in spec['poses'].items():
        folder = resolve(item['source'])
        if pose == 'Rest':
            landmark_path = folder/'base-landmarks.json'
            image = base; pts = np.asarray(json.loads(landmark_path.read_text())['landmarks_px'])[:, :2]
            evidence = {'original_sha256': digest(base_path), 'landmarks_sha256': digest(landmark_path)}
        else:
            record = json.loads((folder/'run.json').read_text())
            raw = folder/record['generation']['file']
            if record.get('viseme') != pose or tuple(record['canvas'][k] for k in ('width','height')) != base.size:
                raise PreparationError(f'{pose}: cached pose identity or canvas does not match the input manifest.')
            request_hash=record['generation'].get('request_settings',{}).get('input_sha256')
            if request_hash is None and record.get('verification',{}).get('source_file_unchanged'):
                # Earlier saved trials predate per-request input digests. Their
                # original source binding remains explicit, never invented.
                request_hash=record['source_sha256']
            if request_hash != spec['original_sha256']:
                raise PreparationError(f'{pose}: generation did not use the immutable original bytes.')
            if record['source_sha256'] != spec['original_sha256'] or digest(raw) != record['generation']['raw_sha256']:
                raise PreparationError(f'{pose}: cached generation belongs to another original or changed.')
            matrix = np.asarray(record['metrics']['transform'],dtype=float)
            if matrix.shape!=(2,3) or not np.isfinite(matrix).all() or np.linalg.det(matrix[:,:2])<=0:
                raise PreparationError(f'{pose}: invalid stable-anchor alignment.')
            decoded = ImageOps.exif_transpose(Image.open(raw)).convert('RGB')
            if abs((decoded.width/decoded.height)/(base.width/base.height)-1)>.02:
                raise PreparationError(f'{pose}: generation changed portrait framing.')
            decoded=decoded.resize(base.size, Image.Resampling.LANCZOS)
            image = Image.fromarray(cv2.warpAffine(np.asarray(decoded), matrix, base.size, flags=cv2.INTER_LANCZOS4, borderMode=cv2.BORDER_REFLECT_101))
            landmark_path = folder/'variant-landmarks.json'
            pts = transform_points(np.asarray(json.loads(landmark_path.read_text())['landmarks_px']), matrix)
            evidence = {'original_sha256': spec['original_sha256'], 'raw_sha256': digest(raw),
                        'run_sha256': digest(folder/'run.json'), 'alignment': matrix.tolist(), 'landmarks_sha256': digest(landmark_path)}
        images[pose] = image.crop((x,y,x+w,y+h)); points[pose] = pts - [x,y]
        if points[pose].shape!=(478,2) or not np.isfinite(points[pose]).all():
            raise PreparationError(f'{pose}: incomplete or nonfinite facial geometry.')
        provenance[pose] = evidence
    return spec, base, images, points, provenance


def analyze(manifest, output, poses, request=False, correction=False, provider=None):
    spec, _, images, _, provenance = load_inputs(manifest)
    template = (ROOT/'docs/prompts/oral-semantics-v1.txt').read_text(encoding='utf-8')
    output.mkdir(parents=True, exist_ok=True)
    binding = {'manifest_sha256': digest(manifest), 'prompt_sha256': sha256(template.encode()).hexdigest(), 'settings': SETTINGS, 'version': VERSION}
    ledger = output/'batch.json'
    if ledger.exists():
        if json.loads(ledger.read_text())['binding'] != binding: raise PreparationError('Batch inputs changed; no new requests.')
    else: write_report(ledger, {'binding': binding, 'maximum_requests': 18, 'new_viseme_generations': 0})
    for pose in poses:
        if pose not in images: raise PreparationError('Unknown pose identifier.')
        directory = output/pose.lower(); directory.mkdir(exist_ok=True)
        image = images[pose]; image.save(directory/'original.png')
        write_report(directory/'source.json', provenance[pose])
        inputs = views(image)
        for name, data in inputs.items(): (directory/name).write_bytes(data)
        use_correction = correction or (directory/'correction/run.json').exists()
        stage = 'correction' if use_correction else 'proposal'
        stage_dir = directory/stage
        if not (stage_dir/'run.json').exists() and len(list(output.glob('*/proposal/run.json'))) + len(list(output.glob('*/correction/run.json'))) >= 18:
            raise PreparationError('The 18-request batch budget is exhausted.')
        feedback = ''
        if use_correction:
            previous = directory/'proposal'
            if not (previous/'run.json').exists() or json.loads((previous/'run.json').read_text())['status'] != 'received':
                raise PreparationError('A provider failure cannot be bypassed by requesting a correction.')
            feedback = 'Review the supplied overlay against the clean original. Correct ALL five regions, especially missing visible enamel or mistaken lip/opening edges. Prior result:\n' + (previous/'output.txt').read_text() + '\nLocal checks:\n' + (previous/'validation.json').read_text()
        prompt = template.replace('{{POSE}}', pose).replace('{{FEEDBACK}}', feedback)
        schema = {'type':'object','properties':{'notes':{'type':'string'},'regions':{'type':'array','items':{
            'type':'object','properties':{'label':{'type':'string','enum':list(('outer_lips','aperture','upper_teeth','lower_teeth','tongue'))},
            'visible':{'type':'boolean'},'polygons':{'type':'array','items':{'type':'array','items':{'type':'array','items':{'type':'number'}}}}},
            'required':['label','visible','polygons']}}},'required':['regions','notes']}
        payload = {'model': SETTINGS['model'], 'input': [{'type': 'text', 'text': prompt}],
                   'generation_config': SETTINGS['generation_config'], 'store': False,
                   'response_format': {'type': 'text', 'mime_type': 'application/json', 'schema': schema}}
        for data in inputs.values():
            payload['input'].append({'type':'image','mime_type':'image/png','data':base64.b64encode(data).decode(),'resolution':'high'})
        if use_correction:
            payload['input'].append({'type':'image','mime_type':'image/png','data':base64.b64encode(png(Image.open(directory/'proposal/overlay.png').resize((image.width*4,image.height*4)))).decode(),'resolution':'high'})
        # capture_round writes a durable started ledger before sending and forbids
        # retries after provider errors, interruptions or changed inputs.
        text, record = capture_round(stage_dir, payload, SETTINGS, request, **({'provider':provider} if provider else {}))
        try:
            contours = OralContours.model_validate_json(text)
            write_report(stage_dir/'contours.json', contours.model_dump())
            masks, refinements = rasterize(contours, image)
            for label, mask in masks.items(): mask.save(stage_dir/f'{label}.png')
            result = validate_masks(masks, pose); result['edge_refinement'] = refinements
            overlay(image, masks).save(stage_dir/'overlay.png')
        except (ValueError,PreparationError) as error:
            result = {'valid':False,'issues':[str(error)[:800]]}
            image.save(stage_dir/'overlay.png')
        write_report(stage_dir/'validation.json', result)
        write_report(directory/'selected.json', {'stage': stage, 'valid': result['valid'], 'source': provenance[pose],
                                                'contours_sha256': digest(stage_dir/'contours.json') if (stage_dir/'contours.json').exists() else None, 'version': VERSION})
        print(json.dumps({'pose': pose, 'stage': stage, 'validation': result, 'usage': record.get('usage')}))
        if not result['valid']:
            if not use_correction:
                analyze(manifest,output,[pose],request=request,correction=True,provider=provider)
            else:
                raise PreparationError(f'{pose}: semantic preparation failed after its single correction; evidence retained.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--poses', nargs='+', required=True)
    parser.add_argument('--request', action='store_true')
    parser.add_argument('--correction', action='store_true')
    args = parser.parse_args()
    try: analyze(args.manifest.resolve(), args.output.resolve(), args.poses, args.request, args.correction)
    except (ValueError, PreparationError) as error: raise SystemExit(str(error)) from None

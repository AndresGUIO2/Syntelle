"""Record source-bound expression references under a separate, finite allowance."""
import argparse
import base64
import json
from pathlib import Path

from dotenv import dotenv_values
from PIL import Image

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.preparation.recorded_provider import RecordedProvider
from app.workflows.motion.prepare_motion import NETWORK, digest
from app.workflows.visemes.viseme_trial import save_provider_image


def run(config_path, slot, output, review=None):
    config = json.loads(config_path.read_text(encoding='utf-8'))
    if config['status'] != 'owner-authorized' or config['limits'] != {'image': 4, 'analysis': 0}:
        raise PreparationError('A separate four-image authorization is required.')
    kind = slot.removesuffix('-correction')
    if kind not in ('smile', 'laugh') or slot not in (kind, kind+'-correction'):
        raise PreparationError('Unknown expression request slot.')
    output = output.resolve()
    stage = output/slot
    stage.mkdir(parents=True, exist_ok=True)
    ledger = stage/'run.json'
    if ledger.exists():
        record = json.loads(ledger.read_text())
        if not (stage/'settings.json').exists() or (stage/'settings.json').read_bytes() != config_path.read_bytes():
            raise PreparationError('Cached reference settings changed; restore the original binding.')
        source = Path(config['original'])
        if not source.is_absolute():
            source = ROOT/source
        if digest(source) != record['original_sha256']:
            raise PreparationError('Cached reference source changed; restore the original bytes.')
        if record['status'] != 'received' or digest(stage/record['file']) != record['sha256']:
            raise PreparationError('Stopped or invalid cached request; do not retry.')
        print(json.dumps({'cached': True, 'slot': slot, 'file': str(stage/record['file'])}))
        return
    # A failed slot stops the entire experiment, including other expression slots.
    for previous in output.glob('*/run.json'):
        if json.loads(previous.read_text())['status'] != 'received':
            raise PreparationError('An earlier request stopped; no further provider calls allowed.')
    original = Path(config['original'])
    if not original.is_absolute():
        original = ROOT/original
    prompt = config['prompts'][kind]
    inputs = [{'type': 'image', 'mime_type': Image.open(original).get_format_mimetype(),
               'data': base64.b64encode(original.read_bytes()).decode()}]
    if slot.endswith('-correction'):
        if review is None:
            raise PreparationError('A correction requires a hash-bound rejected review.')
        proposal = json.loads((output/kind/'run.json').read_text())
        reference = output/kind/proposal['file']
        decision = json.loads(review.read_text())
        if decision.get('accepted') is not False or decision.get('sha256') != digest(reference) or not decision.get('feedback'):
            raise PreparationError('Correction review does not bind a rejected reference.')
        (stage/'review.json').write_bytes(review.read_bytes())
        prompt += '\nCorrect the previous proposal using this review: '+decision['feedback']
        inputs.append({'type': 'image', 'mime_type': Image.open(reference).get_format_mimetype(),
                       'data': base64.b64encode(reference.read_bytes()).decode()})
    key = dotenv_values(ROOT/'.env').get('GEMINI_API_KEY', '')
    if not key:
        raise PreparationError('Configure the existing local Gemini key.')
    db = Archive()
    db.budget(config['budget_id'], digest(config_path), config['limits'])
    (stage/('original'+original.suffix)).write_bytes(original.read_bytes())
    (stage/'settings.json').write_bytes(config_path.read_bytes())
    (stage/'prompt.txt').write_text(prompt, encoding='utf-8')
    payload = {'model': config['model'], 'input': [{'type': 'text', 'text': prompt}, *inputs],
               'response_format': config['response_format']}
    (stage/'request.json').write_bytes(encoded(payload))
    record = {'status': 'started', 'slot': slot, 'original_sha256': digest(original)}
    ledger.write_bytes(encoded(record))
    try:
        raw, status, duration = RecordedProvider(db, config['budget_id'], output, 'image')(payload, NETWORK, key, stage)
        if not 200 <= status < 300:
            raise PreparationError(f'Provider HTTP {status}; stopped without retry.')
        body = json.loads(raw)
        parts = [body['output_image']] if body.get('output_image') else [p for s in body.get('steps', []) for p in s.get('content', []) if p.get('type') == 'image' and p.get('data')]
        if len(parts) != 1:
            raise PreparationError('Expected exactly one reference; inspect preserved response.')
        path = save_provider_image(base64.b64decode(parts[0]['data'], validate=True), stage, kind)
        record.update(status='received', file=path.name, sha256=digest(path), seconds=duration, usage=body.get('usage', {}))
    except Exception as error:
        record.update(status='failed', error=str(error).replace(key, '[redacted]')[:500])
        raise
    finally:
        ledger.write_bytes(encoded(record))
        db.archive_tree(config['budget_id']+'/'+slot, stage)
    print(json.dumps({'slot': slot, 'file': str(path)}), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', type=Path, required=True)
    parser.add_argument('--slot', choices=['smile', 'laugh', 'smile-correction', 'laugh-correction'], required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--review', type=Path)
    args = parser.parse_args()
    run(args.config.resolve(), args.slot, args.output, args.review)

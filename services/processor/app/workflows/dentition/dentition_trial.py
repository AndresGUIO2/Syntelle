"""One explicitly authorized dental plate request; cached, local and never retried."""
import argparse
import json
import os
from datetime import datetime, timezone
from hashlib import sha256
from pathlib import Path

from dotenv import dotenv_values
from app.preparation.faces import PreparationError
from app.preparation.gemini import DEFAULT_MODEL, REQUEST_SETTINGS, generate_a
from app.workflows.visemes.viseme_trial import ROOT, save_provider_image, write_report


def prepare(input_path: Path, reference: Path, output: Path, generate: bool = False):
    original = input_path.read_bytes()
    detail = (reference / 'mouth-a.png').read_bytes()
    source_hash = sha256(original).hexdigest()
    if json.loads((reference / 'run.json').read_text())['source_sha256'] != source_hash:
        raise PreparationError('Approved A reference belongs to a different original.')
    prompt = (ROOT / 'docs/prompts/dentition-v1.txt').read_text(encoding='utf-8')
    prompt_hash = sha256(prompt.encode()).hexdigest()
    output.mkdir(parents=True, exist_ok=True)
    record_path = output / 'run.json'
    if record_path.exists():
        record = json.loads(record_path.read_text())
        if (record['source_sha256'] != source_hash or record['prompt_sha256'] != prompt_hash
                or record['supporting_image']['sha256'] != sha256(detail).hexdigest()
                or sha256((output / 'prompt.txt').read_bytes()).hexdigest() != prompt_hash):
            raise PreparationError('Cached dental request inputs changed; inspect the existing evidence.')
        raw = output / record.get('file', 'missing-output')
        if raw.exists() and sha256(raw.read_bytes()).hexdigest() == record.get('raw_sha256'):
            print('Reusing original dental plate. No provider request.')
            return record
        raise PreparationError('Previous dental request is incomplete or cache changed. No retry is allowed here.')
    if not generate:
        raise PreparationError('No cached dental plate; --generate makes exactly one authorized request.')
    values = dotenv_values(ROOT / '.env')
    key = os.environ.get('GEMINI_API_KEY') or values.get('GEMINI_API_KEY') or ''
    if not key.strip():
        raise PreparationError('Set GEMINI_API_KEY in local secret configuration.')
    model = os.environ.get('GEMINI_IMAGE_MODEL') or values.get('GEMINI_IMAGE_MODEL') or DEFAULT_MODEL
    (output / 'prompt.txt').write_bytes(prompt.encode())
    record = {'schema': 'syntelle-dental-request/1', 'pipeline_version': 'dentition-plate/1',
              'source_sha256': source_hash, 'prompt_sha256': prompt_hash, 'model': model,
              'supporting_image': {'role': 'approved A mouth enamel reference', 'sha256': sha256(detail).hexdigest()},
              'settings': {'response_format': REQUEST_SETTINGS.copy(), 'store': False, 'retries': 0, 'timeout_seconds': 180},
              'status': 'request_started', 'attempts': 1, 'started_at': datetime.now(timezone.utc).isoformat()}
    write_report(record_path, record)
    try:
        raw, metadata = generate_a(original, prompt, key, model, supporting_images=((detail, 'image/png'),))
        # Save paid response before any local inspection or segmentation.
        saved = save_provider_image(raw, output, 'dentition')
        record.update({'status': 'received', 'file': saved.name, 'raw_sha256': sha256(raw).hexdigest(), 'generation': metadata})
    except PreparationError as error:
        record.update({'status': 'failed', 'error': str(error)})
        raise
    finally:
        record['finished_at'] = datetime.now(timezone.utc).isoformat()
        write_report(record_path, record)
    if sha256(input_path.read_bytes()).hexdigest() != source_hash:
        raise PreparationError('Original input changed during generation.')
    print('Dental plate saved. One request; viseme caches unchanged.')
    return record


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--reference', type=Path, default=ROOT / '.tools/viseme-a-05')
    parser.add_argument('--output', type=Path, default=ROOT / '.tools/dentition-01')
    parser.add_argument('--generate', action='store_true')
    args = parser.parse_args()
    prepare(args.input, args.reference, args.output, args.generate)

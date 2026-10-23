"""Run one fixed, PostgreSQL-reserved provider probe; never retry or fall back."""
import argparse
import base64
import json
import subprocess
from urllib.parse import urlparse, urlunparse

from dotenv import dotenv_values

from app.core.paths import ROOT
from app.preparation.archive import Archive
from app.preparation.faces import PreparationError

IDENTITY = 'authorized-character-performance-2026-10-20'
SLOTS = ('gemini-en', 'gemini-es', 'tts-en', 'tts-es')


def run(slot: str):
    if slot not in SLOTS:
        raise PreparationError('Unknown fixed probe slot.')
    output = ROOT/'.tools/character-performance-34'
    output.mkdir(parents=True, exist_ok=True)
    settings = json.loads((ROOT/'docs/characters/performance-experiment.json').read_text())
    if settings['id'] != IDENTITY or settings['limits'] != {'image': 2, 'tts': 2, 'gemini': 2, 'analysis': 0}:
        raise PreparationError('Experiment identity or ceiling changed.')
    db = Archive()
    db.event(IDENTITY, 'authorization', settings)
    values = dotenv_values(ROOT/'.env')
    if values.get('AZURE_SPEECH_REGION') != 'canadacentral':
        raise PreparationError('Use the confirmed personal Canada Central resource.')
    parsed = urlparse(values['SYNTELLE_DATABASE_URL'])
    # The subprocess is inside the existing Compose network, not the host.
    internal = parsed._replace(netloc=f'{parsed.username}:{parsed.password}@postgres:5432')
    result = subprocess.run([
        'docker', 'run', '--rm', '--network', 'syntelle_default',
        '--env-file', str(ROOT/'.env'), '--env', 'SYNTELLE_DATABASE_URL='+urlunparse(internal),
        'syntelle-performance-build', 'go', 'run', '-tags', 'speech', './cmd/performance-probe', '-slot', slot,
    ], capture_output=True, cwd=ROOT, timeout=120)
    # Provider error bodies are already redacted by the Go adapter. Never print
    # the command/environment or a subprocess exception containing credentials.
    for suffix, data in [('json', result.stdout), ('log', result.stderr)]:
        db.checked(data)
        target = output/f'{slot}.{suffix}'
        if target.exists() and target.read_bytes() != data:
            raise PreparationError('Preserve the existing probe output; no overwrite.')
        target.write_bytes(data)
    if result.stdout.strip():
        record = json.loads(result.stdout)
        if record.get('audio_base64'):
            audio = base64.b64decode(record['audio_base64'], validate=True)
            (output/f'{slot}.wav').write_bytes(audio)
    snapshot = db.archive_tree(IDENTITY+'/probes', output)
    print(json.dumps({'slot': slot, 'success': result.returncode == 0, **snapshot}))
    if result.returncode:
        raise PreparationError('Provider probe failed. All further paid work is stopped; inspect the preserved evidence.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('slot', choices=SLOTS)
    run(parser.parse_args().slot)

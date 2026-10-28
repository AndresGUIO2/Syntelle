"""Qualify bounded Serena speech using PostgreSQL records; no image requests."""
import argparse
import base64
import hashlib
import json
import os
import io
import math
import wave
import subprocess
from urllib.parse import urlparse, urlunparse
from urllib.request import Request, urlopen

from dotenv import dotenv_values

from app.core.paths import ROOT
from app.preparation.archive import Archive
from app.preparation.faces import PreparationError

IDENTITY = 'authorized-coordinated-serena-2026-10-27'
VOICE = 'en-US-Serena:DragonHDLatestNeural'
OUTPUT = ROOT / '.tools/coordinated-performance-103'
SLOTS = tuple(f'{purpose}-{language}-{mode}' for purpose in ('reassure', 'ask', 'playful') for language in ('en', 'es') for mode in ('plain', 'directed')) + ('laughter-en', 'laughter-es')


def catalog():
    values = dotenv_values(ROOT / '.env')
    if values.get('AZURE_SPEECH_REGION') != 'canadacentral':
        raise PreparationError('Use the selected personal Canada Central resource.')
    folder = OUTPUT / 'capabilities'
    folder.mkdir(parents=True, exist_ok=True)
    attempt = len(tuple(folder.glob('catalog-read-*.json'))) + 1
    receipt = {'region': 'canadacentral', 'voice': VOICE, 'status': 'unavailable', 'synthesis_attempts': 0}
    try:
        # Match the installed local API's DNS configuration; do not alter host DNS.
        reader = "import os,sys;from urllib.request import Request,urlopen;sys.stdout.buffer.write(urlopen(Request('https://canadacentral.tts.speech.microsoft.com/cognitiveservices/voices/list',headers={'Ocp-Apim-Subscription-Key':os.environ['AZURE_SPEECH_KEY']}),timeout=30).read())"
        fetched = subprocess.run(['docker','run','--rm','--pull','never','--network','syntelle_default','--dns','1.1.1.1','--dns','8.8.8.8','--env','AZURE_SPEECH_KEY','syntelle-processor','python','-c',reader],env={**os.environ,'AZURE_SPEECH_KEY':values['AZURE_SPEECH_KEY']},capture_output=True,timeout=45)
        if fetched.returncode:
            raise PreparationError('Regional catalog transport failed.')
        raw = fetched.stdout
        selected = [v for v in json.loads(raw) if v.get('ShortName') == VOICE]
        (folder / f'catalog-read-{attempt}-voices.json').write_bytes(raw)
        receipt['catalog_sha256'] = hashlib.sha256(raw).hexdigest()
        receipt['selected'] = selected
        if selected:
            v = selected[0]
            languages = {v.get('Locale'), *v.get('SecondaryLocaleList', [])}
            receipt['catalog_languages'] = sorted(languages.intersection({'en-US', 'es-CO'}))
            receipt['probe_languages'] = ['en-US', 'es-CO']
            receipt['status'] = 'available'
    except Exception as error:
        receipt['failure'] = type(error).__name__
    target = folder / f'catalog-read-{attempt}.json'
    target.write_text(json.dumps(receipt, indent=2), encoding='utf-8')
    if receipt['status'] == 'available':
        (folder / 'regional-availability.json').write_bytes(target.read_bytes())
    db = Archive()
    db.event(IDENTITY, 'regional-catalog-read', receipt)
    snapshot = db.archive_tree(IDENTITY + '/catalog', folder)
    print(json.dumps({'status': receipt['status'], 'read_only': True, **snapshot}))
    if receipt['status'] != 'available':
        raise PreparationError('Serena regional availability was not verified; no synthesis was requested.')


def run(slot: str):
    values = dotenv_values(ROOT / '.env')
    settings = json.loads((ROOT / 'docs/characters/coordinated-serena-experiment.json').read_bytes())
    if settings['id'] != IDENTITY or settings['limits'] != {'tts': 14, 'characters': 4000, 'image': 0, 'analysis': 0}:
        raise PreparationError('Preserve the authorized experiment identity and ceilings.')
    receipt = OUTPUT / 'capabilities/regional-availability.json'
    if not receipt.exists():
        raise PreparationError('A verified regional catalog is required before paid synthesis.')
    if slot.endswith('-es-directed'):
        raise PreparationError('Spanish explicit styles are skipped before any provider request.')
    db = Archive()
    db.event(IDENTITY, 'authorization', settings)
    db.archive_tree(IDENTITY + '/inputs', OUTPUT / 'capabilities', {'slot': slot, 'settings': settings})
    parsed = urlparse(values['SYNTELLE_DATABASE_URL'])
    internal = parsed._replace(netloc=parsed.netloc.rsplit('@', 1)[0] + '@postgres:5432')
    env = {**os.environ, 'AZURE_SPEECH_KEY': values['AZURE_SPEECH_KEY'], 'AZURE_SPEECH_REGION': values['AZURE_SPEECH_REGION'], 'SYNTELLE_DATABASE_URL': urlunparse(internal)}
    result = subprocess.run(['docker', 'run', '--rm', '--pull','never','--network', 'syntelle_default', '--dns','1.1.1.1','--dns','8.8.8.8','--env', 'AZURE_SPEECH_KEY', '--env', 'AZURE_SPEECH_REGION', '--env', 'SYNTELLE_DATABASE_URL', '--mount', f'type=bind,source={OUTPUT.as_posix()},target=/experiment,readonly', 'syntelle-serena-probe:103', '-slot', slot, '-catalog', '/experiment/capabilities/regional-availability.json'], capture_output=True, env=env, timeout=100)
    folder = OUTPUT / 'speech' / slot
    folder.mkdir(parents=True, exist_ok=True)
    for name, content in [('response.json', result.stdout), ('provider.log', result.stderr)]:
        db.checked(content)
        target = folder / name
        if target.exists() and target.read_bytes() != content:
            raise PreparationError('Preserve existing results; no overwrite or provider retry.')
        target.write_bytes(content)
    if result.stdout.strip():
        record = json.loads(result.stdout)
        if record.get('audio_base64'):
            (folder / 'audio.wav').write_bytes(base64.b64decode(record['audio_base64'], validate=True))
    archived = db.archive_tree(IDENTITY + '/' + slot, folder)
    print(json.dumps({'slot': slot, 'success': result.returncode == 0, **archived}))
    if result.returncode:
        raise PreparationError('Qualification stopped; inspect the preserved result without retrying.')


def checked_record(record, language, *, voice=None):
    if record.get('voice') != (voice or VOICE) or record.get('language') != language:
        raise PreparationError('A cached voice/language must never be relabeled.')
    audio = base64.b64decode(record['audio_base64'], validate=True)
    with wave.open(io.BytesIO(audio)) as wav:
        if (wav.getnchannels(), wav.getsampwidth(), wav.getframerate()) != (1, 2, 24000):
            raise PreparationError('Expected original mono PCM16 24 kHz audio.')
        duration = wav.getnframes() / 24
    if not math.isfinite(record['duration_ms']) or duration<=0 or abs(duration - record['duration_ms']) > 2:
        raise PreparationError('Native duration does not match the original audio.')
    previous = -1
    events = record.get('visemes', [])
    if len(events) < 2:
        raise PreparationError('Missing native articulation events.')
    for event in events:
        offset = event['offset_ms']
        if not math.isfinite(offset) or offset < 0 or offset < previous or offset > duration + 250 or type(event['id']) is not int or not 0 <= event['id'] <= 21:
            raise PreparationError('Invalid native articulation timeline.')
        previous = offset
    return hashlib.sha256(audio).hexdigest(), duration


def prepare():
    """Export a technical local-review profile; never activate or call providers."""
    base = json.loads((ROOT / '.tools/paired-parallax-100/active-profile/profile.json').read_bytes())
    profile = {**base, 'voice': VOICE, 'voice_review': 'owner-pending'}
    profile['laughter'] = dict(base.get('laughter', {}))
    locales, evidence = {}, {}
    for suffix, language in [('en', 'en-US'), ('es', 'es-CO')]:
        available = True
        styles = []
        for purpose, style in [('reassure', 'reassuring'), ('ask', 'curious'), ('playful', 'amused')]:
            path = OUTPUT / 'speech' / f'{purpose}-{suffix}-plain' / 'response.json'
            if not path.exists():
                available = False
                continue
            digest, _ = checked_record(json.loads(path.read_bytes()), language)
            evidence[f'{purpose}-{suffix}-plain'] = digest
            directed = OUTPUT / 'speech' / f'{purpose}-{suffix}-directed' / 'response.json'
            if directed.exists() and directed.read_bytes().strip():
                digest, _ = checked_record(json.loads(directed.read_bytes()), language)
                evidence[f'{purpose}-{suffix}-directed'] = digest
                styles.append(style)
        laughter = OUTPUT / 'speech' / f'laughter-{suffix}' / 'response.json'
        # Re-evaluate cached nonverbal output with the nonverbal gate; preserve
        # the original failed speech-validation record and never re-request it.
        rejected = laughter.with_name('rejected-result.json')
        if (not laughter.exists() or not laughter.read_bytes().strip()) and rejected.exists():
            cached = json.loads(rejected.read_bytes())
            if cached.get('provider_status') == 'SynthesizingAudioCompleted':
                checked_record(cached, language)
                laughter = rejected
        if laughter.exists() and laughter.read_bytes().strip():
            record = json.loads(laughter.read_bytes())
            digest, duration = checked_record(record, language)
            evidence[f'laughter-{suffix}'] = digest
            if duration <= 2500:
                profile['laughter'][VOICE + '|' + language] = record
        locales[language] = {'available': available, 'styles': styles if available else []}
    if not all(v['available'] for v in locales.values()):
        raise PreparationError('Complete native Serena controls in both languages are required; no candidate was exported.')
    revision = hashlib.sha256(json.dumps(evidence, sort_keys=True).encode()).hexdigest()
    capabilities = {'version': 'voice-capabilities/1', 'revision': revision, 'voice': VOICE, 'region': 'canadacentral', 'locales': locales, 'qualification': 'technical-local-review', 'audible_owner_review': 'pending', 'audio_sha256': evidence}
    folder = OUTPUT / 'active-profile-v2'
    folder.mkdir(exist_ok=True)
    for name, value in [('profile.json', profile), ('voice-capabilities.json', capabilities)]:
        content = json.dumps(value, indent=2).encode()
        target = folder / name
        if target.exists() and target.read_bytes() != content:
            raise PreparationError('Preserve the existing profile; export a separately versioned candidate.')
        target.write_bytes(content)
    archived = Archive().archive_tree(IDENTITY + '/technical-profile', folder)
    print(json.dumps({'exported': True, 'activated': False, 'owner_review': 'pending', **archived}))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    choice = parser.add_mutually_exclusive_group(required=True)
    choice.add_argument('--catalog', action='store_true')
    choice.add_argument('--prepare', action='store_true')
    choice.add_argument('--slot', choices=SLOTS)
    args = parser.parse_args()
    catalog() if args.catalog else prepare() if args.prepare else run(args.slot)

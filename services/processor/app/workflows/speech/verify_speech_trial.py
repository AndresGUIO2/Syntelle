"""Explicit, bounded live Dragon HD verification; no automatic transport retries."""
import argparse
import base64
from datetime import datetime, timezone
import io
import json
from pathlib import Path
import time
import urllib.error
import urllib.request
import wave

from dotenv import dotenv_values
import psycopg
from psycopg.rows import dict_row

from app.preparation.archive import Archive
from app.workflows.visemes.viseme_trial import ROOT

CASES = {
    'ava-en': ('en-US-Ava:DragonHDLatestNeural', 'en-US', 'Hello. My voice and facial movements share the same clock.'),
    'ava-es': ('en-US-Ava:DragonHDLatestNeural', 'es-CO', 'Hola. Mi voz y mis movimientos faciales siguen el mismo reloj.'),
    'andrew-en': ('en-US-Andrew:DragonHDLatestNeural', 'en-US', 'Hello. I can speak, pause, and continue with natural movements.'),
    'andrew-es': ('en-US-Andrew:DragonHDLatestNeural', 'es-CO', 'Hola. Puedo hablar, hacer una pausa y continuar con movimientos naturales.'),
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--case', choices=CASES, default='ava-en')
    args = parser.parse_args()
    voice, language, text = CASES[args.case]
    folder = ROOT / '.tools/head-motion-12/speech' / args.case
    folder.mkdir(parents=True, exist_ok=True)
    if (folder / 'sent.json').exists():
        raise SystemExit('This verification was already sent. Inspect its recorded result; no automatic retry.')
    config = dotenv_values(ROOT / '.env')
    db = Archive(dsn=config['SYNTELLE_DATABASE_URL'])
    request = {'text': text, 'voice': voice, 'language': language,
               'request_key': 'head-motion12-' + args.case + '-v1'}
    (folder / 'request.json').write_text(json.dumps(request, indent=2), encoding='utf-8')
    (folder / 'settings.json').write_text(json.dumps({'region': config['AZURE_SPEECH_REGION'],
        'endpoint': 'http://127.0.0.1:8082/api/v1/speech/synthesize', 'transport_retries': 0,
        'format': 'riff-24khz-16bit-mono-pcm', 'sdk_version': '1.52.0'}, indent=2))
    # The API separately commits its transactional budget reservation before Azure.
    db.archive_tree('head-motion-12/speech/' + args.case, folder)
    (folder / 'sent.json').write_text(json.dumps({'sent_utc': datetime.now(timezone.utc).isoformat()}))
    started = time.perf_counter()
    status, body = 0, b''
    try:
        call = urllib.request.Request('http://127.0.0.1:8082/api/v1/speech/synthesize',
            data=json.dumps(request).encode(), headers={'Content-Type': 'application/json'})
        with urllib.request.urlopen(call, timeout=35) as response:
            status, body = response.status, response.read()
    except urllib.error.HTTPError as error:
        status, body = error.code, error.read()
    except (OSError, TimeoutError) as error:
        body = json.dumps({'local_transport_error': type(error).__name__}).encode()
    (folder / 'response.json').write_bytes(body)
    summary = {'status': status, 'elapsed_ms': round((time.perf_counter()-started)*1000, 2),
               'case': args.case, 'voice': voice, 'language': language, 'native_visemes_verified': False}
    try:
        result = json.loads(body)
        if status == 200:
            audio = base64.b64decode(result['audio_base64'])
            (folder / 'audio.wav').write_bytes(audio)
            (folder / 'visemes.json').write_text(json.dumps(result['visemes'], indent=2))
            with wave.open(io.BytesIO(audio)) as wav:
                assert (wav.getnchannels(), wav.getsampwidth(), wav.getframerate()) == (1, 2, 24000)
                duration = wav.getnframes() / wav.getframerate() * 1000
            assert abs(duration-result['duration_ms']) < 100
            events = result['visemes']
            assert len(events) >= 2 and any(e['id'] > 0 for e in events)
            assert all(0 <= e['id'] <= 21 and 0 <= e['offset_ms'] <= duration+100 for e in events)
            assert all(a['offset_ms'] <= b['offset_ms'] for a,b in zip(events,events[1:]))
            summary.update(native_visemes_verified=True, duration_ms=duration,
                events=len(events), ids=sorted({e['id'] for e in events}), cached=result['cached'])
        with psycopg.connect(config['SYNTELLE_DATABASE_URL'], row_factory=dict_row) as conn:
            row = conn.execute('SELECT * FROM syntelle_speech.requests WHERE request_key=%s', (request['request_key'],)).fetchone()
            assert row is not None
            if status == 200:
                assert row['status'] == 'received' and bytes(row['audio']) == audio
            if row['audio']:
                (folder / 'persisted-audio.wav').write_bytes(bytes(row['audio']))
            row['audio'] = base64.b64encode(bytes(row['audio'] or b'')).decode()
            (folder / 'database-record.json').write_text(json.dumps(row, default=str, indent=2))
            summary['ledger'] = conn.execute('SELECT count(*) AS requests, coalesce(sum(characters),0) AS characters FROM syntelle_speech.requests WHERE budget_id=%s', ('authorized-dragon-motion-2026-10-19',)).fetchone()
    finally:
        (folder / 'verification.json').write_text(json.dumps(summary, indent=2))
        saved = db.archive_tree('head-motion-12/speech/' + args.case, folder)
        restored = db.restore(saved['snapshot_id'], ROOT / '.tools/head-motion-12-restored' / str(saved['snapshot_id']))
        db.event('head-motion-12/speech/' + args.case, 'verified-restoration', {'snapshot': saved, 'files': restored})
        print(json.dumps(summary, indent=2))
    if not summary['native_visemes_verified']:
        raise SystemExit('Live Dragon verification failed; no retry or voice fallback will run.')


if __name__ == '__main__':
    main()

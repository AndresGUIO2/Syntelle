import base64
import io
import wave

import pytest

from app.preparation.faces import PreparationError
from app.workflows.speech import qualify_serena as qualification


def record():
    output = io.BytesIO()
    with wave.open(output, 'wb') as wav:
        wav.setparams((1, 2, 24000, 0, 'NONE', 'not compressed'))
        wav.writeframes(b'\0\0' * 2400)
    return {'voice': qualification.VOICE, 'language': 'en-US',
            'audio_base64': base64.b64encode(output.getvalue()).decode(),
            'duration_ms': 100, 'visemes': [{'id': 1, 'offset_ms': 0}, {'id': 0, 'offset_ms': 100}]}


def test_native_controls_preserve_voice_and_validate_events():
    value = record()
    digest, duration = qualification.checked_record(value, 'en-US')
    assert len(digest) == 64 and duration == 100
    value['voice'] = 'en-US-Ava:DragonHDLatestNeural'
    with pytest.raises(PreparationError):
        qualification.checked_record(value, 'en-US')
    value = record()
    value['visemes'][1]['offset_ms'] = 500
    with pytest.raises(PreparationError):
        qualification.checked_record(value, 'en-US')


def test_missing_catalog_never_invokes_provider(monkeypatch, tmp_path):
    monkeypatch.setattr(qualification, 'OUTPUT', tmp_path)
    monkeypatch.setattr(qualification, 'dotenv_values', lambda _: {})
    monkeypatch.setattr(qualification.subprocess, 'run', lambda *a, **kw: pytest.fail('provider invoked without regional availability'))
    with pytest.raises(PreparationError, match='verified regional catalog'):
        qualification.run('ask-en-plain')


def test_missing_serena_controls_cannot_export_candidate(monkeypatch, tmp_path):
    monkeypatch.setattr(qualification, 'OUTPUT', tmp_path)
    with pytest.raises(PreparationError, match='Complete native Serena controls'):
        qualification.prepare()
    assert not (tmp_path / 'active-profile').exists()

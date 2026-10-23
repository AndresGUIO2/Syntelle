import json
from hashlib import sha256
from pathlib import Path

import pytest

from app.workflows.motion.prepare_performance_profile import prepare


def test_profile_requires_matching_review_and_has_no_unprepared_smile(tmp_path: Path):
    package=tmp_path/'package';package.mkdir()
    (package/'manifest.json').write_text(json.dumps({'package_hash':'a'*64,'packages':{}}))
    review=tmp_path/'review.json'
    review.write_text(json.dumps({'package_hash':'b'*64,'composition_passed':True,'playback_passed':True}))
    with pytest.raises(ValueError,match='Source-bound'):
        prepare(package,review,None,tmp_path/'rejected.json')
    review.write_text(json.dumps({'package_hash':'a'*64,'composition_passed':True,'playback_passed':True}))
    result=prepare(package,review,None,tmp_path/'accepted.json')
    assert 'smile' not in result['capabilities'] and 'laugh' not in result['capabilities']
    with pytest.raises(ValueError,match='Preserve'):
        prepare(package,review,None,tmp_path/'accepted.json')


@pytest.mark.parametrize('dental_ready,valid,expected', [(True, True, True), (False, True, False), (True, False, False)])
def test_reference_laugh_is_independent_of_smile_and_requires_prepared_dentition(tmp_path: Path, dental_ready, valid, expected):
    package = tmp_path / 'package'
    package.mkdir()
    reference = {'version': 'reference-laughter/1', 'validation': {'valid': valid}, 'dental_ready': dental_ready,
                 'source_sha256': 's' * 64, 'motion_build_id': 'motion', 'oral_build_id': 'oral',
                 'reference_sha256': 'r' * 64, 'frames': [{}] * 25}
    (package / 'manifest.json').write_text(json.dumps({'package_hash': 'a' * 64, 'packages': {
        'expressions': {'reference_laughter': reference}, 'motion': {'original_sha256': 's' * 64, 'build_id': 'motion'}, 'oral': {'build_id': 'oral'}}}), encoding='utf-8')
    review = tmp_path / 'review.json'
    review.write_text(json.dumps({'package_hash': 'a' * 64, 'composition_passed': True, 'playback_passed': True}), encoding='utf-8')
    audio = json.dumps({'voice': 'cached-voice', 'language': 'en-US', 'duration_ms': 1000, 'audio_base64': 'fixture'}).encode()
    (tmp_path / 'audio.json').write_bytes(audio)
    bank = tmp_path / 'bank.json'
    bank.write_text(json.dumps({'recordings': [{'file': 'audio.json', 'sha256': sha256(audio).hexdigest(), 'approved': True, 'approval': 'fixture-only'}]}), encoding='utf-8')
    result = prepare(package, review, bank, tmp_path / 'profile.json')
    assert 'smile' not in result['capabilities']
    assert ('laugh' in result['capabilities']) is expected
    assert result['cached_recordings'] == int(expected)

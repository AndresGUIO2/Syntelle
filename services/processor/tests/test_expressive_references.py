import json
from hashlib import sha256

import pytest

from app.preparation.faces import PreparationError
from app.workflows.motion.prepare_expressive_references import run


def test_rejects_changed_cached_reference_without_provider(tmp_path):
    source = tmp_path/'original.jpg'
    source.write_bytes(b'original')
    settings = {'status': 'owner-authorized', 'limits': {'image': 4, 'analysis': 0}, 'original': str(source)}
    config = tmp_path/'config.json'
    config.write_text(json.dumps(settings))
    stage = tmp_path/'output'/'smile'
    stage.mkdir(parents=True)
    (stage/'settings.json').write_bytes(config.read_bytes())
    (stage/'result.jpg').write_bytes(b'received')
    (stage/'run.json').write_text(json.dumps({'status': 'received', 'file': 'result.jpg', 'sha256': sha256(b'received').hexdigest(), 'original_sha256': sha256(b'original').hexdigest()}))
    run(config, 'smile', tmp_path/'output')
    source.write_bytes(b'rebound')
    with pytest.raises(PreparationError, match='source changed'):
        run(config, 'smile', tmp_path/'output')


def test_provider_error_stops_other_expression_slots(tmp_path):
    config = tmp_path/'config.json'
    config.write_text(json.dumps({'status': 'owner-authorized', 'limits': {'image': 4, 'analysis': 0}}))
    stage = tmp_path/'output'/'smile'
    stage.mkdir(parents=True)
    (stage/'run.json').write_text(json.dumps({'status': 'failed'}))
    with pytest.raises(PreparationError, match='earlier request stopped'):
        run(config, 'laugh', tmp_path/'output')

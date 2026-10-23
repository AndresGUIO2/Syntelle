"""Offline provider and registration regressions; no credentials or database writes."""
import base64
from hashlib import sha256
from io import BytesIO
import json

import numpy as np
from PIL import Image
import pytest

from app.core.paths import ROOT
from app.preparation.faces import PreparationError
from app.preparation.expressions import field_from_image, render, validate
from app.workflows.motion import prepare_smile_reference as workflow


def test_reference_request_preserves_bytes_and_reuses_cache_without_retries(tmp_path, monkeypatch):
    original = tmp_path/'portrait.png'
    Image.new('RGB', (100, 100), '#aabbcc').save(original)
    points = tmp_path/'landmarks.json'
    points.write_text(json.dumps({'landmarks_px': [[30+i%30, 30+i%35, 0] for i in range(478)]}))
    prompt = tmp_path/'prompt.txt'; prompt.write_text('Source-bound test prompt.')
    config = tmp_path/'config.json'
    config.write_text(json.dumps({'status': 'owner-authorized', 'budget_id': 'isolated-offline-test',
        'limits': {'image': 2, 'analysis': 0}, 'original': str(original),
        'cached_landmarks': str(points), 'prompt': str(prompt), 'model': 'test-model',
        'response_format': {'type': 'image', 'mime_type': 'image/jpeg', 'image_size': '1K', 'aspect_ratio': '3:4'}}))
    class FakeArchive:
        def budget(self, name, binding, limits):
            assert limits == {'image': 2, 'analysis': 0}
        def archive_tree(self, *args):
            return {'snapshot_id': 'isolated'}
    calls = []
    def provider(_db, _budget, _root, _kind):
        def invoke(payload, settings, key, stage):
            assert (stage/'request.json').exists() and (stage/'run.json').exists()
            assert settings['retries'] == 0 and settings['follow_redirects'] is False
            assert base64.b64decode(payload['input'][1]['data']) == original.read_bytes()
            assert payload['input'][2]['mime_type'] == 'image/png'
            calls.append(payload)
            output = BytesIO(); Image.new('RGB', (100, 100)).save(output, 'JPEG')
            return json.dumps({'output_image': {'data': base64.b64encode(output.getvalue()).decode()}}).encode(), 200, .1
        return invoke
    monkeypatch.setattr(workflow, 'Archive', FakeArchive)
    monkeypatch.setattr(workflow, 'RecordedProvider', provider)
    monkeypatch.setattr(workflow, 'dotenv_values', lambda _: {'GEMINI_API_KEY': 'isolated-test-value'})
    out = tmp_path/'output'
    workflow.run(config, out, True)
    workflow.run(config, out, True)
    assert len(calls) == 1
    assert (out/'proposal/reference.png').exists() and (out/'proposal/reference.webp').exists()
    prompt.write_text('Changed prompt must not silently rebind a received request.')
    with pytest.raises(PreparationError, match='rebound or retried'):
        workflow.run(config, out, True)
    assert len(calls) == 1


def test_registered_smile_moves_complete_seam_and_preserves_unedited_pixels():
    root = ROOT/'.tools/smile-reference-26/prepared'
    candidates = sorted(root.glob('*/record.json'), key=lambda p: p.stat().st_mtime)
    if not candidates:
        pytest.skip('Received local reference is required.')
    folder = candidates[-1].parent
    package = json.loads((folder/'expressions.json').read_text())
    field = field_from_image(Image.open(folder/'field.png'), package['units_per_byte'])
    assert validate(field, package['crop'])['valid']
    assert sha256((folder/'smile-material.png').read_bytes()).hexdigest() == package['smile']['material']['sha256']
    source = Image.open(folder/'input-original.jpg').convert('RGB')
    points = np.asarray(json.loads((folder/'input-landmarks.json').read_text())['landmarks_px'])
    crop = package['crop']
    # Regression for the rejected central lip pin: both seam center and corners
    # must participate; testing only corner movement would miss the failure.
    for landmark in (13, 14, 61, 291):
        x = int((points[landmark, 0]-crop['x'])*field.shape[1]/crop['width'])
        y = int((points[landmark, 1]-crop['y'])*field.shape[0]/crop['height'])
        assert np.linalg.norm(field[y, x, :2]) > 1
    np.testing.assert_array_equal(np.asarray(render(source, field, crop, 0, 0)), np.asarray(source))
    result = np.asarray(render(source, field, crop, 1, 1))
    outside = np.ones(np.asarray(source).shape[:2], bool)
    outside[crop['y']:crop['y']+crop['height'], crop['x']:crop['x']+crop['width']] = False
    np.testing.assert_array_equal(result[outside], np.asarray(source)[outside])

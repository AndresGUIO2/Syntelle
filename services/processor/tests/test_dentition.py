from hashlib import sha256
from io import BytesIO
import json
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw
import pytest
from app import dentition_trial
from app.preparation.dentition import prepare_rows
from app.preparation.faces import PreparationError
from app.preparation.gemini import generate_a
from app.preparation.oral import cavity_mask


def test_request_includes_exact_original_and_separate_a_reference(monkeypatch):
    import base64
    import httpx
    client = httpx.Client
    payloads = []
    def respond(request):
        payloads.append(json.loads(request.content))
        return httpx.Response(200, json={'output_image': {'data': base64.b64encode(b'dental-plate').decode(), 'mime_type': 'image/jpeg'}})
    def make_client(**kwargs):
        kwargs['transport'] = httpx.MockTransport(respond)
        return client(**kwargs)
    monkeypatch.setattr(httpx, 'Client', make_client)
    data, metadata = generate_a(b'original-bytes', 'dental prompt', 'private-test-key', 'test-model', supporting_images=((b'approved-A-crop', 'image/png'),))
    assert data == b'dental-plate' and len(payloads) == 1
    assert [base64.b64decode(i['data']) for i in payloads[0]['input'][1:]] == [b'original-bytes', b'approved-A-crop']
    assert metadata['request_settings']['supporting_images'][0]['sha256'] == sha256(b'approved-A-crop').hexdigest()
    assert 'private-test-key' not in json.dumps(metadata)


def test_dental_request_reuses_cache_and_refuses_lost_output(tmp_path, monkeypatch):
    original = tmp_path/'original.jpg'; original.write_bytes(b'original')
    reference = tmp_path/'a'; reference.mkdir(); (reference/'mouth-a.png').write_bytes(b'A-detail')
    (reference/'run.json').write_text(json.dumps({'source_sha256': sha256(b'original').hexdigest()}))
    image = BytesIO(); Image.new('RGB', (32, 48)).save(image, format='PNG')
    calls = []
    def generate(data, prompt, key, model, **options):
        calls.append((data, options)); return image.getvalue(), {'usage': {}, 'model': model}
    monkeypatch.setattr(dentition_trial, 'generate_a', generate)
    monkeypatch.setattr(dentition_trial, 'dotenv_values', lambda _: {'GEMINI_API_KEY': 'private-test-key'})
    output = tmp_path/'output'
    dentition_trial.prepare(original, reference, output, True)
    dentition_trial.prepare(original, reference, output, True)
    assert len(calls) == 1 and calls[0][0] == b'original'
    record = json.loads((output/'run.json').read_text())
    assert record['attempts'] == 1
    assert record['prompt_sha256'] == sha256((output/'prompt.txt').read_bytes()).hexdigest()
    (reference/'mouth-a.png').write_bytes(b'changed-detail')
    with pytest.raises(PreparationError, match='inputs changed'):
        dentition_trial.prepare(original, reference, output, True)
    (reference/'mouth-a.png').write_bytes(b'A-detail')
    (output/record['file']).unlink()
    with pytest.raises(PreparationError, match='No retry'):
        dentition_trial.prepare(original, reference, output, True)
    assert len(calls) == 1


def test_failed_dental_request_cannot_be_silently_retried(tmp_path, monkeypatch):
    original = tmp_path/'original.jpg'; original.write_bytes(b'original')
    reference = tmp_path/'a'; reference.mkdir(); (reference/'mouth-a.png').write_bytes(b'A-detail')
    (reference/'run.json').write_text(json.dumps({'source_sha256': sha256(b'original').hexdigest()}))
    calls = []
    def fail(*_, **__):
        calls.append(1)
        raise PreparationError('Synthetic provider failure; no retry.')
    monkeypatch.setattr(dentition_trial, 'generate_a', fail)
    monkeypatch.setattr(dentition_trial, 'dotenv_values', lambda _: {'GEMINI_API_KEY': 'private-test-key'})
    for _ in range(2):
        with pytest.raises(PreparationError):
            dentition_trial.prepare(original, reference, tmp_path/'output', True)
    assert len(calls) == 1
    record = json.loads((tmp_path/'output/run.json').read_text())
    assert record['status'] == 'failed' and record['attempts'] == 1
    assert 'private-test-key' not in json.dumps(record)


def test_plate_calibration_is_deterministic_and_preserves_transparent_backdrop():
    raw = Image.new('RGB', (120, 120)); draw = ImageDraw.Draw(raw)
    draw.rectangle((10, 10, 110, 40), fill=(180, 160, 140))
    draw.rectangle((10, 75, 110, 105), fill=(160, 140, 120))
    config = {'matte': {'black_level': 8, 'ramp': 16, 'edge_threshold': 40}, 'rows': {}}
    for name, box, edge_y in [('upper', [0, 0, 120, 60], 25), ('lower', [0, 60, 120, 120], 55)]:
        config['rows'][name] = {'source_box': box, 'width': 90, 'center_x': 60, 'cutting_edge_y': edge_y, 'edge_curve_px': 0, 'edge_smoothing_source_px': 3, 'exposure_gain': 0.6}
    upper, lower, evidence = prepare_rows(raw, config, (120, 100))
    upper2, lower2, evidence2 = prepare_rows(raw, config, (120, 100))
    assert upper.tobytes() == upper2.tobytes() and lower.tobytes() == lower2.tobytes() and evidence == evidence2
    assert upper.getpixel((60, 20)) == (108, 96, 84, 255)
    assert upper.getpixel((0, 0))[3] == lower.getpixel((0, 0))[3] == 0
    assert upper.getbbox()[2]-upper.getbbox()[0] > 70


def test_archived_dental_candidate_preserves_sources_and_shared_rows():
    root = Path(__file__).resolve().parents[3]
    rows = root/'.tools/dentition-01/package-current'
    if not (rows/'package.json').exists(): pytest.skip('Rejected local dental evidence is not distributed.')
    package = json.loads((rows/'package.json').read_text())
    old = root/'.tools/visemes-01/package/package.json'
    if not old.exists(): pytest.skip('Previous local preparation evidence is not distributed.')
    baseline = json.loads(old.read_text())
    assert package['reference']['provenance']['kind'] == 'complete-synthetic-plate'
    assert package['source_sha256'] == baseline['source_sha256']
    assert package['source_run_hashes'] == baseline['source_run_hashes']
    assert all(p['changed_pixels_outside_mask'] == 0 for p in package['poses'].values())
    upper = np.asarray(Image.open(rows/'upper-teeth.webp').convert('RGBA'))
    lower = np.asarray(Image.open(rows/'lower-teeth.webp').convert('RGBA'))
    assert (upper[...,3] == 255).sum() > 2000
    assert (lower[...,3] == 255).sum() > 2000
    # Only lip occlusion and an integer lower-row translation may vary by pose.
    for pose in package['poses'].values():
        endpoint = np.asarray(Image.open(rows/pose['endpoint']).convert('RGBA'))
        cavity = np.asarray(cavity_mask((upper.shape[1],upper.shape[0]),np.array(pose['points']))) > 0
        if pose['upper_visible']:
            visible = (upper[...,3] == 255) & cavity & (endpoint[...,3] == 255)
            np.testing.assert_array_equal(endpoint[visible,:3],upper[visible,:3])
        if pose['lower_visible']:
            moved = Image.new('RGBA',(lower.shape[1],lower.shape[0]))
            moved.alpha_composite(Image.fromarray(lower),tuple(pose['lower_translation']))
            moved = np.asarray(moved)
            visible = (moved[...,3] == 255) & cavity & (endpoint[...,3] == 255)
            np.testing.assert_array_equal(endpoint[visible,:3],moved[visible,:3])


def test_owner_rejected_dentition_cannot_be_rebuilt_or_installed(tmp_path):
    from app.prepare_viseme_package import prepare
    review = tmp_path/'review.json'
    review.write_text(json.dumps({'owner_review': {'status': 'rejected'}}))
    with pytest.raises(PreparationError, match='owner rejected'):
        prepare(tmp_path/'missing-reference',tmp_path/'missing-batch',review,tmp_path/'output')
    assert not (tmp_path/'output').exists()

"""Regression evidence for duplicated cheek motion and fixed lip edit coverage."""
import json

import numpy as np
from PIL import Image
import pytest

from app.core.paths import ROOT


def test_positive_moods_do_not_repeat_the_registered_smile_cheek_motion():
    candidate = ROOT / '.tools/emotion-masks-83/prepared-coverage'
    if not (candidate / 'expressions/expressions.json').is_file():
        pytest.skip('Cached mask correction required.')
    before = ROOT / '.tools/emotions-82/prepared-v11'
    previous = json.loads((before / 'expressions/expressions.json').read_bytes())['emotion_rig']
    current = json.loads((candidate / 'expressions/expressions.json').read_bytes())['emotion_rig']
    points = np.asarray(json.loads((candidate / 'landmarks.json').read_bytes())['landmarks_px'])
    width, height = current['field_size']
    def fields(folder, rig):
        return np.asarray(Image.open(folder / 'expressions' / rig['field']['file'])).reshape(10, height, width, 4)
    a, b = fields(before, previous), fields(candidate, current)
    ys = current['crop']['y'] + (np.arange(height)+.5)*current['crop']['height']/height
    lower_face = ys >= points[2, 1]
    assert np.any(a[1:4, lower_face, :, :2] != 128), 'comparator must reproduce the duplicate field'
    assert np.all(b[1:4, lower_face, :, :2] == 128), 'positive mood still independently moves protected-mouth neighbors'
    np.testing.assert_array_equal(a[[0,4,5,6,7,8,9]], b[[0,4,5,6,7,8,9]])
    assert current['units_per_byte'] == previous['units_per_byte']
    assert current['oral'] == previous['oral'], 'accepted lip geometry changed'
    assert current['edit_coverage'] == 'deformed-oral-coverage/1'


def test_mask_correction_keeps_all_existing_materials():
    new = ROOT / '.tools/emotion-masks-83/package-coverage/manifest.json'
    if not new.is_file():
        pytest.skip('Cached native mask correction required.')
    before = json.loads((ROOT / '.tools/emotions-82/package-v11/manifest.json').read_bytes())
    after = json.loads(new.read_bytes())
    changed = []
    for name, record in before['textures'].items():
        if after['textures'][name]['sha256'] != record['sha256']:
            changed.append(name)
    assert changed == ['expressions/emotion-fields.png']
    assert before['geometry'] == after['geometry']

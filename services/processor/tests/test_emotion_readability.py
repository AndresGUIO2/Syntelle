"""Offline regression checks for the owner's negative-expression calibration."""
import json
import numpy as np
from PIL import Image
import pytest
from app.core.paths import ROOT


@pytest.fixture(scope='module')
def calibration():
    old = ROOT/'.tools/emotion-skin-84/prepared-v2/expressions'
    new = ROOT/'.tools/emotion-readability-87/prepared-v3/expressions'
    if not (new/'expressions.json').exists():
        pytest.skip('Cached readability preparation required')
    return old, new, json.loads((old/'expressions.json').read_bytes())['emotion_rig'], json.loads((new/'expressions.json').read_bytes())['emotion_rig']


def test_other_emotions_and_neutral_remain_exact(calibration):
    old, new, a, b = calibration
    assert a['units_per_byte'] == b['units_per_byte']
    w, h = a['field_size']
    before = np.asarray(Image.open(old/a['field']['file'])).reshape(10,h,w,4)
    after = np.asarray(Image.open(new/b['field']['file'])).reshape(10,h,w,4)
    for i, name in enumerate(a['states']):
        if name not in ('angry','fearful','disgusted'):
            assert np.array_equal(before[i],after[i]), name
    assert not np.array_equal(after[6],after[9])
    assert b['validation']['valid']
    assert all(v['valid'] for v in b['validation']['transition_midpoints'].values())


def test_sadness_moves_corners_instead_of_medial_lip(calibration):
    _, _, a, b = calibration
    before=np.asarray(a['oral']['states']['sad']['Rest-A']['rest_delta'])
    after=np.asarray(b['oral']['states']['sad']['Rest-A']['rest_delta'])
    assert np.count_nonzero(after[:40,1]) < np.count_nonzero(before[:40,1])
    assert after[:,1].sum() < before[:,1].sum()
    assert after[:,1].max()/after[:,1].sum() > before[:,1].max()/before[:,1].sum()


def test_contact_and_attachment_stay_pinned(calibration):
    _, _, _, rig = calibration
    assert rig['oral']['central_contact_pinned']
    assert rig['oral']['dental_translation'] == [0,0]
    assert not rig['oral']['tooth_warping']
    for name, pairs in rig['oral']['states'].items():
        assert rig['oral']['validation']['states'][name]['minimum_triangle_ratio'] > .2
        for pair in pairs.values():
            for key in ('rest_delta','open_delta'):
                assert np.all(np.asarray(pair[key])[40:] == 0)
                # The central upper and lower lip/contact vertices are fixed.
                assert np.all(np.asarray(pair[key])[[4,5,6,14,15,16,24,25,26,34,35,36]] == 0)


def test_existing_materials_are_preserved():
    a=ROOT/'.tools/emotion-skin-84/package-final/manifest.json'
    b=ROOT/'.tools/emotion-readability-87/package-v3/manifest.json'
    if not b.exists():pytest.skip('Exported candidate required')
    before,after=(json.loads(p.read_bytes()) for p in (a,b))
    assert before['geometry'] == after['geometry']
    for name, texture in before['textures'].items():
        if 'emotion-fields' not in name:
            assert texture['sha256'] == after['textures'][name]['sha256'], name

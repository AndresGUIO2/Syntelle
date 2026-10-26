"""Focused checks on cached resources; never prepares images or calls providers."""
import json
from itertools import product

import numpy as np
from PIL import Image
import pytest

from app.core.paths import ROOT


@pytest.fixture(scope='module')
def prepared_emotions():
    folder = ROOT / '.tools/emotions-82/prepared-v11'
    if not (folder / 'preparation.json').is_file():
        pytest.skip('Cached emotion preparation is required.')
    bundle = json.loads((folder / 'resource-bundle.json').read_bytes())
    expression = json.loads((folder / 'expressions/expressions.json').read_bytes())
    oral = json.loads((ROOT / bundle['oral'] / 'package.json').read_bytes())
    landmarks = json.loads((folder / 'landmarks.json').read_bytes())['landmarks_px']
    return folder, expression, oral, np.asarray(landmarks)[:, :2]


def test_neutral_and_protected_regions_are_undeformed(prepared_emotions):
    folder, expression, _, _ = prepared_emotions
    rig = expression['emotion_rig']
    atlas = np.asarray(Image.open(folder / 'expressions' / rig['field']['file']))
    width, height = rig['field_size']
    fields = atlas[..., :2].reshape(10, height, width, 2)
    assert np.all(fields[0] == 128)
    assert all(row['valid'] for row in rig['validation']['states'].values())
    assert len(rig['validation']['transition_midpoints']) == 45
    assert all(row['valid'] for row in rig['validation']['transition_midpoints'].values())
    guard = np.asarray(Image.open(folder / 'expressions/emotion-protected.png'))
    crop = rig['crop']
    y, x = np.indices((height, width))
    sx = np.rint(crop['x'] + (x + .5)*crop['width']/width).astype(int)
    sy = np.rint(crop['y'] + (y + .5)*crop['height']/height).astype(int)
    # Test the interior, excluding subpixel boundary filtering footprints.
    import cv2
    interior = cv2.erode(guard, np.ones((9, 9), np.uint8))[sy, sx] == 255
    assert np.all(fields[:, interior] == 128)


def test_emotions_keep_dental_bytes_and_existing_geometry(prepared_emotions):
    before = json.loads((ROOT / '.tools/speech-lighting-81/package-v2/manifest.json').read_bytes())
    after = json.loads((ROOT / '.tools/emotions-82/package-v11/manifest.json').read_bytes())
    assert before['geometry'] == after['geometry']
    for name, record in before['textures'].items():
        assert {k:v for k,v in after['textures'][name].items() if k != 'source'} == {k:v for k,v in record.items() if k != 'source'}


def test_joint_contacts_and_competing_emotions(prepared_emotions):
    _, expression, oral, points = prepared_emotions
    rig = expression['emotion_rig']['oral']
    origin = np.array([oral['crop']['x'], oral['crop']['y']])
    center = (points[61] + points[291])*.5 - origin
    width = np.linalg.norm(points[61] - points[291])

    def area(vertices, triangles):
        a = vertices[triangles[:, 1]] - vertices[triangles[:, 0]]
        b = vertices[triangles[:, 2]] - vertices[triangles[:, 0]]
        return a[:, 0]*b[:, 1] - a[:, 1]*b[:, 0]

    for name in ('Rest-MBP', 'Rest-FV', 'A-O'):
        pair = oral['pairs'][name]['mesh']
        a, b = np.asarray(pair['rest']), np.asarray(pair['open'])
        triangles = np.asarray(rig['triangles'][name])
        for action in rig['states'].values():
            for key, vertices in (('rest_delta', a), ('open_delta', b)):
                delta = np.asarray(action[name][key])
                central = np.abs(vertices[:40, 0] - center[0]) <= width*.15
                assert np.all(delta[:40][central] == 0), 'central lip/contact seam moved'
                assert np.all(delta[40:] == 0), 'patch attachment moved'
        actions = list(rig['states'].values())
        for left, right in product(actions, repeat=2):
            for u in (0., .5, 1.):
                base = a*(1-u) + b*u
                delta = sum((np.asarray(v[name]['rest_delta'])*(1-u) + np.asarray(v[name]['open_delta'])*u)*.5 for v in (left, right))
                reference = area(base, triangles)
                selected = np.abs(reference) > .05
                assert np.min(area(base+delta, triangles)[selected]/reference[selected]) > .2


def test_preparation_rejects_landmarks_from_a_different_source(prepared_emotions, tmp_path):
    from app.preparation.faces import PreparationError
    from app.workflows.motion.prepare_emotions import prepare

    folder, _, _, _ = prepared_emotions
    landmarks = json.loads((folder / 'landmarks.json').read_bytes())
    landmarks['landmarks_px'][152][1] += 1
    incorrect = tmp_path / 'incorrect.json'
    incorrect.write_text(json.dumps(landmarks), encoding='utf-8')
    output = tmp_path / 'rejected'
    with pytest.raises(PreparationError, match='source-bound motion anatomy'):
        prepare(ROOT / '.tools/speech-lighting-81/prepared-v2/resource-bundle.json', incorrect, output)
    assert not output.exists()

import json
from pathlib import Path

import numpy as np
import pytest

from app.core.paths import ROOT
from app.preparation.faces import PreparationError
from app.workflows.motion import prepare_speech_jaw

SOURCE = ROOT / '.tools/paired-parallax-100/prepared-v2/package'
LANDMARKS = ROOT / '.tools/foreground-11/prepared/syntelle/base-landmarks.json'


@pytest.mark.skipif(not SOURCE.exists(), reason='private cached source fixture')
def test_middle_skin_bridge_is_connected_without_protecting_the_entire_crop(tmp_path):
    prepare_speech_jaw.prepare(SOURCE, LANDMARKS, tmp_path / 'jaw')
    manifest = json.loads((tmp_path / 'jaw/package/manifest.json').read_bytes())
    motion, oral = manifest['packages']['motion'], manifest['packages']['oral']
    rig = motion['speech_jaw']
    geometry = motion['projection']['geometry']
    raw = (tmp_path / 'jaw/package' / manifest['geometry']['file']).read_bytes()
    vertices = np.frombuffer(raw[:geometry['vertices'] * 20], '<f4').reshape(-1, 5)
    xy = vertices[:, :2] * [motion['canvas']['width'], motion['canvas']['height']]
    # This is skin below every observed lower-lip endpoint. It used to be
    # immobilized solely because it was inside a broad editing rectangle.
    middle = np.array([rig['chin'][0], oral['crop']['y'] + oral['crop']['height']])
    nearest = np.argmin(np.sum((xy-middle)**2, axis=1))
    assert rig['weights'][nearest] > .05, 'middle lower-face skin is still a frozen island'
    assert rig['version'] == 'speech-jaw/2'
    support = rig['oral_support']
    assert support['version'] == 'calibrated-outer-lip-support/1'
    assert len(support['polygon']) >= 3
    assert support['transition_width_source_px'] > support['sampling_guard_source_px']


@pytest.mark.skipif(not SOURCE.exists(), reason='private cached source fixture')
def test_subtle_speech_jaw_preserves_oral_and_laughter_resources(tmp_path):
    report = prepare_speech_jaw.prepare(SOURCE, LANDMARKS, tmp_path / 'speech-jaw')
    old = json.loads((SOURCE / 'manifest.json').read_bytes())
    new = json.loads((tmp_path / 'speech-jaw/package/manifest.json').read_bytes())
    assert new['textures'] == old['textures']
    assert new['geometry'] == old['geometry']
    assert new['packages']['oral'] == old['packages']['oral']
    assert new['packages']['expressions'] == old['packages']['expressions']
    assert new['packages']['motion'].get('mandible') is None
    rig = new['packages']['motion']['speech_jaw']
    assert rig['validation']['valid'] and rig['validation']['minimum_area_ratio'] > .25
    assert 0 < rig['validation']['chin_fraction'] <= .0075 + 1e-10
    assert rig['validation']['maximum_oral_weight'] == rig['validation']['maximum_neck_weight'] == 0
    assert report['provider_requests'] == 0
    assert rig['pose_opening']['Rest'] == 0
    assert not rig['provenance']['manual_masks']
    geometry = old['packages']['motion']['projection']['geometry']
    raw = (SOURCE / old['geometry']['file']).read_bytes()
    vertices = np.frombuffer(raw[:geometry['vertices'] * 20], '<f4').reshape(-1, 5)
    assert np.max(np.asarray(rig['weights'])[vertices[:, 3] == 0]) == 0


@pytest.mark.skipif(not SOURCE.exists(), reason='private cached source fixture')
def test_rejects_mismatched_landmarks_before_preparation(tmp_path):
    points = json.loads(LANDMARKS.read_bytes())
    points['landmarks_px'][152][1] += 3
    wrong = tmp_path / 'wrong.json'
    wrong.write_text(json.dumps(points))
    with pytest.raises(PreparationError, match='another source'):
        prepare_speech_jaw.prepare(SOURCE, wrong, tmp_path / 'speech-jaw')
    assert not (tmp_path / 'speech-jaw').exists()

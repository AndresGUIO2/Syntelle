"""Focused cached-resource checks; no provider or preparation budget is used."""
import json
from hashlib import sha256
import pytest
from app.core.paths import ROOT


def test_laughter_preserves_the_existing_manual_smile_and_speech_geometry():
    candidate = ROOT/'.tools/laughter-face-74/candidate-v6'
    if not candidate.exists():
        pytest.skip('Local laughter candidate unavailable')
    previous = ROOT/'.tools/expression-controls-75/runtime-expressions'
    for name in ['field.png','smile-material.png',*[f'smile-skin-{i:02d}.webp' for i in range(25)]]:
        assert sha256((previous/name).read_bytes()).digest() == sha256((candidate/name).read_bytes()).digest()
    a=json.loads((previous/'expressions.json').read_bytes())
    b=json.loads((candidate/'expressions.json').read_bytes())
    assert a['joint_articulation']['pairs']==b['joint_articulation']['pairs']
    laugh=b['joint_articulation']['laugh']
    assert laugh['mode']=='boundary-laugh/1'
    assert laugh['maximum_articulation']==.06
    assert laugh['validation']['valid']
    assert set(laugh['pairs'])=={'Rest-A'}
    assert not laugh['manual_masks']
    assert not laugh['generated_dental_installed']
    assert laugh['lip_material']['generated_dental_pixels_installed']==0

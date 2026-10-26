import json

import numpy as np
import pytest

from app.core.paths import ROOT
from app.preparation.expression_skin import source_skin_frame
from app.preparation.faces import PreparationError


def test_skin_deformation_does_not_introduce_an_appearance_mask():
    foreground = np.full((24, 24, 4), [165, 125, 105, 255], np.uint8)
    field = np.zeros((8, 8, 2), np.float32)
    field[2:6, 2:6, 0] = 1
    crop = dict(x=4, y=4, width=16, height=16)
    for amount in (0, .5, 1):
        np.testing.assert_array_equal(source_skin_frame(foreground, field, crop, amount),
                                      foreground[4:20, 4:20])


def test_skin_uses_source_pixels_and_keeps_neutral_exact():
    yy, xx = np.indices((24, 24))
    foreground = np.stack([xx*8, yy*8, xx+yy, np.full_like(xx, 255)], -1).astype(np.uint8)
    field = np.zeros((8, 8, 2), np.float32)
    field[..., 0] = 1
    crop = dict(x=4, y=4, width=16, height=16)
    np.testing.assert_array_equal(source_skin_frame(foreground, field, crop, 0), foreground[4:20, 4:20])
    # The field tapers at its sampling boundary; its interior is an exact shift.
    np.testing.assert_array_equal(source_skin_frame(foreground, field, crop, 1)[2:-2, 2:-2],
                                  foreground[6:18, 5:17])
    with pytest.raises(PreparationError):
        source_skin_frame(foreground, field, crop, float('nan'))


def test_source_skin_package_preserves_joint_geometry_and_protected_materials():
    path = ROOT/'.tools/emotion-skin-84/package-final/manifest.json'
    if not path.exists():
        pytest.skip('Cached source-skin candidate required.')
    current = json.loads(path.read_bytes())
    previous = json.loads((ROOT/'.tools/emotion-masks-83/package-coverage/manifest.json').read_bytes())
    assert current['geometry'] == previous['geometry']
    for name, texture in previous['textures'].items():
        if not name.startswith('expressions/smile-skin-'):
            assert current['textures'][name]['sha256'] == texture['sha256'], name
    a, b = (manifest['packages']['expressions'] for manifest in (previous, current))
    old_joint, new_joint = a.pop('joint_articulation'), b.pop('joint_articulation')
    assert a == b, 'Manual smile, emotion fields or full-face laughter changed.'
    old_frames, new_frames = old_joint.pop('skin_frames'), new_joint.pop('skin_frames')
    appearance = new_joint.pop('skin_appearance')
    assert old_joint == new_joint, 'Oral articulation or joint geometry changed.'
    assert appearance['reference_appearance_blend'] is False
    assert current['textures']['expressions/'+new_frames[0]['file']]['sha256'] == previous['textures']['expressions/'+old_frames[0]['file']]['sha256']

from hashlib import sha256
import json

import numpy as np
from PIL import Image
import pytest

from app.core.paths import ROOT
from app.preparation.expressions import render, field_from_image, validate


@pytest.mark.parametrize('name', ['syntelle', 'cafe', 'woodland'])
def test_shared_expression_field_preserves_protected_pixels_and_neutral(name):
    records = list((ROOT/'.tools/expressions-25/prepared'/name).glob('*/record.json'))
    if not records:
        pytest.skip('Local cached expression preparation is required.')
    prepared = max(records, key=lambda p: p.stat().st_mtime).parent
    package = json.loads((prepared/'expressions.json').read_text())
    image = Image.open(next(prepared.glob('original.*'))).convert('RGB')
    original = np.asarray(image)
    field = field_from_image(Image.open(prepared/'field.png'), package['units_per_byte'])
    assert sha256((prepared/'field.png').read_bytes()).hexdigest() == package['field']['sha256']
    assert validate(field, package['crop'])['valid']
    assert package['validation']['maximum_inverse_residual_px'] <= .2
    np.testing.assert_array_equal(np.load(prepared/'field.npy'), field)
    np.testing.assert_array_equal(np.asarray(render(image, field, package['crop'], 0, 0)), original)
    protected = np.asarray(Image.open(prepared/'protected.png')) >= 250
    crop = package['crop']; outside = np.ones(original.shape[:2], bool)
    outside[crop['y']:crop['y']+crop['height'], crop['x']:crop['x']+crop['width']] = False
    for smile, brows in ((1, 0), (0, 1), (1, 1)):
        frame = np.asarray(render(image, field, crop, smile, brows))
        np.testing.assert_array_equal(frame[protected], original[protected])
        np.testing.assert_array_equal(frame[outside], original[outside])
        assert np.any(frame != original)
    oral_root = ROOT/'apps/web/src/experiments/viseme-a/characters'/name
    oral = json.loads((oral_root/'package.json').read_text())
    for pose in ('A', 'FV', 'MBP'):
        endpoint = image.convert('RGBA')
        patch = Image.open(oral_root/oral['poses'][pose]['endpoint']).convert('RGBA')
        endpoint.alpha_composite(patch, (oral['crop']['x'], oral['crop']['y']))
        endpoint = endpoint.convert('RGB')
        rendered = np.asarray(render(endpoint, field, crop, 1, 1))
        np.testing.assert_array_equal(rendered[protected], np.asarray(endpoint)[protected])


def test_jacobian_gate_rejects_folded_fields():
    yy, xx = np.indices((32, 32))
    field = np.zeros((32, 32, 3), float)
    field[..., 0] = -2*xx
    result = validate(field, {'width': 32, 'height': 32})
    assert not result['valid']
    assert result['minimum_jacobian'] < 0

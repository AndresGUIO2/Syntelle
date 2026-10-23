import numpy as np
from PIL import Image
import pytest

from app.preparation.aperture_lighting import prepare_lighting
from app.preparation.faces import PreparationError


def test_measured_opening_preserves_coverage_and_appearance(tmp_path):
    pixels = np.zeros((30, 12, 4), np.uint8)
    pixels[:, :, 0] = 173
    pixels[:, :, 3] = 255
    pixels[10:12, :4, 1] = 136
    pixels[5:25, 4:8, 1] = 200
    Image.fromarray(pixels).save(tmp_path/'mask.png')
    package = {'poses':{'fixture':{'mask':'mask.png', 'aperture_sdf':'fixture-sdf'}}}
    prepare_lighting(package, tmp_path, 20)
    result = np.asarray(Image.open(tmp_path/'mask.png'))
    assert np.array_equal(result[:, :, [0,1,3]], pixels[:, :, [0,1,3]])
    assert 20 < result[10, 1, 2] < 40
    assert result[10, 5, 2] == 255
    assert result[10, 9, 2] == 0
    assert package['oral']['visible_aperture']['reference_gap_px'] == 20


@pytest.mark.parametrize('gap', [0, -1, float('nan'), float('inf')])
def test_invalid_reference_does_not_prepare(gap, tmp_path):
    with pytest.raises(PreparationError):
        prepare_lighting({'poses':{}}, tmp_path, gap)

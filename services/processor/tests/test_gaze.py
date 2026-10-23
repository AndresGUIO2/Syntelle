from hashlib import sha256
import json

import cv2
import numpy as np
from PIL import Image
import pytest

from app.core.paths import ROOT
from app.preparation.gaze import harmonic_fill, render


def test_sclera_fill_uses_no_skin_pixels_and_preserves_known_material():
    rgb = np.full((25, 41, 3), [230, 30, 70], np.uint8)  # Deliberately red skin.
    domain = np.zeros((25, 41), bool); domain[6:20, 4:37] = True
    rgb[domain] = [190, 195, 200]
    unknown = np.zeros_like(domain); unknown[6:20, 15:25] = True
    rgb[unknown] = [15, 40, 80]
    filled = harmonic_fill(rgb, domain, unknown)
    np.testing.assert_array_equal(filled[~unknown], rgb[~unknown])
    np.testing.assert_array_equal(filled[unknown], np.tile([190, 195, 200], (unknown.sum(), 1)))


def test_rigid_pupil_translation_and_aperture_ownership():
    rgb = np.full((40, 100, 3), 200, np.uint8)
    masks = np.zeros((40, 100, 4), np.uint8)
    masks[5:35, 3:47, 0] = 255; masks[5:35, 53:97, 0] = 255
    masks[..., 2] = masks[..., 0]; masks[..., 3] = 255
    for center in ((25, 20), (75, 20)):
        ring = np.zeros(rgb.shape[:2], np.uint8)
        cv2.circle(ring, center, 8, 255, -1)
        masks[..., 1] = np.maximum(masks[..., 1], ring)
        rgb[ring > 0] = [20, 40, 60]
        cv2.circle(rgb, center, 3, (0, 0, 0), -1)
    plate = np.full_like(rgb, 200)
    shifted = render(rgb, plate, masks, [[3, 1], [3, 1]], [[25, 20], [75, 20]])
    np.testing.assert_array_equal(shifted[masks[..., 0] == 0], rgb[masks[..., 0] == 0])
    for cx in (25, 75):
        np.testing.assert_array_equal(shifted[18:25, cx:cx+7], rgb[17:24, cx-3:cx+4])
    np.testing.assert_array_equal(render(rgb, plate, masks, [[0, 0], [0, 0]], [[25, 20], [75, 20]]), rgb)


def test_vacated_iris_is_cleared_even_when_shifted_background_is_outside_eye():
    rgb = np.full((40, 100, 3), 200, np.uint8)
    masks = np.zeros((40, 100, 4), np.uint8)
    masks[5:35, 3:47, 0] = 255
    masks[..., 2] = masks[..., 0]; masks[..., 3] = 255
    iris = np.zeros((40, 100), np.uint8); cv2.circle(iris, (25, 25), 8, 255, -1)
    masks[..., 1] = iris; rgb[iris > 0] = [10, 20, 30]
    result = render(rgb, np.full_like(rgb, 200), masks, [[0, -4], [0, 0]], [[25, 25], [75, 25]])
    # At y=32 the old iris was visible; its shifted source y=36 is outside both
    # the aperture and iris. It is background, not missing iris material.
    np.testing.assert_array_equal(result[32, 25], [200, 200, 200])
    np.testing.assert_array_equal(result[21, 25], [10, 20, 30])


def test_subpixel_iris_filter_excludes_skin_and_has_no_support_threshold_jump():
    rgb = np.full((40, 100, 3), [250, 20, 20], np.uint8)
    masks = np.zeros((40, 100, 4), np.uint8)
    masks[5:35, 3:47, 0] = 255; masks[..., 2] = masks[..., 0]; masks[..., 3] = 255
    rgb[masks[..., 0] > 0] = [200, 200, 200]
    iris = np.zeros((40, 100), np.uint8); cv2.circle(iris, (25, 7), 8, 255, -1)
    masks[..., 1] = iris; rgb[(iris > 0) & (masks[..., 2] > 0)] = [20, 40, 60]
    plate = rgb.copy(); plate[masks[..., 0] > 0] = [200, 200, 200]
    frames = [render(rgb, plate, masks, [[0, dy], [0, 0]], [[25, 7], [75, 7]]) for dy in np.arange(.1, .9, .05)]
    for frame in frames:
        # Red skin must never become a moving iris fringe.
        assert not np.any((frame[5:35, 3:47, 0] > 220) & (frame[5:35, 3:47, 1] < 80))
    assert max(np.abs(b.astype(float)-a.astype(float)).max() for a, b in zip(frames, frames[1:])) < 20


def test_soft_iris_edge_does_not_lighten_when_gaze_leaves_zero():
    yy, xx = np.indices((40, 100), dtype=float)
    alpha = np.clip(8.5-np.hypot(xx-25, yy-20), 0, 1)
    masks = np.zeros((40, 100, 4), np.uint8)
    masks[5:35, 3:47, 0] = 255; masks[..., 2] = masks[..., 0]; masks[..., 3] = 255
    masks[..., 1] = np.uint8(np.rint(alpha*255))
    plate = np.full((40, 100, 3), 200, np.uint8)
    rgb = np.uint8(np.rint(plate*(1-alpha[..., None])+np.array([10, 20, 30])*alpha[..., None]))
    near_zero = render(rgb, plate, masks, [[.0001, 0], [0, 0]], [[25, 20], [75, 20]])
    np.testing.assert_array_equal(near_zero, rgb)


@pytest.mark.parametrize('name', ['syntelle', 'cafe', 'woodland'])
def test_installed_gaze_has_source_bound_cache_and_opaque_material(name):
    directory = ROOT/'apps/web/src/experiments/motion/gaze'/name
    package = json.loads((directory/'gaze.json').read_text())
    parent = json.loads((ROOT/'apps/web/src/experiments/motion/rgbd'/name/'package.json').read_text())
    assert package['motion_build_id'] == parent['build_id']
    assert package['source_sha256'] == parent['original_sha256']
    assert package['crop'] == parent['eyes']['crop']
    assert package['validation']['hidden_iris_generated'] is False
    for asset in package['assets'].values():
        assert sha256((directory/asset['file']).read_bytes()).hexdigest() == asset['sha256']
    masks = np.asarray(Image.open(directory/'ownership.png'))
    assert np.all(masks[..., 3] == 255)
    assert np.any(masks[..., 0]) and np.any(masks[..., 1])
    for eye in package['eyes']:
        assert max(eye['bounds_px'][0]) >= .25
        assert eye['maximum_protected_boundary_fraction'] <= .020001
        assert max(eye['bounds_px'][0]) <= eye['width_px']*.05
        assert max(eye['bounds_px'][1]) <= eye['width_px']*.02

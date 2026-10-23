import json
from pathlib import Path

import pytest

np = pytest.importorskip("numpy")
pytest.importorskip("cv2")
from scripts.export_mouth_morph import export

ROOT = Path(__file__).resolve().parents[3]


def test_cached_mesh_covers_crop_without_inversion_through_transition():
    mesh = json.loads((ROOT / "apps/web/src/experiments/viseme-a/mouth-mesh.json").read_text())
    rest, opened, triangles = np.array(mesh["rest"]), np.array(mesh["open"]), np.array(mesh["triangles"])
    assert np.isfinite(rest).all() and np.isfinite(opened).all()
    np.testing.assert_array_equal(rest[40:], opened[40:])
    signs = None
    for t in np.linspace(0, 1, 101):
        vertices = ((1-t)*rest + t*opened)[triangles]
        u, v = vertices[:, 1]-vertices[:, 0], vertices[:, 2]-vertices[:, 0]
        area = u[:, 0]*v[:, 1]-u[:, 1]*v[:, 0]
        if signs is None:
            signs = np.sign(area)
        assert np.min(area*signs) > 0
        assert sum(abs(area))/2 == pytest.approx(mesh["width"]*mesh["height"], abs=0.001)
    assert mesh["maximum_rest_regularization_px"] < 1
    cavity = set(mesh["cavity_triangles"])
    assert len(cavity) == 18
    for index, triangle in enumerate(triangles):
        assert (index in cavity) == all(20 <= point < 40 for point in triangle)


def test_local_export_preserves_opaque_mouth_rgb(tmp_path):
    from PIL import Image
    run = ROOT / ".tools/viseme-a-05"
    if not (run / "run.json").exists():
        pytest.skip("The owner-operated cached trial is not present.")
    export(run, tmp_path)
    with Image.open(run / "mouth-a.png") as original, Image.open(tmp_path / "mouth-texture.webp") as texture:
        np.testing.assert_array_equal(np.asarray(texture.convert("RGB")), np.asarray(original.convert("RGB")))
    result = json.loads((tmp_path / "mouth-mesh.json").read_text())
    cached = json.loads((ROOT / "apps/web/src/experiments/viseme-a/mouth-mesh.json").read_text())
    assert result == cached

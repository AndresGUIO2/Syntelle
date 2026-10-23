import json
from hashlib import sha256
from pathlib import Path
import pytest
from PIL import Image

np=pytest.importorskip("numpy")
pytest.importorskip("cv2")
from app.preparation.mesh import make_mesh
from app.preparation.faces import PreparationError

ROOT=Path(__file__).resolve().parents[3]
ASSETS=ROOT/"apps/web/src/experiments/viseme-a/poses"


def test_all_enabled_transition_meshes_cover_the_crop_without_inverting():
    package=json.loads((ASSETS/"package.json").read_text())
    safe=0
    for name,pair in package["pairs"].items():
        if not pair["safe"]:continue
        safe+=1
        source,target=name.split("-")
        assert all(package["poses"][p]["status"] in {'accepted','experimental'} for p in (source,target))
        mesh=pair["mesh"]; a,b,tri=np.array(mesh["rest"]),np.array(mesh["open"]),np.array(mesh["triangles"])
        signs=None
        for t in np.linspace(0,1,51):
            v=((1-t)*a+t*b)[tri]; u,w=v[:,1]-v[:,0],v[:,2]-v[:,0]
            areas=u[:,0]*w[:,1]-u[:,1]*w[:,0]
            if signs is None:signs=np.sign(areas)
            assert (areas*signs>0).all()
            assert sum(abs(areas))/2==pytest.approx(mesh["width"]*mesh["height"],abs=0.002)
    assert safe==70
    assert package['playback_policy']=='geometry-only-experiment'
    assert package["poses"]["FV"]["status"]=="experimental"
    assert package["poses"]["SZ"]["status"]=="experimental"
    assert package["pairs"]["SZ-FV"]["safe"]
    assert not package["pairs"]["U-CHSHR"]["safe"]
    assert package["pairs"]["O-U"]["safe"]


def test_all_exported_poses_are_lossless_and_masks_confine_edits():
    package=json.loads((ASSETS/"package.json").read_text())
    base=Image.open(ASSETS/"base.webp").convert("RGBA")
    bounds=package["crop"]; cropped=np.asarray(base.crop((bounds["x"],bounds["y"],bounds["x"]+bounds["width"],bounds["y"]+bounds["height"])))
    for pose in package["poses"].values():
        overlay=Image.open(ASSETS/pose["endpoint"]).convert("RGBA")
        frame=Image.fromarray(cropped.copy());frame.alpha_composite(overlay)
        outside=np.asarray(overlay)[:,:,3]==0
        np.testing.assert_array_equal(np.asarray(frame)[outside],cropped[outside])
        assert pose["changed_pixels_outside_mask"]==0
        assert np.all(np.asarray(frame)[:,:,3]==255)


def test_real_batch_used_seven_requests_and_the_identical_original():
    root=ROOT/".tools/visemes-01"
    if not (root/"batch.json").exists():pytest.skip("Owner-operated generation evidence is local.")
    batch=json.loads((root/"batch.json").read_text());prompt_source=ROOT/"docs/prompts/nano-banana-source.txt"
    for pose in batch["order"]:
        directory=root/pose.lower();record=json.loads((directory/"run.json").read_text());generation=record["generation"]
        assert record["source_sha256"]==batch["source_sha256"]==generation["request_settings"]["input_sha256"]
        assert generation["attempts"]==1
        assert generation["source_prompt_sha256"]==sha256(prompt_source.read_bytes()).hexdigest()
        assert generation["prompt_sha256"]==sha256((directory/f"prompt-{pose.lower()}.txt").read_bytes()).hexdigest()
        assert generation["raw_sha256"]==sha256((directory/generation["file"]).read_bytes()).hexdigest()
    assert len(batch["order"])==7

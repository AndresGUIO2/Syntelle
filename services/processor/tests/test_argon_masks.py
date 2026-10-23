import json
from pathlib import Path

import numpy as np
from PIL import Image
import pytest

from app.preparation.argon_masks import resolve_occlusion,load_argon_masks
from app.preparation.faces import PreparationError
from app.preparation.oral import remove_teeth

ROOT=Path(__file__).resolve().parents[3]


def test_overlap_hides_tongue_behind_enamel_without_changing_visible_tissue():
    upper=np.zeros((24,32),dtype=np.uint8);lower=upper.copy();tongue=upper.copy()
    upper[3:7,8:24]=255;lower[14:18,8:24]=255;tongue[6:16,9:23]=255
    upper[7,8:24]=20  # Antialias support also belongs to the foreground teeth.
    before=tongue.copy()
    visible,report=resolve_occlusion(*map(Image.fromarray,(upper,lower,tongue)))
    np.testing.assert_array_equal(tongue,before)
    assert report['tongue_full_pixels']==140 and report['tongue_occluded_pixels']>0
    assert report['visible_semantic_overlap_pixels']==0
    texture=np.random.default_rng(4).integers(0,256,(24,32,3),dtype=np.uint8)
    surface,_=remove_teeth(Image.fromarray(texture),Image.fromarray(np.maximum(upper,lower)),visible)
    protected=np.asarray(visible)>0
    np.testing.assert_array_equal(np.asarray(surface)[protected],texture[protected])
    assert protected.any()
    with pytest.raises(PreparationError,match='ownership is ambiguous'):
        resolve_occlusion(Image.fromarray(upper),Image.fromarray(upper),Image.fromarray(tongue))


def test_cached_argon_replay_preserves_contours_and_binds_exact_source(tmp_path,monkeypatch):
    import socket
    experiment=ROOT/'.tools/gemini-contour-argon-ei-01'
    if not (experiment/'run.json').exists():pytest.skip('Owner-operated analysis evidence is local.')
    def unexpected(*_):raise AssertionError('Replay must be offline.')
    monkeypatch.setattr(socket.socket,'connect',unexpected)
    settings=json.loads((ROOT/'docs/characters/syntelle/argon-composition.json').read_text())
    record=json.loads((ROOT/'.tools/gemini-contour-loop-ei-01/run.json').read_text())['binding']['input']
    crop={'x':734,'y':998,'width':329,'height':221}
    before=(experiment/'review/contours.json').read_bytes()
    masks,evidence=load_argon_masks(ROOT,settings,'EI',record['source_sha256'],record['raw_sha256'],crop,tmp_path/'masks')
    assert (experiment/'review/contours.json').read_bytes()==before
    assert evidence['manual_contour_adjustments'] is False and evidence['new_provider_requests']==0
    assert evidence['occlusion']['tongue_visible_pixels']>1000
    assert evidence['occlusion']['visible_semantic_overlap_pixels']==0
    assert (tmp_path/'masks/tongue.png').is_file() and (tmp_path/'masks/tongue_visible.png').is_file()
    np.testing.assert_array_equal(np.asarray(masks['tongue']),np.asarray(Image.open(tmp_path/'masks/tongue.png')))
    with pytest.raises(PreparationError,match='another original'):
        load_argon_masks(ROOT,settings,'EI','wrong-base',record['raw_sha256'],crop,tmp_path/'bad')

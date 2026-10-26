"""Scoped offline calibration: keep accepted materials and closure untouched."""
import json
from pathlib import Path

import numpy as np
from PIL import Image
import pytest

from app.core.paths import ROOT
from app.preparation.emotion_rig import prepare_emotion_rig
from app.preparation.faces import PreparationError


@pytest.mark.parametrize('states,existing', [(['neutral'],{}),(['unknown'],{}),(['disgusted'],{}),(['disgusted'],{'emotion_rig':{'version':'emotion-rig/1'}})])
def test_invalid_or_implicit_legacy_refinement_fails_before_reading_assets(states,existing):
    with pytest.raises(PreparationError, match='Refinement|Scoped refinement'):
        prepare_emotion_rig(None,None,None,None,existing,None,refine_states=states)


@pytest.fixture(scope='module')
def candidate():
    before=ROOT/'.tools/emotion-readability-87'
    after=ROOT/'.tools/disgust-readability-93'
    if not (after/'package-v6/manifest.json').exists():
        pytest.skip('Cached disgust preparation is required')
    a=json.loads((before/'prepared-v3/expressions/expressions.json').read_bytes())['emotion_rig']
    b=json.loads((after/'prepared-v6/expressions/expressions.json').read_bytes())['emotion_rig']
    return before,after,a,b


def test_nine_other_states_are_frozen(candidate):
    before,after,a,b=candidate
    assert b['refined_states']==['disgusted']
    assert a['units_per_byte']==b['units_per_byte']
    w,h=b['field_size']
    old=np.asarray(Image.open(before/'prepared-v3/expressions/emotion-fields.png')).reshape(10,h,w,4)
    new=np.asarray(Image.open(after/'prepared-v6/expressions/emotion-fields.png')).reshape(10,h,w,4)
    np.testing.assert_array_equal(old[:9],new[:9])
    assert not np.array_equal(old[9],new[9])
    for state,poses in a['oral']['states'].items():
        if state!='disgusted':assert b['oral']['states'][state]==poses
    assert a['eyelid_tension'][:9]==b['eyelid_tension'][:9]
    assert a['rest_opening'][:9]==b['rest_opening'][:9]


def test_no_independent_lip_warp_retains_canonical_shape_and_contact(candidate):
    _,_,a,b=candidate
    oral=b['oral']
    assert oral['central_contact_pinned'] and oral['dental_translation']==[0,0]
    assert not oral['tooth_warping']
    assert oral['validation']['states']['disgusted']['minimum_triangle_ratio']>.2
    assert 'disgusted' not in oral['states'], 'a zero entry would still enable joint UV resampling'
    assert oral['validation']['states']['disgusted']['canonical_geometry']


def test_masks_materials_and_geometry_are_exact(candidate):
    before,after,a,b=candidate
    for file in ['emotion-protected.png']:
        assert (before/'prepared-v3/expressions'/file).read_bytes()==(after/'prepared-v6/expressions'/file).read_bytes()
    old=json.loads((before/'package-v3/manifest.json').read_bytes())
    new=json.loads((after/'package-v6/manifest.json').read_bytes())
    assert old['geometry']==new['geometry']
    assert [k for k,v in old['textures'].items() if v['sha256']!=new['textures'][k]['sha256']]==['expressions/emotion-fields.png']
    assert b['edit_coverage']==a['edit_coverage']=='deformed-oral-coverage/1'


def test_quantized_fields_and_transitions_keep_existing_gates(candidate):
    _,_,_,b=candidate
    assert b['validation']['valid']
    assert len(b['validation']['transition_midpoints'])==45
    assert all(r['valid'] for r in b['validation']['states'].values())
    assert all(r['valid'] for r in b['validation']['transition_midpoints'].values())
    assert 0<b['eyelid_tension'][9]<=.4
    assert 0<b['rest_opening'][9]<=.4


def test_nasal_field_does_not_reach_registered_lip_contours(candidate):
    _,after,_,b=candidate
    bundle=json.loads((after/'prepared-v6/resource-bundle.json').read_bytes())
    oral=json.loads((Path(bundle['oral'])/'package.json').read_bytes())
    lip_top=oral['crop']['y']+min(float(np.asarray(p['mesh'][key])[:40,1].min())
                                 for p in oral['pairs'].values() if p.get('safe')
                                 for key in ('rest','open'))
    w,h=b['field_size']
    atlas=np.asarray(Image.open(after/'prepared-v6/expressions/emotion-fields.png')).reshape(10,h,w,4)
    ys=b['crop']['y']+(np.arange(h)+.5)*b['crop']['height']/h
    assert np.all(atlas[9,ys>=lip_top,:,:2]==128)

import json
from pathlib import Path

import numpy as np
import pytest

from app.core.paths import ROOT
from app.preparation.joint_expressions import prepare_pairs,validate_combined


@pytest.mark.parametrize('character',['syntelle','cafe','woodland'])
def test_generic_identity_preparation_uses_no_character_overrides(character):
    oral=json.loads((ROOT/f'apps/web/src/experiments/viseme-a/characters/{character}/package.json').read_text())
    expressions=json.loads((ROOT/f'apps/web/src/experiments/motion/expressions/{character}/expressions.json').read_text())
    points=np.asarray(json.loads((ROOT/f'.tools/rgbd-motion-18/prepared-current/{character}/inputs/landmarks.json').read_text())['landmarks_px'])
    package=prepare_pairs(oral,np.zeros((32,32,3)),expressions['crop'],points,points)
    assert package['validation']['valid']
    assert package['rest_opening']==0
    for pair in package['pairs'].values():
        assert not np.any(pair['rest_delta'])
        assert not np.any(pair['open_delta'])


def test_current_candidate_preserves_fv_contact_and_mbp_closure():
    root=ROOT/'.tools/expressive-performance-67/runtime-expressions'
    if not root.exists():pytest.skip('Source-bound owner fixture is not available.')
    package=json.loads((root/'expressions.json').read_text())['joint_articulation']
    oral=json.loads((ROOT/'apps/web/src/experiments/viseme-a/characters/syntelle/package.json').read_text())
    for shape in (package,package['laugh']):
        delta=np.asarray(shape['pairs']['FV-A']['rest_delta'])
        assert abs(delta[35,1])<.05
        assert shape['dental_translation']==[0.,0.]
        mesh=np.asarray(oral['pairs']['MBP-A']['mesh']['rest'])
        changed=mesh+np.asarray(shape['pairs']['MBP-A']['rest_delta'])
        assert abs((changed[35,1]-changed[25,1])-(mesh[35,1]-mesh[25,1]))<.1
    assert all(row['valid'] for row in validate_combined(oral,package,package['laugh']))


@pytest.mark.parametrize('character',['syntelle','cafe','woodland'])
def test_synthetic_nonzero_expression_keeps_the_shared_patch_boundary(character):
    oral=json.loads((ROOT/f'apps/web/src/experiments/viseme-a/characters/{character}/package.json').read_text())
    expressions=json.loads((ROOT/f'apps/web/src/experiments/motion/expressions/{character}/expressions.json').read_text())
    points=np.asarray(json.loads((ROOT/f'.tools/rgbd-motion-18/prepared-current/{character}/inputs/landmarks.json').read_text())['landmarks_px'])
    y,x=np.indices((32,32),dtype=float)
    field=np.zeros((32,32,3));field[...,0]=.6*np.sin(np.pi*x/31)*np.sin(np.pi*y/31)
    package=prepare_pairs(oral,field,expressions['crop'],points,points)
    assert package['validation']['valid']
    moved=0
    for name,pair in package['pairs'].items():
        for endpoint,key in [('rest','rest_delta'),('open','open_delta')]:
            vertices=np.asarray(oral['pairs'][name]['mesh'][endpoint]);delta=np.asarray(pair[key])
            boundary=(vertices[:,0]<1)|(vertices[:,1]<1)|(vertices[:,0]>oral['crop']['width']-1)|(vertices[:,1]>oral['crop']['height']-1)
            assert not np.any(delta[boundary])
            moved+=np.count_nonzero(delta)
    assert moved>0

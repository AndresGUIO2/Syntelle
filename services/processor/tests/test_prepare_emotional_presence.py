from copy import deepcopy
import json
import numpy as np
import pytest

from app.preparation.faces import PreparationError
from app.workflows.motion.prepare_emotional_presence import coherent_controls, oral_check, prepare

def test_coordinated_controls_reject_unprepared_smile_extrapolation():
    rig={'eyelid_tension':[0]*10,'rest_opening':[0]*10}
    before=deepcopy(rig)
    assert coherent_controls(rig,1,1)
    assert not coherent_controls(rig,1,1.25)
    assert coherent_controls(rig,2,1.17)
    assert not coherent_controls(rig,2,1.18)
    assert rig==before
    rig['rest_opening'][8]=.4
    assert not coherent_controls(rig,8,1.01)
    assert coherent_controls(rig,8,1.)

def test_emotional_lip_qualification_rejects_inverted_contact_mesh():
    points=[[0,0],[1,0],[0,1]]
    zero=[[0,0]]*3
    delta=[[0,0],[-2,0],[0,0]]
    pair={'mesh':{'rest':points,'open':points}}
    manifest={'packages':{'oral':{'pairs':{'MBP':pair}},'expressions':{
        'joint_articulation':{'pairs':{'MBP':{'rest_delta':zero,'open_delta':zero}}},
        'emotion_rig':{'oral':{'states':{'fearful':{'MBP':{'rest_delta':delta,'open_delta':delta}}},'triangles':{'MBP':[[0,1,2]]}}}}}}
    assert oral_check(manifest,'fearful',.05)['valid']
    assert not oral_check(manifest,'fearful',1)['valid']
    assert manifest['packages']['oral']['pairs']['MBP']['mesh']['rest']==points

def test_preparation_rejects_compounding_gain(tmp_path):
    source=tmp_path/'source';source.mkdir()
    (source/'manifest.json').write_text(json.dumps({'packages':{'motion':{'emotional_presence':{'version':'emotional-presence/1'}}}}))
    with pytest.raises(PreparationError,match='once'):
        prepare(source,tmp_path/'landmarks.json',tmp_path/'out')
    assert not (tmp_path/'out').exists()

import copy
import numpy as np
import pytest
from PIL import Image

from app.preparation.conversational_parallax import oral_transition, rebind
from app.preparation.faces import PreparationError


def test_transition_uses_existing_rigid_registration_and_exact_peak():
    source='a'*64
    expression={'reference_laughter':{'source_sha256':source,'oral':{}}}
    oral={'source_sha256':source,'build_id':'b'*64,'upper_translation':[0,-12],
          'poses':{'Rest':{'lower_translation':[0,4.5]}}}
    before=copy.deepcopy(oral);t=oral_transition(expression,oral,source)
    assert t['upper'][0]==[0,-12] and t['lower'][0]==[0,4.5]
    assert t['upper'][-1]==[0,0] and t['lower'][-1]==[0,0]
    assert np.all(np.diff(np.array(t['upper'])[:,1])>=0)
    assert oral==before


def test_wrong_source_cannot_receive_other_characters_registration():
    with pytest.raises(PreparationError):
        oral_transition({'reference_laughter':{'source_sha256':'a'}},{'source_sha256':'b'},'a')


def test_nested_rebinding_rejects_mixed_resources():
    value={'motion_build_id':'a','nested':[{'motion_build_id':'b'}]}
    with pytest.raises(PreparationError):rebind(value,'a','c')


def test_reference_union_cannot_remove_source_boundary_pass(tmp_path):
    from app.preparation.multiview_parallax import extend_geometry
    n=2;yy,xx=np.mgrid[0:n+1,0:n+1]
    v=np.column_stack([xx.ravel()/n,yy.ravel()/n,np.zeros(9),np.ones(9),np.zeros(9)]).astype('<f4')
    triangles=np.array([[0,1,3],[3,1,4],[1,2,4],[4,2,5],[3,4,6],[6,4,7],[4,5,7],[7,5,8]],dtype='<u2')
    raw=v.tobytes()+triangles.tobytes()
    from hashlib import sha256
    (tmp_path/'projection.bin').write_bytes(raw)
    motion={'anatomy':{'face_height':8,'chin_y':8},'projection':{'calibration':{},'grid':n,'geometry':{'file':'projection.bin','sha256':sha256(raw).hexdigest(),'vertices':9,'triangles':8,'fringe_triangles':0},
            'camera':{'size':[16,16],'center':[8,8],'distance':48,'focal':48,'near':10,'far':100,'pivot_px':[8,8]},'pivot':[0,0,0]}}
    source=np.full((16,16,4),255,np.uint8);source[:,:7,3]=128
    support=np.full((16,16,4),255,np.uint8)
    output=tmp_path/'output';output.mkdir()
    extend_geometry(motion,tmp_path,Image.fromarray(source),[support],output)
    assert motion['projection']['geometry']['fringe_triangles']>0


def test_failed_reference_stops_before_any_provider_or_database_call(tmp_path,monkeypatch):
    from hashlib import sha256
    import json
    from app.workflows.motion import prepare_parallax_reference as command
    image=tmp_path/'source.png';Image.new('RGB',(16,16),(128,128,128)).save(image)
    identity=sha256(image.read_bytes()).hexdigest()
    config=tmp_path/'config.json';config.write_text(json.dumps(dict(status='owner-authorized',limits={'image':6,'analysis':0},transport_retries=0,stop_on_provider_error=True,original=str(image),source_sha256=identity,slots={'yaw-right':'turn'})))
    need=tmp_path/'need.json';need.write_text(json.dumps(dict(source_sha256=identity,needed_slots=['yaw-right'],reason='test')))
    output=tmp_path/'requests';stage=output/'yaw-left';stage.mkdir(parents=True);(stage/'run.json').write_text('{"status":"failed"}')
    monkeypatch.setattr(command,'Archive',lambda:pytest.fail('Database/provider must not be reached after failure'))
    with pytest.raises(PreparationError,match='earlier request stopped'):
        command.request_reference(config,'yaw-right',output,need)


@pytest.mark.parametrize('slot,angles,expected',[
    ('yaw-left',[-12,0,0],True),('yaw-right',[12,0,0],True),
    ('yaw-left',[12,0,0],False),('yaw-left',[-.04,-1.2,.2],False),
    ('yaw-right',[12,8,0],False),('yaw-right',[12,0,5],False),
])
def test_pose_direction_and_cross_axis_are_qualified(slot,angles,expected):
    from app.preparation.reference_policy import load_policy,assess_pose
    policy,_=load_policy();points=np.zeros((478,3));points[33,:2]=[30,40];points[263,:2]=[70,40]
    got=assess_pose(policy,slot,angles,points,points,(100,100))
    assert got['accepted']==expected


def test_one_reference_cannot_qualify_both_directions():
    from app.preparation.reference_policy import load_policy,direction_coverage,require_direction_coverage
    policy,_=load_policy();right=dict(slot='yaw-right',accepted=True,achieved_degrees=[13.4,0,0])
    incomplete=direction_coverage(policy,[right])
    assert incomplete['maximum_symmetric_yaw']==0 and incomplete['missing_slots']==['yaw-left']
    with pytest.raises(PreparationError,match='Incomplete measured yaw coverage'):require_direction_coverage(incomplete)
    left=dict(slot='yaw-left',accepted=True,achieved_degrees=[-11,0,0])
    full=direction_coverage(policy,[right,left]);require_direction_coverage(full)
    assert full['maximum_symmetric_yaw']==9


def test_view_material_does_not_cover_the_opposite_side():
    from app.preparation.reference_policy import lateral_visibility
    x=np.array([10,50,90])
    assert np.array_equal(lateral_visibility(x,50,80,12),[1,.5,0])
    assert np.array_equal(lateral_visibility(x,50,80,-12),[0,.5,1])


def test_revised_prompt_preview_preserves_budget_and_uses_viewer_coordinates(tmp_path,monkeypatch):
    import json
    from hashlib import sha256
    from app.workflows.motion import prepare_parallax_reference as command
    source=tmp_path/'source.png';Image.new('RGB',(16,16)).save(source)
    config=tmp_path/'config.json';config.write_text(json.dumps(dict(original=str(source),source_sha256=sha256(source.read_bytes()).hexdigest())))
    original=config.read_bytes();monkeypatch.setattr(command,'Archive',lambda:pytest.fail('A prompt preview must not reach the ledger/provider'))
    result=command.preview_prompt(config,'yaw-left',tmp_path/'preview')
    prompt=(tmp_path/'preview/prompt.txt').read_text(encoding='utf-8')
    assert prompt == 'Turn the head 12 degrees toward the viewer\u2019s left. Keep the image identical except for this change. Rotate the head itself, not just the eyes. Let the gaze follow the head turn naturally.'
    policy,_=command.load_policy()
    assert command.prompt_for(policy,'yaw-right') == prompt.replace('left','right')
    assert not policy['prompt_prefix'] and 'image 2' not in prompt.lower()
    assert result['provider_calls']==0 and result['authorization'] is False and config.read_bytes()==original


def test_export_cannot_bypass_missing_or_excessive_direction_coverage():
    from app.preparation.reference_policy import validate_export_coverage
    motion={'projection':{'bounds':{'yaw':8},'multiview':{'views':[{'axis':0,'pose':[13,0,0]}]}}}
    with pytest.raises(PreparationError,match='Incomplete measured yaw coverage'):validate_export_coverage(motion)
    motion['projection']['multiview']['views'].append({'axis':0,'pose':[-11,0,0]})
    validate_export_coverage(motion)
    motion['projection']['bounds']['yaw']=10
    with pytest.raises(PreparationError,match='exceeds the measured'):validate_export_coverage(motion)


def test_historical_config_cannot_acquire_with_the_old_prompt(tmp_path,monkeypatch):
    import json
    from hashlib import sha256
    from app.workflows.motion import prepare_parallax_reference as command
    source=tmp_path/'source.png';Image.new('RGB',(16,16)).save(source);identity=sha256(source.read_bytes()).hexdigest()
    config=tmp_path/'config.json';config.write_text(json.dumps(dict(status='owner-authorized',limits={'image':6,'analysis':0},transport_retries=0,stop_on_provider_error=True,original=str(source),source_sha256=identity,slots={'yaw-left':'historical prompt'})))
    need=tmp_path/'need.json';need.write_text(json.dumps(dict(source_sha256=identity,needed_slots=['yaw-left'],reason='a visible turn is missing')))
    monkeypatch.setattr(command,'Archive',lambda:pytest.fail('An unbound prompt must not reach the ledger/provider'))
    with pytest.raises(PreparationError,match='revised reference policy'):command.request_reference(config,'yaw-left',tmp_path/'requests',need)

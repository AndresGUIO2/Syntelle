import base64
from hashlib import sha256
from io import BytesIO
import json

import numpy as np
from PIL import Image
import pytest

from app import gemini_contour_loop as loop
from app.preparation.faces import PreparationError


def prediction(crossed=False):
    upper=[[150,200],[850,200],[850,400],[150,400]]
    if crossed:upper=[[150,200],[850,400],[150,400],[850,200]]
    return {'coordinate_space':'normalized-full-input-image-xy-0-1000','regions':[
        {'label':'upper_teeth','visible':True,'polygons':[upper],'notes':'Proposal'},
        {'label':'lower_teeth','visible':True,'polygons':[[[200,600],[800,600],[800,700],[200,700]]],'notes':''},
        {'label':'tongue','visible':False,'polygons':[],'notes':'No visible tongue'}]}


@pytest.fixture
def source(tmp_path):
    folder=tmp_path/'ei';folder.mkdir()
    Image.new('RGB',(100,100),(140,90,75)).save(folder/'raw.png')
    record={'viseme':'EI','source_sha256':'original-base','canvas':{'width':200,'height':200},
            'generation':{'file':'raw.png','raw_sha256':sha256((folder/'raw.png').read_bytes()).hexdigest()},
            'metrics':{'transform':[[1,0,0],[0,1,0]]}}
    points=np.zeros((478,3))
    from app.preparation.mesh import INNER,OUTER
    for ids,radius in [(OUTER,20),(INNER,10)]:
        for i,theta in zip(ids,np.linspace(0,2*np.pi,20,endpoint=False)):
            points[i,:2]=[100+40*np.cos(theta),100+radius*np.sin(theta)]
    points[61,:2]=[60,100];points[291,:2]=[140,100]
    (folder/'run.json').write_text(json.dumps(record))
    (folder/'variant-landmarks.json').write_text(json.dumps({'landmarks_px':points.tolist()}))
    return folder


def test_enlargement_grid_and_lip_views_share_frame_without_manual_dental_annotations(source):
    settings=json.loads(loop.SETTINGS.read_text(encoding='utf-8'))
    native,evidence=loop.original_crop(source,settings)
    views,lips=loop.input_views(source,native,evidence,settings)
    original=Image.open(BytesIO(native)).convert('RGB');size=(original.width*4,original.height*4)
    for name,data in views.items():assert Image.open(BytesIO(data)).size==size,name
    np.testing.assert_array_equal(np.array(Image.open(BytesIO(views['clean.png']))),np.array(original.resize(size,Image.Resampling.LANCZOS)))
    assert views['grid.png']!=views['clean.png'] and views['mediapipe-lips.png']!=views['clean.png']
    assert lips['manual_mask_inputs'] is False
    np.testing.assert_allclose(lips['display_to_original'],np.asarray(evidence['input_to_original'])@np.diag([.25,.25,1]))
    request=loop.round_request('proposal',views,settings)
    assert len(request['input'])==4 and request['response_format']['type']=='text'
    assert all(part['resolution']=='high' for part in request['input'][1:])
    assert request['store'] is False and request['generation_config']['thinking_level']=='medium'


def test_two_requests_include_exact_own_overlay_and_validation_then_reuse_cache(source,tmp_path,monkeypatch):
    calls=[]
    def post(payload,settings,key):
        calls.append(payload)
        result=prediction(crossed=len(calls)==1)
        return json.dumps({'output_text':json.dumps(result),'usage':{'total_tokens':123}}).encode(),200,.1
    monkeypatch.setattr(loop,'post_once',post)
    monkeypatch.setattr(loop,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    output=tmp_path/'output';record=loop.prepare(source,output,True)
    assert len(calls)==2 and record['provider_requests']==2
    assert len(calls[0]['input'])==4 and len(calls[1]['input'])==5
    assert base64.b64decode(calls[1]['input'][-1]['data'])==(output/'correction-feedback.png').read_bytes()
    assert 'degenerate or crossing contour' in calls[1]['input'][0]['text']
    assert json.dumps(prediction(True)) in calls[1]['input'][0]['text']
    assert record['rounds']['proposal']['validation']['status']=='rejected_geometry'
    assert record['rounds']['correction']['validation']['status']=='unreviewed_proposal'
    assert record['manual_contour_adjustments'] is False and record['active_mask_installation'] is False
    before=(output/'run.json').read_bytes()
    def unexpected(*_):raise AssertionError('Cached replay must not contact Gemini.')
    monkeypatch.setattr(loop,'post_once',unexpected)
    loop.prepare(source,output,True)
    assert (output/'run.json').read_bytes()==before
    assert 'private-test-key' not in ''.join(p.read_text(encoding='utf-8') for p in output.rglob('*.json'))
    (output/'correction-feedback.png').write_bytes(b'changed')
    with pytest.raises(PreparationError,match='evidence changed'):loop.prepare(source,output,True)


def test_correction_failure_preserves_first_round_and_never_retries_either_stage(source,tmp_path,monkeypatch):
    calls=[]
    def post(*_):
        calls.append(1)
        if len(calls)==1:return json.dumps({'output_text':json.dumps(prediction())}).encode(),200,.1
        return b'{"error":"synthetic provider error"}',429,.1
    monkeypatch.setattr(loop,'post_once',post)
    monkeypatch.setattr(loop,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    output=tmp_path/'output'
    for _ in range(2):
        with pytest.raises(PreparationError):loop.prepare(source,output,True)
    assert calls==[1,1] and (output/'proposal/review.png').is_file()
    assert json.loads((output/'proposal/run.json').read_text())['status']=='received'
    assert json.loads((output/'correction/run.json').read_text())['status']=='failed'
    assert json.loads((output/'run.json').read_text())['status']=='stopped'


def test_incorrect_stage_budget_stops_before_network_or_output(source,tmp_path,monkeypatch):
    settings=json.loads(loop.SETTINGS.read_text());settings['maximum_requests']=3
    path=tmp_path/'settings.json';path.write_text(json.dumps(settings))
    def unexpected(*_):raise AssertionError('Invalid budget must not reach provider.')
    monkeypatch.setattr(loop,'post_once',unexpected)
    with pytest.raises(PreparationError,match='one correction'):
        loop.prepare(source,tmp_path/'output',True,settings_path=path)
    assert not (tmp_path/'output').exists()


def test_http_success_with_incomplete_output_is_preserved_and_never_retried(source,tmp_path,monkeypatch):
    calls=[]
    def post(*_):
        calls.append(1)
        if len(calls)==1:return json.dumps({'output_text':json.dumps(prediction())}).encode(),200,.1
        return json.dumps({'status':'incomplete','output_text':'{"coordinate_space":',
                           'usage':{'total_thought_tokens':7860,'total_output_tokens':315}}).encode(),200,.1
    monkeypatch.setattr(loop,'post_once',post)
    monkeypatch.setattr(loop,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    output=tmp_path/'output'
    with pytest.raises(PreparationError,match='response is incomplete'):loop.prepare(source,output,True)
    with pytest.raises(PreparationError,match='no retry'):loop.prepare(source,output,True)
    record=json.loads((output/'correction/run.json').read_text())
    assert len(calls)==2 and record['status']=='failed' and record['provider_status']=='incomplete'
    assert record['usage']['total_thought_tokens']==7860
    assert (output/'correction/output.txt').read_text()=='{"coordinate_space":'
    assert not (output/'correction/contours.json').exists()


def test_argon_review_uses_same_views_and_one_call_then_reuses_cache(source,tmp_path,monkeypatch):
    from app import gemini_contour_review as review
    calls=[]
    def post(payload,*_):
        calls.append(payload)
        return json.dumps({'status':'completed','output_text':json.dumps(prediction()),'usage':{'total_tokens':123}}).encode(),200,.1
    monkeypatch.setattr(loop,'post_once',post)
    monkeypatch.setattr(loop,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    prepared=tmp_path/'loop';loop.prepare(source,prepared,True)
    calls.clear();output=tmp_path/'review'
    result=review.prepare(prepared,output,True)
    assert len(calls)==1 and calls[0]['model']=='gemini-4-argon'
    assert len(calls[0]['input'])==5 and calls[0]['generation_config']['max_output_tokens']==24576
    for index,name in enumerate(('clean.png','grid.png','mediapipe-lips.png','correction-feedback.png'),1):
        assert base64.b64decode(calls[0]['input'][index]['data'])==(prepared/name).read_bytes()
    assert result['new_viseme_generations']==0 and result['active_mask_installation'] is False
    assert (output/'comparison.png').is_file()
    review.prepare(prepared,output,True)
    assert len(calls)==1
    (output/'review/mask-overlay.png').write_bytes(b'changed')
    with pytest.raises(PreparationError,match='evidence changed'):review.prepare(prepared,output,True)


def test_argon_review_failure_stops_and_preserves_source(source,tmp_path,monkeypatch):
    from app import gemini_contour_review as review
    monkeypatch.setattr(loop,'post_once',lambda *_: (json.dumps({'output_text':json.dumps(prediction())}).encode(),200,.1))
    monkeypatch.setattr(loop,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    prepared=tmp_path/'loop';loop.prepare(source,prepared,True)
    before=(prepared/'run.json').read_bytes();calls=[]
    def fail(*_):calls.append(1);return b'{"error":"synthetic error"}',403,.1
    monkeypatch.setattr(loop,'post_once',fail)
    output=tmp_path/'review'
    for _ in range(2):
        with pytest.raises(PreparationError):review.prepare(prepared,output,True)
    assert calls==[1] and (prepared/'run.json').read_bytes()==before
    assert (output/'review/response.json').is_file()

import base64
from hashlib import sha256
from io import BytesIO
import json

import numpy as np
from PIL import Image
import pytest

from app import prepare_motion
from app.preparation.eyes import render_eyes, EYE_ANCHORS, EYE_RINGS
from app.preparation.faces import PreparationError
from app.preparation.motion_assets import binding,cached,save_record,refine_subject
from app.preparation.motion_geometry import deform,validate


def test_eye_coverage_keeps_exposed_iris_fixed_and_preserves_unedited_pixels():
    a=np.zeros((50,80,3),np.uint8);a[:]=[150,95,70]
    a[15:35,20:60]=[230,230,230];a[20:30,32:43]=[24,90,120]
    b=np.full_like(a,[160,103,78]);mask=np.zeros((50,80),np.uint8);mask[5:45,15:65]=255
    curves=np.zeros((80,4));curves[20:60]=[15,35,25,1]
    original=Image.fromarray(a);closed=Image.fromarray(b);m=Image.fromarray(mask)
    np.testing.assert_array_equal(np.asarray(render_eyes(original,closed,m,curves,0)),a)
    for amount in [.01,.25,.5,.75,.99,1]:
        result=np.asarray(render_eyes(original,closed,m,curves,amount))
        np.testing.assert_array_equal(result[mask==0],a[mask==0])
        if amount<=.5:np.testing.assert_array_equal(result[24:27,34:41],a[24:27,34:41])
    result=np.asarray(render_eyes(original,closed,m,curves,1))
    np.testing.assert_array_equal(result[mask==255],b[mask==255])
    almost=np.asarray(render_eyes(original,closed,m,curves,.9999)).astype(int)
    assert np.max(np.abs(almost-result.astype(int)))<=1


def test_alignment_excludes_moving_eye_and_mouth_points():
    eye_points=set(i for ring in EYE_RINGS for i in ring)
    assert not eye_points.intersection(EYE_ANCHORS)
    assert not set([13,14,61,291,78,308]).intersection(EYE_ANCHORS)


def test_foreground_refinement_preserves_opaque_dark_material():
    rgb=np.full((100,120,3),235,np.uint8);rgb[20:95,30:90]=[15,12,22]
    probability=np.full((100,120),.083,np.float32);probability[20:95,30:90]=.983
    alpha,result=refine_subject(rgb,probability)
    assert alpha[45,60]==1 and alpha[5,5]==0
    np.testing.assert_array_equal(result[45,60],rgb[45,60])
    assert alpha[45,60]>alpha[5,5]  # Coverage follows semantics, not material brightness.


def test_cache_is_bound_to_source_model_algorithm_and_actual_output_bytes(tmp_path):
    image=Image.new('RGB',(20,30),'red');key=binding(image,{'model':'one'})
    (tmp_path/'depth.png').write_bytes(b'actual-depth-bytes')
    record=save_record(tmp_path,'record.json',{'seconds':3},key,['depth.png'])
    assert cached(tmp_path,'record.json',key)==record
    assert cached(tmp_path,'record.json',binding(Image.new('RGB',(20,30),'blue'),{'model':'one'})) is None
    assert cached(tmp_path,'record.json',binding(image,{'model':'two'})) is None
    (tmp_path/'depth.png').write_bytes(b'changed')
    with pytest.raises(PreparationError,match='cached motion resource changed'):cached(tmp_path,'record.json',key)


def test_bounded_motion_has_no_inversions_and_breathing_does_not_scale_face():
    anatomy={'head_pivot':[100,150],'chin_y':150,'face_width':100,'face_height':120}
    settings={'yaw_degrees':1,'pitch_degrees':.6,'roll_degrees':.6,'breath_face_height_fraction':.006}
    depth=Image.fromarray(np.tile(np.arange(200,dtype=np.uint8),(300,1)))
    result=validate(depth,anatomy,(200,300),settings)
    assert result['minimum_signed_double_area']>0 and result['triangles_checked']==81*8192
    points=np.array([[75.,60.],[125.,120.],[100.,150.],[50.,240.]])
    moved=deform(points,np.full(4,.5),anatomy,(200,300),[0,0,0],1)
    np.testing.assert_array_equal(moved[:3],points[:3]);assert moved[3,1]>points[3,1]
    np.testing.assert_array_equal(deform(points,np.full(4,.5),anatomy,(200,300),[0,0,0],0),points)


def generation_setup(tmp_path,monkeypatch):
    monkeypatch.setattr(prepare_motion,'dotenv_values',lambda _: {'GEMINI_API_KEY':'test-key'})
    original=tmp_path/'original.png';im=Image.new('RGB',(64,64),'#886655');im.save(original)
    points=np.zeros((478,3));points[:468,:2]=np.linspace([15,10],[50,55],468)
    directory=tmp_path/'case';directory.mkdir()
    return original,im,points,directory,{'model':'test-model'}


def test_request_uses_exact_original_first_preserves_prompt_and_reuses_cache(tmp_path,monkeypatch):
    original,im,points,directory,config=generation_setup(tmp_path,monkeypatch);calls=[]
    raw=BytesIO();im.save(raw,format='PNG')
    def provider(payload,settings,key,stage):
        calls.append(payload)
        assert base64.b64decode(payload['input'][1]['data'])==original.read_bytes()
        assert payload['input'][2]['mime_type']=='image/png' and settings['retries']==0
        return json.dumps({'output_image':{'data':base64.b64encode(raw.getvalue()).decode()},'usage':{'images':1}}).encode(),200,{}
    result=prepare_motion.generation(original,im,points,directory,provider,config,True)
    again=prepare_motion.generation(original,im,points,directory,provider,config,False)
    assert result==again and len(calls)==1
    assert result['original_sha256']==sha256(original.read_bytes()).hexdigest()
    assert (directory/'proposal/prompt.txt').read_text()==calls[0]['input'][0]['text']
    assert json.loads((directory/'proposal/request.json').read_text())==calls[0]


def test_provider_error_is_preserved_and_cannot_be_retried(tmp_path,monkeypatch):
    original,im,points,directory,config=generation_setup(tmp_path,monkeypatch);calls=[]
    def provider(*_):calls.append(True);return b'{"error":"failure"}',503,{}
    with pytest.raises(PreparationError,match='stopped without retry'):prepare_motion.generation(original,im,points,directory,provider,config,True)
    assert json.loads((directory/'proposal/run.json').read_text())['status']=='failed'
    with pytest.raises(PreparationError,match='cannot be retried'):prepare_motion.generation(original,im,points,directory,provider,config,True)
    assert len(calls)==1

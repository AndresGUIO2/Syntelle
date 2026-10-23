import base64
from hashlib import sha256
from io import BytesIO
import json

import httpx
import numpy as np
from PIL import Image
import pytest

from app import gemini_contour_trial as trial
from app.preparation.faces import PreparationError


def prediction():
    return {'coordinate_space':'normalized-full-input-image-xy-0-1000','regions':[
        {'label':'upper_teeth','visible':True,'polygons':[[[150,200],[850,200],[850,400],[150,400]]],'notes':''},
        {'label':'lower_teeth','visible':True,'polygons':[[[200,600],[800,600],[800,700],[200,700]]],'notes':''},
        {'label':'tongue','visible':False,'polygons':[],'notes':'No visible tongue.'}]}


@pytest.fixture
def source(tmp_path):
    folder=tmp_path/'ei';folder.mkdir()
    Image.new('RGB',(100,100),(140,90,75)).save(folder/'raw.png')
    record={'viseme':'EI','source_sha256':'original-base','canvas':{'width':200,'height':200},
            'generation':{'file':'raw.png','raw_sha256':sha256((folder/'raw.png').read_bytes()).hexdigest()},
            'metrics':{'transform':[[1,0,0],[0,1,0]]}}
    points=np.zeros((478,3))
    for i,theta in zip(trial.OUTER,np.linspace(0,2*np.pi,20,endpoint=False)):
        points[i,:2]=[100+40*np.cos(theta),100+20*np.sin(theta)]
    points[61,:2]=[60,100];points[291,:2]=[140,100]
    (folder/'run.json').write_text(json.dumps(record))
    (folder/'variant-landmarks.json').write_text(json.dumps({'landmarks_px':points.tolist()}))
    return folder


def test_request_is_text_segmentation_of_exact_original_crop_without_annotations(source):
    settings=json.loads(trial.SETTINGS.read_text(encoding='utf-8'))
    image,evidence=trial.original_crop(source,settings)
    bounds=evidence['crop'];original=Image.open(source/'raw.png').convert('RGB')
    expected=original.crop((bounds['x'],bounds['y'],bounds['x']+bounds['width'],bounds['y']+bounds['height']))
    np.testing.assert_array_equal(np.asarray(Image.open(BytesIO(image))),np.asarray(expected))
    request=trial.build_request(image,'test prompt',settings)
    assert [i['type'] for i in request['input']]==['text','image']
    assert base64.b64decode(request['input'][1]['data'])==image
    assert request['response_format']['type']=='text' and request['response_format']['mime_type']=='application/json'
    assert request['store'] is False and evidence['resized'] is False
    assert 'image_size' not in json.dumps(request)
    assert all(keyword not in json.dumps(request['response_format']['schema']) for keyword in ('$ref','$defs','prefixItems','const'))


def test_one_request_is_cached_and_corrupted_cache_cannot_trigger_another(source,tmp_path,monkeypatch):
    calls=[]
    def post(payload,settings,key):
        calls.append(payload)
        return json.dumps({'output_text':json.dumps(prediction()),'usage':{'total_tokens':123}}).encode(),200,0.2
    monkeypatch.setattr(trial,'post_once',post)
    monkeypatch.setattr(trial,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    output=tmp_path/'output'
    first=trial.prepare(source,output,True)
    second=trial.prepare(source,output)
    assert len(calls)==1 and first['attempts']==second['attempts']==1
    assert first['usage']=={'total_tokens':123}
    assert first['quality']['manual_adjustments'] is False
    assert 'private-test-key' not in ''.join(p.read_text(encoding='utf-8') for p in output.glob('*.json'))
    assert (output/'prompt.txt').read_bytes()==trial.PROMPT.read_bytes()
    assert (output/'settings.json').read_bytes()==trial.SETTINGS.read_bytes()
    (output/'output.txt').write_text('{}')
    with pytest.raises(PreparationError,match='evidence changed'):
        trial.prepare(source,output,True)
    assert len(calls)==1


def test_provider_failure_has_no_retry_and_cannot_be_repeated(source,tmp_path,monkeypatch):
    calls=[]
    def post(*args):
        calls.append(1);return b'{"error":"synthetic failure"}',429,0.1
    monkeypatch.setattr(trial,'post_once',post)
    monkeypatch.setattr(trial,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    output=tmp_path/'output'
    for _ in range(2):
        with pytest.raises(PreparationError):trial.prepare(source,output,True)
    assert calls==[1]
    record=json.loads((output/'run.json').read_text())
    assert record['status']=='failed' and record['http_status']==429
    assert (output/'response.json').read_bytes()==b'{"error":"synthetic failure"}'


def test_transport_posts_once_with_no_image_output_and_redacts_provider_echo(monkeypatch):
    client=httpx.Client;calls=[]
    def respond(request):
        calls.append(request)
        return httpx.Response(400,json={'error':'private-test-key'})
    def make_client(**kwargs):
        assert kwargs['follow_redirects'] is False
        kwargs['transport']=httpx.MockTransport(respond)
        return client(**kwargs)
    monkeypatch.setattr(httpx,'Client',make_client)
    settings=json.loads(trial.SETTINGS.read_text())
    response,status,_=trial.post_once({'model':settings['model']},settings,'private-test-key')
    assert len(calls)==1 and calls[0].method=='POST' and status==400
    assert b'private-test-key' not in response and b'[redacted]' in response


def test_invalid_or_ambiguous_contours_are_not_silently_repaired(source,tmp_path):
    settings=json.loads(trial.SETTINGS.read_text())
    image,evidence=trial.original_crop(source,settings)
    bad=prediction();bad['regions'][0]['polygons']=[[[100,100],[900,900],[100,900],[900,100]]]
    with pytest.raises(PreparationError,match='crossing'):
        trial.render_contours(image,trial.Contours.model_validate(bad),evidence,settings,tmp_path)
    overlap=prediction();overlap['regions'][1]['polygons']=overlap['regions'][0]['polygons']
    with pytest.raises(PreparationError,match='overlap'):
        trial.render_contours(image,trial.Contours.model_validate(overlap),evidence,settings,tmp_path)
    invalid=prediction();invalid['regions'][0]['polygons'][0][0][0]=1001
    with pytest.raises(ValueError):trial.Contours.model_validate(invalid)
    absent=prediction();absent['regions'][2]['polygons']=[[[0,0],[1,0],[0,1]]]
    with pytest.raises(ValueError):trial.Contours.model_validate(absent)


def test_overlay_preserves_unmasked_pixels_and_records_coordinate_transforms(source,tmp_path):
    settings=json.loads(trial.SETTINGS.read_text())
    image,evidence=trial.original_crop(source,settings)
    quality=trial.render_contours(image,trial.Contours.model_validate(prediction()),evidence,settings,tmp_path)
    raw=np.asarray(Image.open(BytesIO(image)));overlay=np.asarray(Image.open(tmp_path/'mask-overlay.png'))
    masks=[np.asarray(Image.open(tmp_path/f'{label}-mask.png')) for label in settings['render']['colors']]
    outside=np.maximum.reduce(masks)==0
    np.testing.assert_array_equal(raw[outside],overlay[outside])
    assert not masks[2].any() and quality['changed_pixels_outside_masks']==0
    coords=json.loads((tmp_path/'coordinates.json').read_text())['regions']['upper_teeth']
    first=coords['input_pixels'][0][0]
    np.testing.assert_allclose(coords['original_pixels'][0][0],np.array(first)+[evidence['crop']['x'],evidence['crop']['y']])
    np.testing.assert_allclose(coords['aligned_character_pixels'][0][0],np.array(coords['original_pixels'][0][0])*2)


def test_unsupported_model_setting_is_rejected_before_any_request(source,tmp_path,monkeypatch):
    settings=json.loads(trial.SETTINGS.read_text())
    settings['generation_config']['thinking_level']='minimal'
    path=tmp_path/'settings.json';path.write_text(json.dumps(settings))
    def unexpected(*_):raise AssertionError('Unsupported setting must never reach the provider.')
    monkeypatch.setattr(trial,'post_once',unexpected)
    with pytest.raises(PreparationError,match='minimal is unsupported'):
        trial.prepare(source,tmp_path/'output',True,settings_path=path)
    assert not (tmp_path/'output/run.json').exists()


def test_rejected_response_can_only_be_inspected_offline_without_repair_or_acceptance(source,tmp_path,monkeypatch):
    calls=[];bad=prediction();bad['regions'][0]['polygons']=[[[100,100],[900,900],[100,900],[900,100]]]
    def post(*_):
        calls.append(1);return json.dumps({'output_text':json.dumps(bad)}).encode(),200,0.1
    monkeypatch.setattr(trial,'post_once',post)
    monkeypatch.setattr(trial,'dotenv_values',lambda _: {'GEMINI_API_KEY':'private-test-key'})
    output=tmp_path/'output'
    with pytest.raises(PreparationError,match='validation failed'):trial.prepare(source,output,True)
    before=(output/'run.json').read_bytes()
    quality=trial.inspect_cached(output)
    assert quality['status']=='rejected_geometry' and not quality['manual_adjustments']
    assert len(quality['issues'])>=1 and (output/'inspection/review.png').is_file()
    assert (output/'run.json').read_bytes()==before
    assert json.loads((output/'inspection/coordinates.json').read_text())['regions']['upper_teeth']['normalized']==bad['regions'][0]['polygons']
    with pytest.raises(PreparationError,match='Previous request incomplete or failed'):
        trial.prepare(source,output,True)
    assert calls==[1]

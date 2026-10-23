"""Independent invariants for automatic oral separation and rigid placement."""
import json
from hashlib import sha256
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw
import pytest

from app.preparation.semantic_mouth import OralContours, LABELS, rasterize, validate_masks
from app.preparation.semantic_rig import fit_lower, fit_upper, aperture_sdf, reconstruct_cavity, enforce_bite, pose_quality
from app.preparation.faces import PreparationError


def rect(box, size=(80,60)):
    image=Image.new('L',size);ImageDraw.Draw(image).rectangle(box,fill=255);return image


def regions():
    return {'outer_lips':rect((8,8,70,50)), 'aperture':rect((15,18,65,40)),
            'upper_teeth':rect((25,18,55,23)), 'lower_teeth':rect((25,35,55,40)),
            'tongue':rect((35,27,45,33))}


def test_semantics_rejects_a_dental_mask_outside_the_real_opening():
    masks=regions();masks['upper_teeth']=rect((20,5,60,10))
    assert not validate_masks(masks,'A')['valid']


def test_absent_teeth_and_tongue_are_valid_not_a_detection_failure():
    masks=regions()
    for label in ('upper_teeth','lower_teeth','tongue'):masks[label]=Image.new('L',(80,60))
    assert validate_masks(masks,'U')['valid']
    masks['aperture']=Image.new('L',(80,60))
    assert validate_masks(masks,'MBP')['valid']
    assert not np.asarray(aperture_sdf(masks['aperture'])).any()


def test_lower_row_follows_measured_incisa_not_a_fixed_pose_offset():
    protected=rect((25,30,55,40));completed=rect((10,29,70,55))
    opening=rect((20,20,60,40));observed=rect((25,35,55,40))
    assert fit_lower(protected,completed,opening,observed)==5
    observed=Image.new('L',(80,60))
    shift=fit_lower(protected,completed,opening,observed)
    assert shift==12 # first completed pixel y29 must stay behind lower lip y40
    assert 29+shift>40


def test_upper_arch_uses_one_robust_translation_from_broad_observations():
    protected=rect((15,10,65,22))
    observed=[rect((15,10,65,25)),rect((15,12,65,25)),rect((35,8,42,42))]
    shift,report=fit_upper(protected,observed)
    assert shift==3;assert len(report['weights'])==2


def test_bite_stop_prevents_rows_interpenetrating_without_moving_upper():
    upper=rect((20,10,60,25));lower=rect((20,30,60,45));aperture=rect((15,12,65,40))
    translation=enforce_bite(upper,lower,0,-10,aperture)
    assert 30+translation>25 and translation>-10


def test_wrong_fv_is_reported_instead_of_disguised_as_contact():
    report=pose_quality('FV',regions())
    assert not report['ready'] and 'lower dental row' in report['issues'][0]
    masks=regions();masks['lower_teeth']=Image.new('L',(80,60));masks['upper_teeth']=rect((25,18,55,40))
    assert pose_quality('FV',masks)['ready']


def test_cavity_reconstruction_preserves_tongue_lips_and_dark_enamel_alpha():
    masks=regions();raw=np.full((60,80,3),(120,70,65),np.uint8)
    aperture=np.asarray(masks['aperture'])>=128;raw[aperture]=(32,15,20)
    teeth=(np.asarray(masks['upper_teeth'])>=128)|(np.asarray(masks['lower_teeth'])>=128)
    raw[teeth]=(150,130,115);tongue=np.asarray(masks['tongue'])>=128;raw[tongue]=(140,65,80)
    image,removal=reconstruct_cavity(Image.fromarray(raw),masks)
    result=np.asarray(image)
    assert np.array_equal(result[tongue],raw[tongue])
    assert np.array_equal(result[~aperture],raw[~aperture])
    assert np.asarray(removal)[~aperture].max()==0
    assert result[teeth].mean()<70


def test_crossing_contours_fail_instead_of_becoming_a_visible_region():
    raw={'notes':'test','regions':[{'label':k,'visible':False,'polygons':[]} for k in LABELS]}
    raw['regions'][0].update(visible=True,polygons=[[[100,100],[900,900],[900,100],[100,900]]])
    with pytest.raises(PreparationError):rasterize(OralContours.model_validate(raw),Image.new('RGB',(80,60)))


def test_edge_refinement_is_bounded_and_keeps_original_response_immutable():
    raw={'notes':'test','regions':[{'label':k,'visible':False,'polygons':[]} for k in LABELS]}
    raw['regions'][0].update(visible=True,polygons=[[[100,100],[900,100],[900,900],[100,900]]])
    contours=OralContours.model_validate(raw);before=contours.model_dump_json()
    _,report=rasterize(contours,Image.new('RGB',(80,60)))
    assert max(v['max_native_px'] for v in report.values())<=1
    assert contours.model_dump_json()==before


@pytest.fixture
def semantic_request(tmp_path,monkeypatch):
    from app import oral_semantics as module
    from app import gemini_contour_loop as network
    manifest=tmp_path/'input.json';manifest.write_text('{}')
    blank=Image.new('RGB',(80,60),(120,70,65))
    monkeypatch.setattr(module,'load_inputs',lambda _: ({},blank,{'MBP':blank},{},{'MBP':{'original_sha256':'same-original'}}))
    monkeypatch.setenv('GEMINI_API_KEY','secret-test-credential')
    calls=[]
    def post(payload,*_):
        calls.append(payload)
        result={'notes':'closed lips','regions':[{'label':k,'visible':k=='outer_lips','polygons':[[[100,100],[900,100],[900,900],[100,900]]] if k=='outer_lips' else []} for k in LABELS]}
        return json.dumps({'output_text':json.dumps(result),'usage':{'total_tokens':123}}).encode(),200,.1
    monkeypatch.setattr(network,'post_once',post)
    return module,network,manifest,tmp_path/'analysis',calls


def test_replay_uses_cache_and_correction_receives_its_own_overlay(semantic_request):
    module,_,manifest,output,calls=semantic_request
    module.analyze(manifest,output,['MBP'],request=True)
    module.analyze(manifest,output,['MBP'],request=True)
    assert len(calls)==1
    module.analyze(manifest,output,['MBP'],request=True,correction=True)
    assert len(calls)==2 and len(calls[1]['input'])==4
    assert 'Prior result' in calls[1]['input'][0]['text']
    assert all('secret-test-credential' not in p.read_text() for p in output.rglob('*.json'))
    module.analyze(manifest,output,['MBP'],request=True,correction=True)
    assert len(calls)==2


def test_request_budget_and_provider_failures_never_retry(semantic_request):
    module,network,manifest,output,calls=semantic_request
    attempts=[]
    def failure(*_):attempts.append(1);return b'{"error":"quota"}',429,.1
    network.post_once=failure
    for _ in range(2):
        with pytest.raises(PreparationError):module.analyze(manifest,output,['MBP'],request=True)
    assert len(attempts)==1
    # Failed attempts consume the batch allowance too.
    for i in range(17):
        folder=output/f'failed-{i}'/'proposal';folder.mkdir(parents=True);(folder/'run.json').write_text('{}')
    with pytest.raises(PreparationError,match='budget'):
        module.analyze(manifest,output,['MBP'],request=True,correction=True)
    assert len(attempts)==1


def test_invalid_geometry_gets_only_one_automatic_visual_correction(semantic_request,monkeypatch):
    module,network,manifest,output,calls=semantic_request
    original=network.post_once
    def invalid(payload,*args):
        data,status,duration=original(payload,*args)
        body=json.loads(data);contours=json.loads(body['output_text'])
        contours['regions'][0]['polygons']=[[[100,100],[900,900],[900,100],[100,900]]]
        body['output_text']=json.dumps(contours)
        return json.dumps(body).encode(),status,duration
    monkeypatch.setattr(network,'post_once',invalid)
    with pytest.raises(PreparationError,match='single correction'):
        module.analyze(manifest,output,['MBP'],request=True)
    assert len(calls)==2
    assert (output/'mbp/proposal/response.json').exists() and (output/'mbp/correction/response.json').exists()


@pytest.fixture
def cached_character(tmp_path):
    image=Image.new('RGB',(80,60),(120,80,65));original=tmp_path/'original.png';image.save(original)
    raw=tmp_path/'raw.png';image.save(raw);source_hash=sha256(original.read_bytes()).hexdigest()
    points={'landmarks_px':np.zeros((478,3)).tolist()}
    for name in ('base-landmarks.json','variant-landmarks.json'):(tmp_path/name).write_text(json.dumps(points))
    record={'viseme':'EI','source_sha256':source_hash,'canvas':{'width':80,'height':60},
        'generation':{'file':'raw.png','raw_sha256':sha256(raw.read_bytes()).hexdigest(),'request_settings':{'input_sha256':source_hash}},
        'metrics':{'transform':[[1,0,0],[0,1,0]]}}
    (tmp_path/'run.json').write_text(json.dumps(record))
    manifest=tmp_path/'input.json';manifest.write_text(json.dumps({'original':'original.png','original_sha256':source_hash,
        'crop':{'x':0,'y':0,'width':80,'height':60},'poses':{'Rest':{'source':'.'},'EI':{'source':'.'}}}))
    return manifest,record,tmp_path


def test_cached_character_checks_original_bytes_pose_identity_and_request_binding(cached_character):
    from app.oral_semantics import load_inputs
    manifest,record,folder=cached_character
    spec,base,images,points,provenance=load_inputs(manifest)
    assert images['EI'].size==(80,60) and points['EI'].shape==(478,2)
    assert provenance['EI']['original_sha256']==spec['original_sha256']
    record['generation']['request_settings']['input_sha256']='wrong-original'
    (folder/'run.json').write_text(json.dumps(record))
    with pytest.raises(PreparationError,match='immutable original'):load_inputs(manifest)
    record['generation']['request_settings']['input_sha256']=spec['original_sha256'];record['viseme']='CHSHR'
    (folder/'run.json').write_text(json.dumps(record))
    with pytest.raises(PreparationError,match='pose identity'):load_inputs(manifest)
    (folder/'original.png').write_bytes(b'changed')
    with pytest.raises(PreparationError,match='Original portrait bytes changed'):load_inputs(manifest)


def test_unfinalized_previews_can_never_be_installed_as_fallbacks(tmp_path):
    from app.prepare_semantic_oral import install
    output=tmp_path/'staged';output.mkdir()
    (output/'package.json').write_text(json.dumps({'endpoints_finalized':False}))
    with pytest.raises(PreparationError,match='Finalize browser endpoints'):install(output,tmp_path/'runtime')
    assert not (tmp_path/'runtime').exists()

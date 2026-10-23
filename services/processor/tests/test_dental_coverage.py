import json
from hashlib import sha256

import numpy as np
from PIL import Image
import pytest

from app.preparation.dental_coverage import continue_attachment
from app.preparation.faces import PreparationError


def test_extraction_preserves_subpixel_contours_and_opaque_dark_enamel():
    from app.preparation.dental_coverage import extract_fragment
    raw=np.full((10,10,3),[20,15,10],np.uint8)
    alpha=np.zeros((10,10),np.uint8);alpha[3:7,2:8]=255;alpha[2,2:8]=70
    result=np.asarray(extract_fragment(Image.fromarray(raw),Image.fromarray(alpha)))
    assert np.array_equal(result[...,3],alpha)
    assert np.array_equal(result[3:7,2:8,:3],raw[3:7,2:8])
    assert not result[alpha==0].any()


@pytest.mark.parametrize('row',['upper','lower'])
def test_attachment_retains_teeth_and_never_creates_a_second_incising_edge(row):
    a=np.zeros((50,60,4),np.uint8);a[20:24,10:50]=[34,22,15,255]
    a[21,20:30,:3]=[42,30,19]
    image,record=continue_attachment(Image.fromarray(a),row);b=np.asarray(image)
    protected=a[...,3]>=128
    assert np.array_equal(b[protected],a[protected])
    assert record['completed_pixels']>0 and record['selected_rgb_unchanged']
    for x in range(10,50):
        ys=np.flatnonzero(b[:,x,3]);assert np.all(np.diff(ys)==1)
        if row=='upper':assert ys.max()==23
        else:assert ys.min()==20
    assert np.all(b[...,3][b[...,3]>0]==255) # dark enamel is opaque


def test_completion_rejects_missing_enamel():
    with pytest.raises(PreparationError):continue_attachment(Image.new('RGBA',(20,20)),'lower')


def test_shared_lighting_preserves_dark_enamel_opacity_and_texture():
    from app.preparation.dental_coverage import neutralize_shared_lighting
    upper=np.zeros((20,30,4),np.uint8);lower=upper.copy()
    upper[5:10,5:25]=[45,35,25,255];lower[10:15,5:25]=[110,95,80,255]
    upper[7,10,:3]=[30,20,15]
    images,record=neutralize_shared_lighting(Image.fromarray(upper),Image.fromarray(lower))
    a,b=map(np.asarray,images)
    assert np.array_equal(a[...,3],upper[...,3]) and np.array_equal(b[...,3],lower[...,3])
    assert a[7,10,0]<a[7,11,0] and a[7,11,0]>upper[7,11,0]
    assert a[7,10,3]==255 and record['alpha_unchanged']


def test_repair_cache_is_bound_and_provider_failure_stops_batch(tmp_path):
    from app.oral_repair import request
    stage=tmp_path/'lower';stage.mkdir();raw=b'original';(stage/'raw.jpg').write_bytes(raw)
    (stage/'run.json').write_text(json.dumps({'status':'received','file':'raw.jpg','raw_sha256':sha256(raw).hexdigest()}))
    assert request(tmp_path,'lower')['status']=='received'
    (stage/'raw.jpg').write_bytes(b'changed')
    with pytest.raises(PreparationError,match='changed'):request(tmp_path,'lower')
    (stage/'run.json').write_text('{"status":"failed"}')
    with pytest.raises(PreparationError,match='stopped'):request(tmp_path,'fv')
    with pytest.raises(PreparationError,match='retry'):request(tmp_path,'lower')


def test_generated_mouth_drift_is_registered_without_changing_its_opening():
    from app.preparation.semantic_rig import register_oral_patch
    a=np.zeros((60,70),np.uint8);a[25:32,15:55]=255
    anchor=Image.fromarray(a)
    observed=np.roll(a,3,axis=0);opening=np.zeros_like(a);opening[29:42,10:60]=255
    masks={'upper_teeth':Image.fromarray(observed),'aperture':Image.fromarray(opening)}
    image=Image.fromarray(np.repeat(observed[...,None],3,axis=2))
    _,shifted,pts,record=register_oral_patch(image,masks,np.zeros((478,2)),anchor,0)
    assert record['translation']==[0,-3] and np.all(pts[:,1]==-3)
    assert np.array_equal(np.roll(np.asarray(shifted['aperture']),3,axis=0),opening)
    assert np.array_equal(np.asarray(shifted['upper_teeth']),a)
    masks['upper_teeth']=Image.fromarray(np.roll(a,6,axis=0))
    with pytest.raises(PreparationError,match='drift'):
        register_oral_patch(image,masks,np.zeros((478,2)),anchor,0)

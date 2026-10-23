from hashlib import sha256
import json
from pathlib import Path
import socket

import numpy as np
from PIL import Image,ImageDraw
import pytest

from app.preparation.dental_selection import detect_dental_rows,load_originals,select_reference
from app.preparation.faces import PreparationError
from app.preparation.mesh import INNER
from app.preparation.oral import cavity_mask,add_dentition,reviewed_dental_masks,dental_bite_stop

ROOT=Path(__file__).resolve().parents[3]
SETTINGS=json.loads((ROOT/'docs/characters/syntelle/dental-selection.json').read_text())


def mouth(tooth_width=140,opening=70,touching=False,teeth=True,tongue=False):
    image=Image.new('RGB',(240,180),(180,70,65));draw=ImageDraw.Draw(image)
    draw.ellipse((20,90-opening/2,220,90+opening/2),fill=(24,5,4))
    points=np.zeros((478,3))
    for i,theta in zip(INNER,np.linspace(np.pi,3*np.pi,20,endpoint=False)):
        points[i,:2]=[120+100*np.cos(theta),90+opening/2*np.sin(theta)]
    points[61,:2]=[20,90];points[291,:2]=[220,90]
    if teeth:
        x0=120-tooth_width/2;x1=120+tooth_width/2
        draw.rectangle((x0,90-opening*.36,x1,90-opening*.18),fill=(175,150,135))
        draw.rectangle((x0+5,90+opening*.22,x1-5,90+opening*.34),fill=(170,145,132))
        if touching:draw.rectangle((x0,90-opening*.36,x1,90+opening*.34),fill=(175,150,135))
    if tongue:draw.rectangle((105,86,135,98),fill=(190,65,80))
    return image,points


def test_selects_useful_dental_visibility_rather_than_largest_aperture_or_pose_id():
    wide,p=mouth(150,60); open_few,q=mouth(80,80)
    result,_=select_reference({'not-EI':wide,'EI':open_few},{'not-EI':p,'EI':q},SETTINGS)
    assert result['selected_pose']=='not-EI'
    assert result['candidates']['not-EI']['score']>result['candidates']['EI']['score']
    assert result['new_provider_requests']==0


def test_tongue_and_external_highlights_stay_outside_the_mask_proposal():
    image,points=mouth(tongue=True);draw=ImageDraw.Draw(image)
    draw.rectangle((0,0,239,20),fill='white')
    upper,lower,metrics=detect_dental_rows(image,points,SETTINGS)
    assert metrics['eligible']
    combined=np.maximum(np.asarray(upper),np.asarray(lower))
    assert not combined[86:99,105:136].any()
    assert not combined[:21].any()
    assert not combined[np.asarray(cavity_mask(image.size,points))==0].any()
    assert not (np.asarray(upper).astype(bool)&np.asarray(lower).astype(bool)).any()


def test_touching_rows_are_rejected_even_when_they_are_bright_and_wide():
    image,points=mouth(170,touching=True)
    _,_,metrics=detect_dental_rows(image,points,SETTINGS)
    assert not metrics['eligible']
    assert any('separated' in reason or 'gap' in reason for reason in metrics['reasons'])


def test_no_candidate_or_a_tie_requires_review_without_a_fallback():
    closed,p=mouth(teeth=False)
    result,_=select_reference({'closed':closed},{'closed':p},SETTINGS)
    assert result['selected_pose'] is None and result['status']=='needs_review'
    good,p=mouth()
    result,_=select_reference({'one':good,'two':good},{'one':p,'two':p},SETTINGS)
    assert result['selected_pose'] is None and result['score_margin']==0


def test_masking_and_ranking_are_reproducible_with_reordered_candidates():
    a,p=mouth(150);b,q=mouth(100)
    first,m1=select_reference({'a':a,'b':b},{'a':p,'b':q},SETTINGS)
    second,m2=select_reference({'b':b,'a':a},{'b':q,'a':p},SETTINGS)
    assert first==second
    for pose in m1:
        assert all(x.tobytes()==y.tobytes() for x,y in zip(m1[pose],m2[pose]))


def test_current_selection_replays_the_real_originals_with_network_disabled(monkeypatch):
    reference=ROOT/'.tools/viseme-a-05';batch=ROOT/'.tools/visemes-01'
    if not (batch/'batch.json').exists():pytest.skip('Original paid evidence is local.')
    package=json.loads((ROOT/'apps/web/src/experiments/viseme-a/poses/package.json').read_text())
    crop=package['crop'];bounds=(crop['x'],crop['y'],crop['x']+crop['width'],crop['y']+crop['height'])
    size=(package['canvas']['width'],package['canvas']['height'])
    def unexpected(*_,**__):raise AssertionError('Dental selection must never contact a provider.')
    monkeypatch.setattr(socket.socket,'connect',unexpected)
    images,points,inputs=load_originals(reference,batch,size,bounds,package['poses'])
    selected,masks=select_reference(images,points,SETTINGS)
    assert selected['selected_pose']=='EI'
    assert selected['ranking']==['EI','A']
    assert not selected['candidates']['FV']['eligible']
    assert not selected['candidates']['SZ']['eligible']
    assert package['reference']['selection']['selected_pose']==selected['selected_pose']
    assert package['reference']['provenance']['raw_sha256']==inputs['EI']['raw_sha256']
    assets=ROOT/'apps/web/src/experiments/viseme-a/poses'
    raw=np.asarray(images['EI'])
    for row in ['upper','lower']:
        canonical=np.asarray(Image.open(assets/f'{row}-teeth.webp').convert('RGBA'))
        opaque=canonical[...,3]==255
        assert opaque.sum()>100
        np.testing.assert_array_equal(canonical[opaque,:3],raw[opaque])
    assert package['poses']['EI']['lower_translation']==[0,0]


def test_changed_or_missing_original_cache_never_invokes_generation(tmp_path):
    folder=tmp_path/'reference';folder.mkdir()
    record={'generation':{'file':'generated-a.jpg','raw_sha256':sha256(b'original').hexdigest()}}
    (folder/'run.json').write_text(json.dumps(record))
    with pytest.raises(PreparationError,match='missing'):
        load_originals(folder,tmp_path,(100,100),(0,0,50,50),['A'])
    (folder/'generated-a.jpg').write_bytes(b'changed')
    with pytest.raises(PreparationError,match='changed'):
        load_originals(folder,tmp_path,(100,100),(0,0,50,50),['A'])


def test_lower_jaw_offsets_never_cross_the_fixed_upper_dentition():
    assets=ROOT/'apps/web/src/experiments/viseme-a/poses'
    package=json.loads((assets/'package.json').read_text())
    jaw=package['reference']['selection']['jaw_motion']
    upper=np.asarray(Image.open(assets/'upper-teeth.webp').convert('RGBA'))
    lower=np.asarray(Image.open(assets/'lower-teeth.webp').convert('RGBA'))
    _,profile=dental_bite_stop(Image.fromarray(upper[...,3]),Image.fromarray(lower[...,3]),jaw['minimum_gap_px'])
    minimum_reference_gap=min(profile['reference_gaps_px'])
    assert jaw['poses']['Rest']['bite_stop_applied']
    for key,pair in package['pairs'].items():
        if not pair['safe']:continue
        source,target=key.split('-')
        a=package['poses'][source]['lower_translation'][1]
        b=package['poses'][target]['lower_translation'][1]
        for t in np.linspace(0,1,21):
            offset=(1-t)*a+t*b
            assert minimum_reference_gap+offset >= jaw['minimum_gap_px']-0.001


def test_no_eligible_reference_keeps_review_evidence_and_does_not_install_a_fallback(tmp_path,monkeypatch):
    from app.prepare_viseme_package import prepare
    reference=ROOT/'.tools/viseme-a-05';batch=ROOT/'.tools/visemes-01'
    if not (batch/'batch.json').exists():pytest.skip('Original paid evidence is local.')
    def unexpected(*_,**__):raise AssertionError('No network call is allowed during failed selection.')
    monkeypatch.setattr(socket.socket,'connect',unexpected)
    settings=dict(SETTINGS);settings['minimum_row_span_ratio']=2
    settings_path=tmp_path/'settings.json';settings_path.write_text(json.dumps(settings))
    review=json.loads((ROOT/'docs/characters/syntelle/viseme-auto-review.json').read_text())
    review['reference']['automatic_original']=str(settings_path)
    review_path=tmp_path/'review.json';review_path.write_text(json.dumps(review))
    runtime=ROOT/'apps/web/src/experiments/viseme-a/poses/package.json';before=runtime.read_bytes()
    with pytest.raises(PreparationError,match='No unambiguous dental reference'):
        prepare(reference,batch,review_path,tmp_path/'output')
    evidence=json.loads((tmp_path/'output/selection/selection.json').read_text())
    assert evidence['status']=='needs_review' and evidence['selected_pose'] is None
    assert (tmp_path/'output/selection/ei/upper-proposal.png').is_file()
    assert not (tmp_path/'output/package.json').exists()
    assert runtime.read_bytes()==before


def test_dental_inset_leaves_the_lip_boundary_untouched():
    _,points=mouth()
    texture=Image.new('RGB',(240,180),(25,5,4))
    white=Image.new('RGBA',texture.size,(255,255,255,255))
    result=np.asarray(add_dentition(texture,white,white,points,{'upper_visible':True,'lower_visible':False,'lower_translation':[0,0],'dental_inset_px':1}))
    raw_mask=np.asarray(cavity_mask(texture.size,points))>0
    inset=np.asarray(cavity_mask(texture.size,points,1))
    boundary=raw_mask & (inset==0)
    assert boundary.any()
    np.testing.assert_array_equal(result[boundary],np.asarray(texture)[boundary])
    assert np.all(result[90,120]==255)


def test_rounded_toothless_endpoints_keep_the_original_cavity_instead_of_inserting_a_dental_strip():
    assets=ROOT/'apps/web/src/experiments/viseme-a/poses'
    package=json.loads((assets/'package.json').read_text())
    for pose in ['O','U']:
        entry=package['poses'][pose]
        assert not entry['upper_visible'] and not entry['lower_visible']
        endpoint=np.asarray(Image.open(assets/entry['endpoint']).convert('RGBA'))
        original=np.asarray(Image.open(assets/entry['surface']).convert('RGB'))
        np.testing.assert_array_equal(endpoint[...,:3],original)


def test_reviewed_masks_cannot_be_reused_for_another_original_or_crop():
    record=json.loads((ROOT/'docs/characters/syntelle/dental-masks-reviewed.json').read_text(encoding='utf-8'))
    args=(record['source_pose'],record['source_sha256'],record['raw_sha256'],record['coordinates']['crop'])
    upper,lower=reviewed_dental_masks(record,*args)
    assert np.count_nonzero(np.asarray(upper)>=128)>500
    assert np.count_nonzero(np.asarray(lower)>=128)>500
    for index,bad in [(0,'A'),(1,'wrong-base'),(2,'wrong-raw'),(3,dict(args[3],x=0))]:
        altered=list(args);altered[index]=bad
        with pytest.raises(PreparationError,match='do not match'):
            reviewed_dental_masks(record,*altered)
    rejected=dict(record,status='rejected')
    with pytest.raises(PreparationError,match='not an available'):
        reviewed_dental_masks(rejected,*args)


def test_bite_stop_compares_opposing_columns_instead_of_unrelated_side_and_front_teeth():
    upper=Image.new('L',(30,30));lower=Image.new('L',(30,30))
    a,b=ImageDraw.Draw(upper),ImageDraw.Draw(lower)
    a.rectangle((4,3,10,7),fill=255);a.rectangle((11,7,25,12),fill=255)
    b.rectangle((4,13,10,17),fill=255);b.rectangle((11,18,25,22),fill=255)
    # Both opposing regions have six pixels of clearance despite their curved rows.
    stop,evidence=dental_bite_stop(upper,lower,1)
    assert stop==-5 and set(evidence['reference_gaps_px'])=={6}
    empty=Image.new('L',upper.size)
    with pytest.raises(PreparationError,match='no shared columns'):
        dental_bite_stop(upper,empty,1)
    with pytest.raises(PreparationError,match='overlap or cross'):
        dental_bite_stop(upper,upper,1)

"""Automatic reference ownership, opaque material and generic shader coverage."""
import json

import numpy as np
from PIL import Image, ImageDraw
import pytest

from app.core.paths import ROOT
from app.preparation.faces import PreparationError
from app.preparation.mesh import INNER
from app.preparation.reference_dentition import extract_reference_rows
from app.workflows.oral.prepare_reference_dentition import transition_opening


def fixture(lower=True):
    h,w=120,200;yy,xx=np.indices((h,w))
    points=np.zeros((478,2),float)
    angles=np.linspace(np.pi,3*np.pi,20,endpoint=False)
    points[INNER]=np.column_stack([100+70*np.cos(angles),60+22*np.sin(angles)])
    points[61]=[30,60];points[291]=[170,60]
    opening=Image.new('L',(w,h))
    ImageDraw.Draw(opening).polygon([tuple(v) for v in points[INNER]],fill=255)
    inside=np.asarray(opening)>0
    raw=np.zeros((h,w,3),np.uint8);raw[:]=[170,85,88];raw[inside]=[54,15,10]
    slit=63-8*((xx-100)/70)**2
    upper=inside&(yy<slit-2)&(yy>43)&(xx>42)&(xx<158)
    bottom=inside&(yy>slit+2)&(yy<76)&(xx>58)&(xx<150)
    raw[upper]=[218,207,187]
    if lower:raw[bottom]=[116,89,74]
    return Image.fromarray(raw),points,inside,upper,bottom


def test_curved_gap_separates_shadowed_lower_without_recolouring_or_lip_pixels():
    image,points,aperture,upper,lower=fixture()
    masks,fragments,report=extract_reference_rows(image,points)
    assert report['ready'],report['reasons']
    assert report['seam']['median_local_separation']>=.55
    assert np.ptp(np.asarray(report['seam']['path'])[:,1])>=4
    for row,expected in [('upper',upper),('lower',lower)]:
        mask=np.asarray(masks[row])>0
        fragment=np.asarray(fragments[row]);raw=np.asarray(image)
        assert mask.any()
        assert not (mask&~expected).any()
        assert np.array_equal(fragment[mask,:3],raw[mask])
        assert (fragment[mask,3]==255).all()
        assert not fragment[~mask].any()
    assert not ((np.asarray(masks['upper'])>0)&(np.asarray(masks['lower'])>0)).any()


def test_missing_lower_crowns_cannot_pass_by_splitting_the_upper_row():
    image,points,*_=fixture(lower=False)
    _,_,report=extract_reference_rows(image,points)
    assert not report['ready']
    assert any('lower' in item or 'gap' in item for item in report['reasons'])


def test_ambiguous_reference_and_nonfinite_landmarks_fail_without_fallback():
    _,points,*_=fixture()
    with pytest.raises(PreparationError,match='enamel'):
        extract_reference_rows(Image.new('RGB',(200,120),'black'),points)
    points[0,0]=np.nan
    with pytest.raises(PreparationError,match='Finite'):
        extract_reference_rows(Image.new('RGB',(200,120),'white'),points)


def test_generic_barycentric_sdf_interpolation_matches_identity_geometry():
    mesh={'width':8,'height':6,'rest':[[0,0],[8,0],[8,6],[0,6]],
          'open':[[0,0],[8,0],[8,6],[0,6]],'triangles':[[0,1,2],[0,2,3]]}
    a=np.tile(np.arange(8)*20,(6,1)).astype(np.uint8)
    b=a[:,::-1].copy()
    for t in (0.,.25,.5,.75,1.):
        assert np.array_equal(transition_opening(mesh,a,b,t),a*(1-t)+b*t>=128)


@pytest.mark.parametrize(('case','eligible'),[('cafe',True),('woodland',False)])
def test_cached_other_characters_preserve_rgb_and_reject_an_unobserved_gap(case,eligible):
    directory=ROOT/'.tools/corpus-08'/case/'poses/ei'
    if not (directory/'run.json').exists():pytest.skip('Cached corpus fixture unavailable')
    record=json.loads((directory/'run.json').read_bytes())
    image=Image.open(directory/record['generation']['file']).convert('RGB').resize(
        (record['canvas']['width'],record['canvas']['height']),Image.Resampling.LANCZOS)
    points=np.asarray(json.loads((directory/'variant-landmarks.json').read_bytes())['landmarks_px'])
    masks,fragments,report=extract_reference_rows(image,points)
    assert report['ready'] is eligible
    assert not report['manual_masks'] and not report['paid_calls']
    for row in ('upper','lower'):
        mask=np.asarray(masks[row])>0
        assert np.array_equal(np.asarray(fragments[row])[mask,:3],np.asarray(image)[mask])
    if not eligible:
        assert any('gap' in item for item in report['reasons'])


def test_cached_prepared_arch_has_one_source_identity_and_explicit_transition_evidence():
    directory=ROOT/'.tools/reference-laughter-76/dental-v7'
    if not (directory/'dental-material.json').exists():pytest.skip('Local reference trial unavailable')
    spec=json.loads((directory/'dental-material.json').read_bytes())
    oral=json.loads((directory/'oral/package.json').read_bytes())
    assert spec['source_sha256']==oral['source_sha256']
    assert spec['ready']==(spec['extraction_ready'] and spec['speech_validation']['ready'])
    assert sum(a!=b for a,b in (name.split('-') for name in oral['pairs']))==72
    assert oral['reference_dentition']['shared_lower_calibration']['per_pose_overrides'] is False
    transitions=spec['speech_validation']['transitions']
    assert transitions['directed_pairs']==72
    assert transitions['samples_per_pair']==9
    # Failed intermediates must remain failed even when all endpoints pass.
    assert not spec['ready'] or transitions['valid']

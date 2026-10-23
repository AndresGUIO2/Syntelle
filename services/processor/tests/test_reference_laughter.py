"""Source-bound expression geometry, frozen connectivity and manual invariants."""
import json
from hashlib import sha256

import numpy as np
import pytest
from PIL import Image

from app.core.paths import ROOT
from app.preparation.eyes import align_eyes
from app.preparation.reference_laughter import eye_profiles, facial_mesh, mesh_maps


@pytest.mark.parametrize('character', ['syntelle','cafe','woodland'])
def test_common_face_geometry_accepts_cached_non_syntelle_inputs(character):
    folder=ROOT/'.tools/rgbd-motion-18/prepared-current'/character/'inputs'
    path=folder/'landmarks.json'
    if not path.exists():
        pytest.skip('Cached character anatomy unavailable')
    points=np.asarray(json.loads(path.read_bytes())['landmarks_px'])[:,:2]
    motion=json.loads((ROOT/'apps/web/src/experiments/motion/rgbd'/character/'package.json').read_bytes())
    width=float(np.linalg.norm(points[61]-points[291]))
    target=points.copy()
    target[[61,291],1]-=width*.015
    source,end,triangles,mask,report=facial_mesh(points,target,(motion['canvas']['width'],motion['canvas']['height']))
    assert report['valid'] and report['amplitude']==1
    assert np.array_equal(end[[report['indices'].index(i) for i in (61,291)]],target[[61,291]])
    assert not mask[0].any() and not mask[-1].any()
    assert len(triangles)>0 and source.shape==end.shape


def test_mesh_maps_preserves_frozen_connectivity_and_identity():
    vertices=np.array([[0.,0.],[5.,0.],[0.,5.],[5.,5.]])
    triangles=np.array([[0,1,2],[1,3,2]])
    yy,xx=np.indices((6,6),np.float32);xy=np.stack([xx,yy],-1)
    a,b=mesh_maps(vertices,vertices,vertices+[2.,3.],triangles,xy)
    np.testing.assert_allclose(a,xy,atol=1e-6)
    np.testing.assert_allclose(b,xy+[2.,3.],atol=1e-6)


def test_actual_reference_preserves_full_amplitude_and_eye_coverage():
    manifest=ROOT/'docs/characters/reference-laughter-inputs.json'
    config=json.loads(manifest.read_bytes());resolve=lambda key:ROOT/config[key]
    if not resolve('original').exists():
        pytest.skip('Owner source unavailable')
    p=np.asarray(json.loads(resolve('landmarks').read_bytes())['landmarks_px'])[:,:2]
    generated=ROOT/'.tools/expressive-performance-67/joint-candidate-v12/laugh/landmarks.json'
    if not generated.exists():
        pytest.skip('Cached laughter reference unavailable')
    q=np.asarray(json.loads(generated.read_bytes())['landmarks_px'])[:,:2]
    a,b=Image.open(resolve('original')),Image.open(resolve('laugh'))
    _,target,_=align_eyes(a,b,p,q,a.width/b.width)
    start,end,_,_,validation=facial_mesh(p,target,a.size)
    assert validation['minimum_triangle_ratio']>0
    assert validation['amplitude']==1
    chin=validation['indices'].index(152)
    np.testing.assert_allclose(end[chin]-start[chin],target[152]-p[152])
    assert validation['chin_displacement_px'][1]>20
    assert validation['lower_lip_displacement_px'][1]>20
    profiles=eye_profiles(p,target,json.loads(resolve('motion').read_bytes()))
    assert len(profiles['profiles'])==25
    assert profiles['profiles'][0]!=profiles['profiles'][-1]


def test_prepared_reference_keeps_exact_original_manual_smile():
    prepared=ROOT/'.tools/reference-laughter-76/expressions/expressions.json'
    if not prepared.exists():
        pytest.skip('New local reference package unavailable')
    candidate=json.loads(prepared.read_bytes())
    reference=candidate['reference_laughter']
    assert reference['version']=='reference-laughter/1'
    assert reference['amplitude_reduction']==1
    assert not reference['reference_teeth_baked_into_skin']
    assert len(reference['frames'])==25
    before=ROOT/'.tools/expression-controls-75/runtime-expressions'
    for name in ('field.png','smile-material.png'):
        assert sha256((before/name).read_bytes()).digest()==sha256((prepared.parent/name).read_bytes()).digest()


def test_reference_contact_tissue_is_not_darkened_and_arch_does_not_slide_up():
    folder=ROOT/'.tools/reference-laughter-76/expressions-jaw-v2'
    if not folder.exists():
        pytest.skip('Corrected reference package unavailable')
    package=json.loads((folder/'expressions.json').read_bytes())['reference_laughter']
    assert package['validation']['chin_displacement_px'][1]>20
    assert all(v==[0.,0.] for v in package['oral']['lower_translation'])
    dental=ROOT/'.tools/reference-laughter-76/dental-v7'
    oral=json.loads((dental/'oral/package.json').read_bytes())
    crop=oral['crop'];x,y,w,h=(crop[k] for k in ('x','y','width','height'))
    reference=np.asarray(Image.open(folder/'registered-reference.png'))[y:y+h,x:x+w,:3]
    cavity=np.asarray(Image.open(folder/package['oral']['cavity']['file']))[:,:,:3]
    masks=np.maximum(np.asarray(Image.open(dental/'upper-mask.png')),np.asarray(Image.open(dental/'lower-mask.png')))
    assert np.array_equal(cavity[masks==0],reference[masks==0])

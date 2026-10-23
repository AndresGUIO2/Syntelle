import numpy as np
import pytest

from app.preparation.faces import PreparationError
from app.preparation.projection_rig import robust_affine, limit_slopes, project, unproject, validate


def camera():
    return {'size':[800,600],'center':[400,300],'focal':1200.,'distance':1200.,'near':400.,'far':2000.,'pivot_px':[400,400]}


def test_robust_calibration_recovers_scale_and_offset_with_outliers():
    x=np.linspace(.5,4,200);y=x*37-54;y[::19]+=300
    slope,offset,report=robust_affine(x,y)
    assert abs(slope-37)<.01 and abs(offset+54)<.01
    assert report['downweighted_anchors']>=10
    with pytest.raises(PreparationError):robust_affine(x,-x)
    with pytest.raises(PreparationError):robust_affine(np.ones(60),np.ones(60))


def test_neutral_unprojection_is_exact_and_reference_texture_is_projective():
    v=np.array([[.1,.2,-40,1,0],[.8,.3,100,1,0],[.4,.9,70,0,1]])
    c=camera();world=unproject(v,c)
    expected=(v[:,:2]*c['size']-c['center'])*(c['distance']-v[:,2,None])/c['focal']
    np.testing.assert_allclose(world[:,:2],expected,atol=1e-12)
    xy,q=project(v,c,[0,100,0],[0,0,0])
    np.testing.assert_allclose(xy,v[:,:2]*c['size'],atol=1e-12)
    # Homogeneous source coordinates cancel the reference camera's varying W.
    barycentric=np.array([.2,.3,.5]);uv=v[:,:2]
    source=np.column_stack((uv*q[:,None],q))
    interpolated=(barycentric[:,None]*(source/q[:,None])).sum(0)
    np.testing.assert_allclose(interpolated[:2]/interpolated[2],barycentric@uv,atol=1e-12)
    conventional=(barycentric[:,None]*(uv/q[:,None])).sum(0)/(barycentric/q).sum()
    assert np.max(abs(conventional-barycentric@uv))>.001


def test_rotation_separates_depths_and_keeps_torso_fixed():
    v=np.array([[.5,.3,90,1,0],[.5,.3,-90,1,0],[.5,.85,0,0,1]])
    c=camera();original,_=project(v,c,[0,100,0],[0,0,0])
    turned,q=project(v,c,[0,100,0],[3,0,0])
    assert turned[0,0]>original[0,0]>turned[1,0]
    assert q[0]<q[1]
    np.testing.assert_array_equal(turned[2],original[2])
    breathed,_=project(v,c,[0,100,0],[0,0,0],3)
    np.testing.assert_array_equal(breathed[:2],original[:2])
    assert breathed[2,1]==original[2,1]+3


def test_slope_projection_preserves_valid_detail_and_regularizes_cliffs():
    yy,xx=np.mgrid[0:9,0:9];plane=xx*2.+yy
    np.testing.assert_array_equal(limit_slopes(plane.ravel(),[80,80],8,6),plane.ravel())
    cliff=plane.copy();cliff[:,4:]+=500
    corrected=limit_slopes(cliff.ravel(),[80,80],8,6).reshape(9,9)
    assert np.max(abs(np.diff(corrected,axis=1)))<60.1
    assert corrected.max()-corrected.min()>100


def test_validation_rejects_inversion_and_clipping():
    v=np.column_stack(([[0,0],[1,0],[0,1],[1,1]],np.zeros(4),np.ones(4),np.zeros(4)))
    t=np.array([[0,1,2],[2,1,3]])
    assert validate(v,t,camera(),[0,100,0],[3,1,.3],300)['valid']
    v[3,:2]=[-1,-1]
    assert not validate(v,t,camera(),[0,100,0],[80,80,0],300)['valid']
    c=camera();c['near']=1300
    assert not validate(v,t,c,[0,100,0],[0,0,0],300)['valid']


def test_failed_or_different_cache_is_never_overwritten(tmp_path):
    import json
    from app.workflows.motion.prepare_projection_motion import verified_cache
    assert not verified_cache(tmp_path,'one')
    (tmp_path/'input-record.json').write_text(json.dumps({'binding':'one'}))
    with pytest.raises(PreparationError,match='previously failed'):verified_cache(tmp_path,'one')
    with pytest.raises(PreparationError,match='select a new output'):verified_cache(tmp_path,'two')


def test_cache_verifies_package_and_geometry_bytes(tmp_path):
    import json
    from hashlib import sha256
    from app.preparation.archive import encoded
    from app.workflows.motion.prepare_projection_motion import verified_cache
    package={'projection':{'geometry':{'sha256':sha256(b'geometry').hexdigest()}}}
    package['build_id']=sha256(encoded(package)).hexdigest()
    (tmp_path/'input-record.json').write_text(json.dumps({'binding':'one'}))
    (tmp_path/'record.json').write_text(json.dumps({'binding':'one'}))
    (tmp_path/'package.json').write_text(json.dumps(package))
    (tmp_path/'projection.bin').write_bytes(b'geometry')
    assert verified_cache(tmp_path,'one')
    (tmp_path/'projection.bin').write_bytes(b'corrupted')
    with pytest.raises(PreparationError,match='outputs have changed'):verified_cache(tmp_path,'one')

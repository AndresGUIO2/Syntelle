import numpy as np

from app.preparation.head_rig import deform, validate


def test_neutral_rig_is_exact_and_breathing_cannot_pulse_face():
    vertices=np.array([[.5,.2,20,1,0],[.6,.4,-5,1,0],[.5,.8,0,.1,1]])
    pivot=np.array([50,50]);size=np.array([100,100])
    np.testing.assert_array_equal(deform(vertices,pivot,size,[0,0,0],0),vertices[:,:2]*size)
    breathed=deform(vertices,pivot,size,[0,0,0],3)
    np.testing.assert_array_equal(breathed[:2],vertices[:2,:2]*size)
    assert breathed[2,1]>80


def test_validation_checks_the_envelope_and_rejects_folded_geometry():
    uv=np.array([[0,0],[1,0],[0,1],[1,1]])
    vertices=np.column_stack((uv,np.zeros(4),np.ones(4),np.zeros(4)))
    result=validate(vertices,[50,50],[100,100],1,[3,2,1.2],50)
    assert result['valid'] and result['configurations']==375
    vertices[3,:2]=[-1,-1]
    assert not validate(vertices,[50,50],[100,100],1,[3,2,1.2],50)['valid']


def test_yaw_rotates_opposing_depths_without_translating_the_body():
    vertices=np.array([[.5,.2,20,1,0],[.5,.2,-20,1,0],[.5,.8,0,0,0]])
    pivot=np.array([50,50]);size=np.array([100,100])
    neutral=deform(vertices,pivot,size,[0,0,0],0)
    shifted=deform(vertices,pivot,size,[8,0,0],0)
    delta=shifted-neutral
    assert delta[0,0]>0>delta[1,0]
    assert abs(delta[0,0]+delta[1,0])<1e-10
    np.testing.assert_array_equal(delta[:,1],np.zeros(3))
    np.testing.assert_array_equal(shifted[2],neutral[2])
    vertices=np.column_stack(([[0,0],[1,0],[0,1],[1,1]],np.zeros(4),np.ones(4),np.zeros(4)))
    report=validate(vertices,pivot,size,1,[8,2,1.2],50)
    assert report['valid'] and report['configurations']==375

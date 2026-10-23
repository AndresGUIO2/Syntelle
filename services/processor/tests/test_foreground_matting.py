"""Known-alpha fixtures, independent of the generated character masks."""
import cv2
import numpy as np
import pytest

from app.preparation.foreground import semantic_trimap, solve_matte, cache_settings
from app.preparation.faces import PreparationError


def fixture():
    alpha=np.zeros((100,140),np.float64)
    alpha[15:95,45:105]=1
    # Antialiased, translucent isolated strands beside an opaque dark subject.
    for x,a in [(40,.25),(41,.7),(108,.45),(109,.15)]: alpha[20:85,x]=a
    alpha=cv2.GaussianBlur(alpha,(3,3),.45)
    foreground=np.full((100,140,3),[.12,.07,.045])
    background=np.full((100,140,3),[.91,.88,.81])
    rgb=np.rint((alpha[...,None]*foreground+(1-alpha[...,None])*background)*255).astype(np.uint8)
    trimap=np.full(alpha.shape,.5);trimap[:,:30]=0;trimap[:,120:]=0
    trimap[:8]=0;trimap[97:]=0;trimap[22:85,54:96]=1
    return rgb,alpha,foreground,trimap


def test_closed_form_recovers_transparent_strands_and_removes_pale_fringe():
    rgb,truth,foreground,trimap=fixture()
    alpha,color=solve_matte(rgb,trimap)
    uncertain=(truth>.01)&(truth<.99)
    assert np.mean(np.abs(alpha-truth)[uncertain])<.035
    assert np.mean(alpha[25:80,40:42])>.3  # Binary erosion would lose these.
    exposed=(truth>.15)&(truth<.85)
    assert np.mean(np.abs(color/255-foreground)[exposed])<.055
    np.testing.assert_array_equal(color[alpha==1],rgb[alpha==1])
    # Both light and dark destinations must use decontaminated edge RGB.
    for bg in [0.,1.]:
        actual=alpha[...,None]*(color/255)+(1-alpha[...,None])*bg
        expected=truth[...,None]*foreground+(1-truth[...,None])*bg
        assert np.mean(np.abs(actual-expected)[uncertain])<.045


def test_trimap_leaves_low_confidence_hair_near_subject_uncertain():
    p=np.zeros((100,140));p[15:95,45:105]=1;p[20:85,43]=.03
    trimap=semantic_trimap(p,[70,45])
    assert trimap[45,43]==.5 and trimap[45,70]==1 and trimap[0,0]==0


def test_subject_selection_excludes_disconnected_furniture_without_eroding_rim():
    p=np.zeros((100,140));p[20:90,40:90]=1;p[30:70,110:130]=1
    trimap=semantic_trimap(p,[65,40])
    assert trimap[40,120]==0 and trimap[40,39]==.5


def test_invalid_seeds_fail_explicitly_and_settings_bind_algorithm():
    with pytest.raises(PreparationError,match='insufficient'):
        semantic_trimap(np.full((50,50),.5))
    assert len(cache_settings()['implementation_sha256'])==64


def test_enclosed_shadowed_hair_is_opaque_instead_of_a_background_hole():
    p=np.zeros((120,140));p[10:115,25:115]=.99
    p[55:95,50:75]=.4
    trimap=semantic_trimap(p,[65,35])
    assert trimap[70,60]==1
    assert trimap[55,24]==.5  # The real outer boundary is still uncertain.
    p[60:90,55:70]=0
    assert semantic_trimap(p,[65,35])[70,60]==0  # Confident true holes survive.


def test_coarse_hair_certainty_cannot_lock_real_background_gaps_opaque():
    probability=np.zeros((200,200));probability[15:195,35:165]=.99
    hair=np.zeros((200,200),bool);hair[20:110,35:165]=True
    original=semantic_trimap(probability,[100,65])
    refined=semantic_trimap(probability,[100,65],hair)
    assert original[60,38]==1 and refined[60,38]==.5
    assert refined[60,100]==1
    assert refined[140,38]==original[140,38]  # White clothing is not hair.
    np.testing.assert_array_equal(refined[original==0],original[original==0])


def test_native_tile_origins_cover_edges_and_large_portraits_without_resizing():
    from app.preparation.hair_matting import origins
    for length in [100,512,572,1024,1792,2400]:
        coverage=np.zeros(length,bool)
        for origin in origins(length): coverage[origin:origin+512]=True
        assert coverage.all()

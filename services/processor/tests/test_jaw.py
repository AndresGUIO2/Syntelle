import numpy as np
from PIL import Image,ImageDraw
import pytest

from app.preparation.jaw import calibrate_jaw
from app.preparation.faces import PreparationError


def inputs():
    upper=Image.new('L',(100,100));lower=upper.copy()
    ImageDraw.Draw(upper).rectangle((20,20,80,30),fill=255)
    ImageDraw.Draw(lower).rectangle((20,45,80,55),fill=255)
    points={name:np.zeros((478,2)) for name in ('EI','shifted','narrow','open')}
    for name,top,bottom in [('EI',10,65),('shifted',30,85),('narrow',12,30),('open',8,68)]:
        points[name][13,1]=top;points[name][14,1]=bottom
    return points,upper,lower,{'method':'lip-aperture-delta','minimum_clearance_mouth_width_ratio':0.025}


def test_aperture_calibration_ignores_whole_mouth_translation_and_preserves_reference():
    points,upper,lower,settings=inputs()
    positions,evidence=calibrate_jaw(points,'EI',upper,lower,100,settings)
    assert positions['EI']==positions['shifted']==[0,0]
    assert positions['open']==[0,5]
    assert evidence['minimum_gap_px']==2.5
    assert positions['narrow']==[0,-12]  # 15px row separation retains >= 2.5px clearance.
    for a in positions.values():
        for b in positions.values():
            for t in np.linspace(0,1,21):assert 15+a[1]*(1-t)+b[1]*t>=2.5


def test_incompatible_clearance_or_nonfinite_settings_fail_instead_of_changing_reference():
    points,upper,lower,settings=inputs()
    with pytest.raises(PreparationError,match='change the reference'):
        calibrate_jaw(points,'EI',upper,lower,1000,settings)
    with pytest.raises(PreparationError,match='Invalid jaw'):
        calibrate_jaw(points,'EI',upper,lower,100,dict(settings,minimum_clearance_mouth_width_ratio=float('nan')))

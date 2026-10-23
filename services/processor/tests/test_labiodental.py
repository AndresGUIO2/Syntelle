import numpy as np
from PIL import Image
import pytest

from app.preparation.labiodental import fit_contact
from app.preparation.faces import PreparationError
from app.preparation.semantic_rig import row_profile,pose_quality


def fixture():
    h,w=80,120;yy,xx=np.indices((h,w))
    image=Image.fromarray(np.stack((xx+40,yy+40,xx+yy),axis=-1).astype('uint8'))
    masks={name:Image.new('L',(w,h)) for name in ('outer_lips','aperture','upper_teeth','lower_teeth','tongue')}
    def rect(name,x0,y0,x1,y1):
        a=np.zeros((h,w),np.uint8);a[y0:y1,x0:x1]=255;masks[name]=Image.fromarray(a)
    rect('outer_lips',20,15,100,65)
    masks['aperture']=Image.fromarray(((((xx-60)/35)**2+((yy-38)/10)**2)<=1).astype(np.uint8)*255)
    rect('upper_teeth',40,28,80,38);rect('lower_teeth',40,44,80,48)
    material=np.zeros((h,w,4),np.uint8);material[15:38,40:80]=[35,26,18,255]
    return image,masks,Image.fromarray(material)


def test_contact_closes_to_fixed_incisors_without_opening_or_changing_upper_lip():
    image,masks,material=fixture()
    corrected,cavity,regions,record=fit_contact(image,image,masks,material,0)
    assert record['applied'] and record['aperture_after_px']<record['aperture_before_px']
    before=np.asarray(masks['aperture'])>0;after=np.asarray(regions['aperture'])>0
    assert not (after&~before).any()
    assert np.array_equal(np.asarray(corrected)[:28],np.asarray(image)[:28])
    assert not np.asarray(regions['lower_teeth']).any()
    assert pose_quality('FV',regions)['ready']
    lower=row_profile(regions['aperture'],'bottom');incisal=row_profile(material.getchannel('A'),'bottom')
    assert abs(np.nanmedian(lower-incisal))<=1
    assert not np.array_equal(np.asarray(corrected)[38:65],np.asarray(image)[38:65])
    # A dark upper arch is still an opaque reference; contact is not whitened
    # texture detection and never changes the supplied material.
    assert np.all(np.asarray(material)[20:38,40:80,3]==255)


def test_contact_rejects_missing_shared_enamel():
    image,masks,material=fixture()
    with pytest.raises(PreparationError,match='insufficient'):fit_contact(image,image,masks,Image.new('RGBA',material.size),0)

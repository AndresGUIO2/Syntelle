"""Measured F/V contact: roll the lower lip to the fixed upper incisal edge.

No character coordinates, individual-tooth movement or aperture enlargement.
The local texture warp is constrained by observed inner/outer lip boundaries.
"""
import cv2
import numpy as np
from PIL import Image

from .faces import PreparationError
from .semantic_rig import row_profile


def fit_contact(image,cavity,masks,upper_material,upper_y):
    opening=np.asarray(masks['aperture'])>=128
    bottom=row_profile(masks['aperture'],'bottom');top=row_profile(masks['aperture'],'top')
    outer=row_profile(masks['outer_lips'],'bottom')
    enamel=upper_material.getchannel('A')
    # The material's incisal silhouette is fixed, including dark enamel.
    incisal=row_profile(enamel,'bottom')+upper_y
    common=np.isfinite(bottom)&np.isfinite(incisal)
    columns=np.flatnonzero(common)
    if len(columns)<3:raise PreparationError('FV has insufficient shared upper incisal coverage for contact.')
    width=len(np.flatnonzero(np.isfinite(bottom)))
    existing_gap=bottom[columns]-incisal[columns]
    no_lower=not (np.asarray(masks['lower_teeth'])>=128).any()
    if no_lower and float(np.median(existing_gap))<=1.5:
        return image,cavity,masks,{'method':'measured-labiodental-contact/1','applied':False,'reason':'Existing shared-incisor contact is usable.'}
    h,w=opening.shape;xs=np.arange(w);active=np.flatnonzero(np.isfinite(bottom))
    if len(active)<3:raise PreparationError('FV has no usable observed aperture.')
    old_inner=np.interp(xs,active,bottom[active])+1
    old_outer=np.interp(xs,np.flatnonzero(np.isfinite(outer)),outer[np.isfinite(outer)])+1
    aperture_top=np.interp(xs,active,top[active])
    desired=aperture_top+.75
    desired[columns]=incisal[columns]+.5
    desired=cv2.GaussianBlur(desired[None,:],(0,0),max(.6,width*.025))[0]
    # Contact is constrained inside the existing opening. Corners taper to the
    # unmodified lip; no tooth is moved and no opening is made larger.
    support=np.zeros(w,float);support[active]=1
    delta=np.minimum(0,desired-old_inner)*support
    delta[:active.min()]=0;delta[active.max()+1:]=0
    if -float(delta.min())>max(3,width*.22):
        raise PreparationError('FV contact needs excessive lower-lip displacement.')
    new_inner=old_inner+delta
    new_outer=np.maximum(new_inner+1,old_outer+delta*1.1)
    far=old_outer+max(3,width*.25)
    yy,xx=np.indices((h,w),dtype=np.float32);map_y=yy.copy()
    lip=(yy>=new_inner)&(yy<new_outer)
    chin=(yy>=new_outer)&(yy<far)
    lip_source=old_inner+(yy-new_inner)*(old_outer-old_inner)/np.maximum(1,new_outer-new_inner)
    chin_source=old_outer+(yy-new_outer)*(far-old_outer)/np.maximum(1,far-new_outer)
    map_y[lip]=lip_source[lip];map_y[chin]=chin_source[chin]
    moved=np.abs(map_y-yy)>.001
    def warp(im,nearest=False):
        result=cv2.remap(np.asarray(im),xx,map_y.astype(np.float32),cv2.INTER_NEAREST if nearest else cv2.INTER_CUBIC,borderMode=cv2.BORDER_REFLECT_101)
        original=np.asarray(im);result[~moved]=original[~moved]
        return Image.fromarray(result)
    corrected=warp(image);corrected_cavity=warp(cavity)
    regions={name:warp(mask,True) for name,mask in masks.items()}
    aperture=opening&(yy<new_inner)
    regions['aperture']=Image.fromarray(aperture.astype(np.uint8)*255)
    shifted=cv2.warpAffine(np.asarray(enamel),np.array([[1,0,0],[0,1,upper_y]],float),(w,h),flags=cv2.INTER_NEAREST)
    regions['upper_teeth']=Image.fromarray(((shifted>=128)&aperture).astype(np.uint8)*255)
    regions['lower_teeth']=Image.new('L',(w,h))
    # The unchanged cavity/tongue remains behind the newly occluding lip.
    tongue=(np.asarray(masks['tongue'])>=128)&aperture
    regions['tongue']=Image.fromarray(tongue.astype(np.uint8)*255)
    if (aperture&~opening).any():raise PreparationError('FV correction enlarged the observed aperture.')
    return corrected,corrected_cavity,regions,{'method':'measured-labiodental-contact/1','applied':True,
        'maximum_displacement_px':float(-delta.min()),'contact_columns':int(len(columns)),
        'aperture_before_px':int(opening.sum()),'aperture_after_px':int(aperture.sum()),
        'upper_arch_moved':False,'manual_coordinates':False,'visible_lower_teeth':0,
        'inner_lip_displacement_by_column':delta.tolist(),'outer_lip_displacement_by_column':(new_outer-old_outer).tolist()}

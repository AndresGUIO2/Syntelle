"""Complete material continuity around immutable selected fragments, entirely offline."""
import cv2
import numpy as np
from PIL import Image

VERSION='dental-material-continuity/1'


def reconcile_completion(completed, selected, blend_width=6):
    """Match generated boundary color to its original neighbour; retain selected RGB."""
    result=np.array(completed.convert('RGBA'))
    original=np.asarray(selected.convert('RGBA'))
    protected=original[...,3]>=128
    distance,labels=cv2.distanceTransformWithLabels((~protected).astype(np.uint8),cv2.DIST_L2,5,labelType=cv2.DIST_LABEL_PIXEL)
    lookup=np.vstack((np.zeros((1,3)),original[protected,:3].astype(float)))
    nearest=lookup[np.minimum(labels,len(lookup)-1)]
    amount=np.clip((distance-1)/max(1,blend_width-1),0,1)
    amount=amount*amount*(3-2*amount)
    generated=(result[...,3]>0)&~protected
    blended=nearest*(1-amount[...,None])+result[...,:3]*amount[...,None]
    result[generated,:3]=np.rint(blended[generated]).astype(np.uint8)
    # Despill generated material only; retain photographed enamel/shadows verbatim.
    green=generated&(result[...,1]>result[...,0])&(result[...,1]>result[...,2])
    result[...,1][green]=np.rint((result[...,0][green].astype(float)+result[...,2][green])/2).astype(np.uint8)
    # Subpixel silhouette coverage, without brightness-dependent enamel opacity.
    alpha=result[...,3].astype(float)/255
    smooth=cv2.GaussianBlur(alpha,(0,0),.45)
    inside=cv2.distanceTransform((alpha>=.5).astype(np.uint8),cv2.DIST_L2,cv2.DIST_MASK_PRECISE)
    smooth[inside>=1.5]=1
    smooth[alpha==0]=0
    result[...,3]=np.rint(smooth*255).astype(np.uint8)
    result[protected,:3]=original[protected,:3]
    result[protected,3]=255
    return Image.fromarray(result),{'version':VERSION,'blend_width_px':blend_width,
        'generated_seam_pixels':int((generated&(distance<blend_width)).sum()),
        'protected_pixels':int(protected.sum()),'protected_RGB_restored':bool(np.array_equal(result[protected,:3],original[protected,:3]))}

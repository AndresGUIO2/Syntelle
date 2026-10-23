"""Reviewed semantic masks and rigid reference dentition for the local character."""
import cv2
import numpy as np
from PIL import Image, ImageDraw, ImageFilter
from .mesh import INNER
from .faces import PreparationError


def polygon_mask(size, polygons, offset=(0,0), feather=0):
    mask=Image.new("L",size,0); draw=ImageDraw.Draw(mask)
    for polygon in polygons:
        if len(polygon)>=3: draw.polygon([(x+offset[0],y+offset[1]) for x,y in polygon],fill=255)
    if feather: mask=mask.filter(ImageFilter.GaussianBlur(feather))
    return mask


def remove_teeth(texture, removal, tongue, radius=3, dilate=2, limit=None):
    raw=np.asarray(texture.convert("RGB"))
    mask=np.asarray(removal).copy()
    if dilate: mask=cv2.dilate(mask,np.ones((2*dilate+1,2*dilate+1),np.uint8))
    if limit is not None: mask=np.minimum(mask,np.asarray(limit))
    mask[np.asarray(tongue)>0]=0
    fixed=cv2.inpaint(raw,mask,radius,cv2.INPAINT_TELEA) if mask.any() else raw.copy()
    # Defend against later algorithm changes affecting protected tongue pixels.
    protected=np.asarray(tongue)>0
    fixed[protected]=raw[protected]
    return Image.fromarray(fixed),Image.fromarray(mask)


def cavity_mask(size, points, inset_px=0):
    mask=polygon_mask(size,[points[INNER,:2].tolist()])
    if inset_px>0:
        distance=cv2.distanceTransform(np.asarray(mask),cv2.DIST_L2,cv2.DIST_MASK_PRECISE)
        # Keep enamel inside the lip border; one pixel of antialiasing at the inset.
        mask=Image.fromarray(np.rint(np.clip(distance-inset_px,0,1)*255).astype(np.uint8))
    return mask


def reference_row(texture, mask):
    row=texture.convert("RGBA"); row.putalpha(mask)
    return row


def reviewed_dental_masks(record, selected, source_sha256, raw_sha256, crop):
    """Replay character annotations only against the exact inspected original/crop."""
    if (record['source_pose']!=selected or record['source_sha256']!=source_sha256
            or record['raw_sha256']!=raw_sha256 or record['coordinates']!={'space':'aligned-crop-pixels','crop':crop}):
        raise PreparationError('Reviewed dental masks do not match the selected original and aligned crop.')
    if record['status']!='internal_candidate':
        raise PreparationError('Reviewed dental masks are not an available internal candidate.')
    width,height=crop['width'],crop['height']
    scale=record['rasterization']['supersampling']
    if not isinstance(scale,int) or not 1<=scale<=8 or record['rasterization']['resampling']!='LANCZOS':
        raise PreparationError('Unsupported reviewed-mask rasterization settings.')
    masks=[]
    for row in ('upper','lower'):
        if not record[row]: raise PreparationError('A reviewed dental row is empty.')
        polygons=[]
        for polygon in record[row]:
            points=np.asarray(polygon,dtype=float)
            if (points.ndim!=2 or points.shape[1]!=2 or len(points)<3 or not np.isfinite(points).all()
                    or (points<0).any() or (points[:,0]>=width).any() or (points[:,1]>=height).any()):
                raise PreparationError('Invalid reviewed dental contour.')
            polygons.append((points*scale).tolist())
        mask=polygon_mask((width*scale,height*scale),polygons).resize((width,height),Image.Resampling.LANCZOS)
        masks.append(mask)
    if ((np.asarray(masks[0])>=128)&(np.asarray(masks[1])>=128)).any():
        raise PreparationError('Reviewed upper/lower dental rows overlap.')
    return tuple(masks)


def dental_bite_stop(upper_mask, lower_mask, minimum_gap):
    """Stop the lower row at the opposing enamel in each shared image column."""
    upper=np.asarray(upper_mask)>=128
    lower=np.asarray(lower_mask)>=128
    columns=np.flatnonzero(upper.any(axis=0)&lower.any(axis=0))
    if not len(columns): raise PreparationError('Dental rows have no shared columns for a bite stop.')
    gaps=[int(np.flatnonzero(lower[:,x]).min()-np.flatnonzero(upper[:,x]).max()) for x in columns]
    if min(gaps)<=0: raise PreparationError('Reference dental rows overlap or cross.')
    return int(np.ceil(minimum_gap-min(gaps))),{'shared_columns':columns.tolist(),'reference_gaps_px':gaps}


def add_dentition(texture, upper, lower, points, settings):
    result=texture.convert("RGBA")
    clip=np.asarray(cavity_mask(result.size,points,settings.get('dental_inset_px',0)),dtype=float)/255
    if settings.get("upper_visible"):
        row=upper.copy(); row.putalpha(Image.fromarray(np.rint(np.asarray(row.getchannel("A"))*clip).astype(np.uint8)))
        result.alpha_composite(row)
    if settings.get("lower_visible"):
        dx,dy=settings["lower_translation"]
        moved=Image.new("RGBA",result.size); moved.alpha_composite(lower,(int(dx),int(dy)))
        moved.putalpha(Image.fromarray(np.rint(np.asarray(moved.getchannel("A"))*clip).astype(np.uint8)))
        result.alpha_composite(moved)
    return result.convert("RGB")

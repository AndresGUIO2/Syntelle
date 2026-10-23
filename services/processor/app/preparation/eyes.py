"""Texture-preserving eyelid coverage from open and closed landmark contours."""
import cv2
import numpy as np
from PIL import Image

from .faces import PreparationError
from .mouth import fit_similarity, transform_points

# MediaPipe topology, never character-specific coordinates. Moving eye and lip
# points are deliberately absent from the similarity fit.
EYE_ANCHORS = (10,67,297,109,338,127,356,6,168,197,195,4,98,327)
EYE_RINGS = ((33,246,161,160,159,158,157,173,133,155,154,153,145,144,163,7),
             (362,398,384,385,386,387,388,466,263,249,390,373,374,380,381,382))


def face_crop(image, points):
    xy=points[:468,:2];low=xy.min(0);high=xy.max(0)
    side=min(min(image.size),int(np.ceil(max(high-low)*1.3)))
    center=(low+high)/2;left=int(np.clip(center[0]-side/2,0,image.width-side));top=int(np.clip(center[1]-side/2,0,image.height-side))
    return (left,top,left+side,top+side)


def align_eyes(base, generated, base_points, generated_points, expected_scale):
    indices=np.array(EYE_ANCHORS)
    matrix,residual=fit_similarity(generated_points[indices,:2],base_points[indices,:2])
    width=float(np.linalg.norm(base_points[33,:2]-base_points[263,:2]))
    scale=float(np.linalg.norm(matrix[:,0]));angle=float(np.degrees(np.arctan2(matrix[1,0],matrix[0,0])))
    if residual>width*.04 or not .85<scale/expected_scale<1.15 or abs(angle)>5:
        raise PreparationError('Closed-eye face cannot be aligned reliably without moving facial identity.')
    aligned=Image.fromarray(cv2.warpAffine(np.asarray(generated.convert('RGB')),matrix,base.size,
        flags=cv2.INTER_LANCZOS4,borderMode=cv2.BORDER_REFLECT_101))
    return aligned,transform_points(generated_points,matrix),{'anchors':list(EYE_ANCHORS),
        'transform':matrix.tolist(),'anchor_rmse_px':residual,'scale':scale,'rotation_degrees':angle}


def profile(points, indices, xs):
    p=points[list(indices),:2];order=np.argsort(p[:,0]);x,unique=np.unique(p[order,0],return_index=True)
    return np.interp(xs,x,p[order,1][unique])


def prepare_eyes(base,closed,base_points,closed_points):
    all_points=np.concatenate([points[list(ring),:2] for points in (base_points,closed_points) for ring in EYE_RINGS])
    eye_width=max(np.ptp(base_points[list(r),0]) for r in EYE_RINGS)
    padding=max(4,round(eye_width*.22))
    left,top=np.maximum(0,np.floor(all_points.min(0)-padding)).astype(int)
    right,bottom=np.minimum(base.size,np.ceil(all_points.max(0)+padding+1)).astype(int)
    bounds=(int(left),int(top),int(right),int(bottom));w,h=right-left,bottom-top
    mask=np.zeros((h,w),np.uint8);curves=np.zeros((w,4),np.float64);observations=[]
    for ring in EYE_RINGS:
        upper=ring[:9];lower=(ring[0],*ring[:8:-1],ring[8])
        x0=int(np.floor(min(base_points[list(ring),0].min(),closed_points[list(ring),0].min())))
        x1=int(np.ceil(max(base_points[list(ring),0].max(),closed_points[list(ring),0].max())))
        xs=np.arange(x0,x1+1,dtype=float)
        a=profile(base_points,upper,xs)-top;b=profile(base_points,lower,xs)-top
        a,b=np.minimum(a,b),np.maximum(a,b)
        ca=profile(closed_points,upper,xs)-top;cb=profile(closed_points,lower,xs)-top
        seam=(ca+cb)/2
        central=slice(max(1,len(xs)//5),max(2,len(xs)*4//5))
        open_gap=float(np.median((b-a)[central]));closed_gap=float(np.median(np.abs(ca-cb)[central]))
        if open_gap<2 or closed_gap>max(2,open_gap*.4):
            raise PreparationError('The proposed eyes are not confidently closed relative to the original.')
        if np.any(seam<=1) or np.any(seam>=h-2):raise PreparationError('Closed eyelid lies outside its prepared patch.')
        columns=(xs-left).astype(int);curves[columns,:]=np.column_stack((a,b,seam,np.ones(len(xs))))
        local=np.concatenate((base_points[list(ring),:2],closed_points[list(ring),:2]))-[left,top]
        hull=cv2.convexHull(np.rint(local).astype(np.int32));part=np.zeros_like(mask);cv2.fillConvexPoly(part,hull,255)
        radius=max(2,round((x1-x0)*.13));part=cv2.dilate(part,cv2.getStructuringElement(cv2.MORPH_ELLIPSE,(2*radius+1,)*2))
        mask=np.maximum(mask,part)
        observations.append({'open_gap_px':open_gap,'closed_gap_px':closed_gap,'width_px':x1-x0})
    mask=cv2.GaussianBlur(mask,(0,0),max(1,eye_width*.018));mask[[0,-1],:]=0;mask[:,[0,-1]]=0
    for amount in np.linspace(0,1,101):
        active=curves[:,3]>0
        upper=(1-amount)*curves[:,0]+amount*curves[:,2]
        lower=(1-amount)*curves[:,1]+amount*curves[:,2]
        if (upper[active]>lower[active]+1e-7).any():raise PreparationError('Inverted eyelid opening.')
    return bounds,Image.fromarray(mask),curves,observations


def render_eyes(base, closed, mask, curves, amount):
    """CPU inspection/reference; exposed original iris pixels never move."""
    a=np.asarray(base.convert('RGB'));b=np.asarray(closed.convert('RGB'));h,w=a.shape[:2]
    amount=float(np.clip(amount,0,1))
    if amount==0:return base.copy()
    yy,xx=np.indices((h,w),dtype=np.float32)
    upper=(1-amount)*curves[:,0]+amount*curves[:,2]
    lower=(1-amount)*curves[:,1]+amount*curves[:,2]
    is_upper=yy<upper;active=curves[:,3]>0
    changes=np.diff(np.r_[False,active,False].astype(int));runs=np.flatnonzero(changes==-1)-np.flatnonzero(changes==1)
    band=max(2,float(runs.max())*.12)
    band=np.minimum(band,np.minimum(np.minimum(curves[:,0],curves[:,2]),np.minimum(h-1-curves[:,1],h-1-curves[:,2]))*.8)
    def sample_y(source_upper,source_lower):
        up=np.where(yy>=upper-band,yy+source_upper-upper,yy*(source_upper-band)/np.maximum(upper-band,1))
        down=np.where(yy<=lower+band,yy+source_lower-lower,
            h-1+(yy-(h-1))*(source_lower+band-(h-1))/np.minimum(lower+band-(h-1),-1))
        return np.where(is_upper,up,down)
    base_y=sample_y(curves[:,0],curves[:,1])
    closed_y=sample_y(curves[:,2],curves[:,2])
    base_y=np.where(is_upper,np.minimum(base_y,curves[:,0]-.5),np.maximum(base_y,curves[:,1]+.5))
    sample=lambda im,y:cv2.remap(im,xx,y.astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101).astype(float)
    skin=sample(a,base_y)*(1-amount)+sample(b,closed_y)*amount
    visible=np.clip(np.minimum(yy+.5,lower)-np.maximum(yy-.5,upper),0,1)
    result=skin*(1-visible[...,None])+a*visible[...,None]
    result[:,~active]=a[:,~active]*(1-amount)+b[:,~active]*amount
    if amount==1:result=b.astype(float)
    alpha=np.asarray(mask)[...,None]/255
    return Image.fromarray(np.rint(np.clip(result*alpha+a*(1-alpha),0,255)).astype(np.uint8))

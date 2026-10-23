"""Source-bound facial morphs; independent of the registered manual smile.

All coordinates come from cached MediaPipe topology and measured registration.
Reference enamel is never baked into the animated skin material.
"""
from __future__ import annotations

import cv2
import numpy as np
from PIL import Image
from scipy.spatial import Delaunay

from .eyes import EYE_RINGS, profile
from .faces import PreparationError
from .mesh import INNER, OUTER

FACE_OVAL = (10,338,297,332,284,251,389,356,454,323,361,288,397,365,379,378,400,
             377,152,148,176,149,150,136,172,58,132,93,234,127,162,21,54,103,67,109)
FACE_INTERIOR = (168,6,197,195,5,4,2,98,327,50,280,101,330,205,425,187,411,
                 70,63,105,66,107,336,296,334,293,300)


def filled_polygon(size, points):
    result = np.zeros((size[1], size[0]), np.uint8)
    cv2.fillPoly(result, [np.rint(np.asarray(points)*4).astype(np.int32)], 255, shift=2)
    return result


def contact_shadow(oral, inner):
    """Reuse oral lighting and measured aperture; never modify enamel pixels."""
    gap=max(0.,float(inner[-1][15][1]-inner[-1][5][1]))
    lighting=oral.get('oral',{})
    strength=float(lighting.get('contact_shadow_strength',.16))
    width=float(lighting.get('contact_shadow_width_px',3.))
    if not np.isfinite([gap,strength,width]).all() or not 0<=strength<=.45 or width<=0:
        raise PreparationError('Reference contact lighting exceeds its bounded profile.')
    return {'version':'aperture-contact-shadow/1','strength':strength,
            'width_px':max(width,gap*.12)}


def mesh_maps(vertices, source, target, triangles, xy):
    """Rasterize the validated connectivity; never retriangulate a morph frame."""
    height,width=xy.shape[:2]
    output_a=xy.copy();output_b=xy.copy()
    origin=xy[0,0]
    for triangle in triangles:
        v=vertices[triangle]-origin
        left,top=np.maximum(0,np.floor(v.min(0))).astype(int)
        right,bottom=np.minimum([width,height],np.ceil(v.max(0))+1).astype(int)
        if right<=left or bottom<=top:
            continue
        yy,xx=np.indices((bottom-top,right-left),float)
        x=xx+left;y=yy+top
        p,q=v[1]-v[0],v[2]-v[0]
        determinant=p[0]*q[1]-p[1]*q[0]
        if abs(determinant)<1e-8:
            continue
        wb=((x-v[0,0])*q[1]-(y-v[0,1])*q[0])/determinant
        wc=(p[0]*(y-v[0,1])-p[1]*(x-v[0,0]))/determinant
        wa=1-wb-wc
        inside=(wa>=-1e-6)&(wb>=-1e-6)&(wc>=-1e-6)
        for result,material in ((output_a,source),(output_b,target)):
            value=wa[...,None]*material[triangle[0]]+wb[...,None]*material[triangle[1]]+wc[...,None]*material[triangle[2]]
            result[top:bottom,left:right][inside]=value[inside]
    return output_a,output_b


def facial_mesh(points, target, size):
    """Use a common mesh, preserving full reference amplitude inside the oval."""
    indices = sorted(set(FACE_OVAL + FACE_INTERIOR + tuple(OUTER) + EYE_RINGS[0] + EYE_RINGS[1]))
    a, b = points[indices, :2].copy(), target[indices, :2].copy()
    # The jaw contour must follow the received expression. Attach the moving
    # face to a stationary outer support, rather than pinning the chin itself.
    width=float(np.linalg.norm(points[234,:2]-points[454,:2]))
    height=float(points[152,1]-points[10,1])
    low=np.minimum(a.min(0),b.min(0))-[width*.08,height*.04]
    high=np.maximum(a.max(0),b.max(0))+[width*.08,height*.22]
    low=np.maximum(low,[0,0]);high=np.minimum(high,np.asarray(size)-1)
    x0,y0=low;x1,y1=high
    support=np.array([[x0,y0],[(x0+x1)/2,y0],[x1,y0],[x1,(y0+y1)/2],
                      [x1,y1],[(x0+x1)/2,y1],[x0,y1],[x0,(y0+y1)/2]])
    a=np.vstack([a,support]);b=np.vstack([b,support])
    mid = (a+b)/2
    triangles = Delaunay(mid).simplices
    def area(v):
        p, q = v[triangles[:, 1]]-v[triangles[:, 0]], v[triangles[:, 2]]-v[triangles[:, 0]]
        return p[:, 0]*q[:, 1]-p[:, 1]*q[:, 0]
    original = area(a)
    # The oral hole is rendered separately, not treated as a skin triangle.
    centers = mid[triangles].mean(1).astype(np.float32)
    mouth = target[OUTER, :2].astype(np.float32)
    skin = np.array([cv2.pointPolygonTest(mouth, tuple(p), False) <= 0 for p in centers])
    relevant = skin & (abs(original) > .05)
    ratios = [area(a*(1-f)+b*f)[relevant]/original[relevant] for f in np.linspace(0,1,25)]
    minimum = min(float(v.min()) for v in ratios)
    if minimum <= 0:
        raise PreparationError('Reference facial mesh inverts; full expression cannot be reduced to hide this failure.')
    boundary=np.maximum(filled_polygon(size,points[list(FACE_OVAL),:2]),
                        filled_polygon(size,target[list(FACE_OVAL),:2]))
    jaw=np.vstack([points[list(FACE_OVAL),:2],target[list(FACE_OVAL),:2]])
    jaw=jaw[jaw[:,1]>points[152,1]-height*.20]
    neck_y=min(size[1]-1,max(points[152,1],target[152,1])+height*.18)
    neck=np.array([[points[152,0]-width*.28,neck_y],[points[152,0]+width*.28,neck_y]])
    cv2.fillConvexPoly(boundary,cv2.convexHull(np.rint(np.vstack([jaw,neck])).astype(np.int32)),255)
    mask = np.clip(cv2.distanceTransform(boundary,cv2.DIST_L2,5)/max(2,width*.035),0,1)
    mask = mask*mask*(3-2*mask)
    return a,b,triangles,mask,{'valid':True,'minimum_triangle_ratio':minimum,'amplitude':1.,
                              'indices':indices,'source':a.tolist(),'target':b.tolist(),
                              'triangles':triangles.tolist(),'configurations':25,
                              'chin_displacement_px':(target[152,:2]-points[152,:2]).tolist(),
                              'lower_lip_displacement_px':(target[17,:2]-points[17,:2]).tolist()}


def eye_profiles(points, target, motion):
    crop=motion['eyes']['crop'];x,y,w,h=(crop[k] for k in ('x','y','width','height'))
    base=np.asarray(motion['eyes']['curves'],float)
    result=[]
    for amount in np.linspace(0,1,25):
        curves=base.copy();offsets=[]
        for ring in EYE_RINGS:
            upper=ring[:9];lower=(ring[0],*ring[:8:-1],ring[8])
            source=points[list(ring),:2];dest=target[list(ring),:2]
            left=int(np.floor(min(source[:,0].min(),dest[:,0].min())))
            right=int(np.ceil(max(source[:,0].max(),dest[:,0].max())))
            if left < x or right >= x+w:
                raise PreparationError('Laughing eye exceeds its source-bound eye patch.')
            xs=np.arange(left,right+1,dtype=float)
            up=profile(points,upper,xs)*(1-amount)+profile(target,upper,xs)*amount-y
            down=profile(points,lower,xs)*(1-amount)+profile(target,lower,xs)*amount-y
            u,l=np.minimum(up,down),np.maximum(up,down)
            if u.min()<0 or l.max()>=h:
                raise PreparationError('Laughing eyelid exceeds prepared eye coverage.')
            shift=(dest[[0,8]].mean(0)-source[[0,8]].mean(0))*amount
            columns=(xs-x).astype(int)
            seam=np.clip(np.interp(xs-x,np.arange(w),base[:,2])+shift[1],u,l)
            curves[columns]=np.column_stack((u,l,seam,np.ones(len(xs))))
            offsets.append(shift.tolist())
        curves[:,2]=np.clip(curves[:,2],curves[:,0],curves[:,1])
        result.append({'curves':curves.tolist(),'iris_offsets':offsets})
    return {'crop':crop,'profiles':result,'iris_material':'original-rigid-samples',
            'tissue_ownership':'aperture-and-closing-lid-sweep-only'}


def prepare_frames(original, foreground, aligned, points, target, motion, oral, tooth_masks, output):
    """Warp both appearances into one geometry, with a separate oral aperture."""
    original=np.asarray(original.convert('RGB'));ref=np.asarray(aligned.convert('RGB')).copy()
    fg=np.asarray(foreground.convert('RGBA'))
    height,width=original.shape[:2]
    a,b,triangles,mask,validation=facial_mesh(points,target,(width,height))
    bound=np.nonzero(mask>0);left,top=int(bound[1].min()),int(bound[0].min())
    right,bottom=int(bound[1].max())+1,int(bound[0].max())+1
    crop={'x':left,'y':top,'width':right-left,'height':bottom-top}
    oc=oral['crop'];ox,oy,ow,oh=(oc[k] for k in ('x','y','width','height'))
    rows=np.maximum.reduce([np.asarray(m.convert('L')) for m in tooth_masks])
    if rows.shape!=(oh,ow):
        raise PreparationError('Reference tooth masks do not share the registered oral crop.')
    cavity=ref[oy:oy+oh,ox:ox+ow].copy()
    # This fill is hidden behind the opaque, separately sampled dental rows.
    # It prevents a second tooth arrangement in intermediate skin frames.
    # Preserve observed gums/lip contact pixels. Expanding this removal into
    # surrounding tissue creates artificial dark gaps beside the crowns.
    erase=np.uint8(rows>0)*255
    cavity=cv2.inpaint(cavity,erase,3,cv2.INPAINT_TELEA)
    Image.fromarray(cavity).save(output/'reference-cavity.webp',lossless=True,exact=True,method=6)
    ref[oy:oy+oh,ox:ox+ow]=cavity
    yy,xx=np.indices((bottom-top,right-left),np.float32)
    xy=np.stack((xx+left,yy+top),-1)
    active=mask[top:bottom,left:right,None]
    frames=[];inner=[];outer=[];lower=[]
    origin=np.array([ox,oy])
    for index,amount in enumerate(np.linspace(0,1,25)):
        vertices=a*(1-amount)+b*amount
        source_map,target_map=mesh_maps(vertices,a,b,triangles,xy)
        source=cv2.remap(original,source_map[...,0],source_map[...,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        reference=cv2.remap(ref,target_map[...,0],target_map[...,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        skin=source*(1-amount)+reference*amount
        neutral=fg[top:bottom,left:right,:3]
        rgb=np.rint(neutral*(1-active)+skin*active).astype(np.uint8)
        rgba=fg[top:bottom,left:right].copy();rgba[...,:3]=rgb
        # Iris samples stay source-exact and rigid. The live eye patch restores
        # their exact aperture and gaze; reference iris never enters the atlas.
        midpoints=points[:,:2]*(1-amount)+target[:,:2]*amount
        for ring in EYE_RINGS:
            shift=(midpoints[list(ring)[::8]][:2].mean(0)-points[list(ring)[::8]][:2,:2].mean(0))
            aperture=filled_polygon((right-left,bottom-top),midpoints[list(ring)]-[left,top])>0
            iris=cv2.remap(original,(xx+left-shift[0]).astype(np.float32),(yy+top-shift[1]).astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
            rgba[aperture,:3]=iris[aperture]
        if index==0:
            rgba=fg[top:bottom,left:right].copy()
        protected=active[...,0]==0
        if not np.array_equal(rgba[protected],fg[top:bottom,left:right][protected]):
            raise PreparationError('A facial frame changed protected foreground pixels.')
        name=f'reference-face-{index:02d}.webp'
        Image.fromarray(rgba).save(output/name,lossless=True,exact=True,method=6)
        frames.append({'file':name})
        inner.append((midpoints[INNER]-origin).tolist())
        outer.append((midpoints[OUTER]-origin).tolist())
        # The reference arch is already in its observed laughing placement.
        # Lip occlusion reveals it; audio jaw motion only opens the lower row.
        lower.append([0.,0.])
    jaw_limit=max(0.,float(target[14,1]-target[13,1]))*.08
    return {'version':'reference-laughter/1','crop':crop,'frames':frames,
            'eyes':eye_profiles(points,target,motion),
            'oral':{'inner':inner,'outer':outer,'lower_translation':lower,
                    'cavity':{'file':'reference-cavity.webp'},'jaw_limit_px':jaw_limit,
                    'contact_shadow':contact_shadow(oral,inner)},
            'validation':{**validation,'protected_pixels_unchanged':True},'manual_masks':False,'amplitude_reduction':1.,
            'reference_teeth_baked_into_skin':False}

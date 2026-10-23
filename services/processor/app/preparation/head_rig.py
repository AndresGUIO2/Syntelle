"""Source-bound head geometry; no character-specific masks or coordinates."""
from itertools import product

import cv2
import numpy as np
from scipy.interpolate import LinearNDInterpolator
from scipy.ndimage import gaussian_filter

from .faces import PreparationError
from .motion_geometry import smooth

VERSION = 'anatomical-head-rig/3'
BOUNDS = {'idle': [6., .9, .6], 'speaking': [8., 2., 1.2]}


def deform(vertices, pivot, size, angles, breath):
    v=np.asarray(vertices);xy=v[:,:2]*size;p=np.column_stack((xy-pivot,v[:,2]))
    yaw,pitch,roll=np.radians(angles);cy,sy=np.cos(yaw),np.sin(yaw);cp,sp=np.cos(pitch),np.sin(pitch);cr,sr=np.cos(roll),np.sin(roll)
    a=np.column_stack((cy*p[:,0]+sy*p[:,2],p[:,1],-sy*p[:,0]+cy*p[:,2]))
    b=np.column_stack((a[:,0],cp*a[:,1]-sp*a[:,2],sp*a[:,1]+cp*a[:,2]))
    rotated=np.column_stack((cr*b[:,0]-sr*b[:,1],sr*b[:,0]+cr*b[:,1]))
    result=xy+v[:,3,None]*(rotated-p[:,:2])
    result[:,1]+=v[:,4]*breath
    result[:,0]+=v[:,4]*(xy[:,0]-pivot[0])*breath/size[1]*.25
    return result


def validate(vertices,pivot,size,n,bounds,face_height):
    grid=np.arange((n+1)**2).reshape(n+1,n+1);a=grid[:-1,:-1].ravel();b=a+1;c=a+n+1
    triangles=np.vstack((np.column_stack((a,b,c)),np.column_stack((c,b,c+1))))
    original_area=size[0]*size[1]/n**2;minimum=float('inf')
    for angles in product(np.linspace(-1,1,5),repeat=3):
        for breath in [-1,0,1]:
            points=deform(vertices,pivot,size,np.asarray(angles)*bounds,breath*face_height*.006)
            t=points[triangles];ab=t[:,1]-t[:,0];ac=t[:,2]-t[:,0]
            area=ab[:,0]*ac[:,1]-ab[:,1]*ac[:,0]
            minimum=min(minimum,float(area.min()/original_area))
    return {'configurations':375,'minimum_area_ratio':minimum,'valid':bool(np.isfinite(minimum) and minimum>.2)}


def prepare(points,classes,depth,size):
    points=np.asarray(points,dtype=float);w,h=size;n=64
    if points.shape!=(478,3) or not np.isfinite(points).all():raise PreparationError('Incomplete head landmarks.')
    face_height=float(points[152,1]-points[10,1]);face_width=float(np.linalg.norm(points[234,:2]-points[454,:2]))
    if face_height<=0 or face_width<=0:raise PreparationError('Invalid anatomical dimensions.')
    pivot=[float((points[234,0]+points[454,0])/2),float(points[152,1]+face_height*.18)]
    yy,xx=np.mgrid[0:n+1,0:n+1];uv=np.column_stack((xx.ravel()/n,yy.ravel()/n));xy=uv*[w,h]
    d=cv2.resize(np.asarray(depth,dtype=float),(n+1,n+1)).ravel()/255
    z=(d-.5)*face_width*.35
    # MediaPipe Z uses image-width units, opposite the near-positive depth field.
    facial=LinearNDInterpolator(points[:468,:2],-points[:468,2]+points[152,2])(xy)
    available=np.isfinite(facial);z[available]=facial[available]
    z=gaussian_filter(z.reshape(n+1,n+1),.6).ravel()
    semantic=np.stack([cv2.resize(p,(n+1,n+1)) for p in classes])
    head_material=np.clip(semantic[1]+semantic[3]+semantic[5],0,1).ravel()
    neck=1-smooth(points[152,1]-face_height*.08,points[152,1]+face_height*.5,xy[:,1])
    # A rigid face/head region blends into a small shoulder follow, never a separate eye/mouth transform.
    head=neck*np.maximum(head_material,.1)
    head[xy[:,1]<points[152,1]-face_height*.08]=1
    head=gaussian_filter(head.reshape(n+1,n+1),.7).ravel()
    torso=smooth(points[152,1]+face_height*.28,points[152,1]+face_height*.7,xy[:,1])
    torso*=1-smooth(h-face_height*.15,h,xy[:,1])
    torso*=np.clip(semantic[2]+semantic[4]+semantic[5],0,1).ravel()
    vertices=np.column_stack((uv,z,head,torso))
    factor=1.
    for _ in range(16):
        report=validate(vertices,pivot,np.array(size),n,np.array(BOUNDS['speaking'])*factor,face_height)
        if report['valid']:break
        factor*=.85
    else:raise PreparationError('Head motion envelope cannot be prepared without inverted geometry.')
    return {'version':VERSION,'grid':n,'vertices':vertices.round(7).tolist(),'pivot':pivot,
            'bounds':{key:(np.array(value)*factor).tolist() for key,value in BOUNDS.items()},
            'posture':{'version':'head-posture/3','minimum_yaw_fraction':.65,'turn_duration_ms':2600},
            'amplitude_factor':factor,'geometry_validation':report,'manual_overrides':False}

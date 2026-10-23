"""Calibrated RGB-D surface, projective source texture and bounded head rotation."""
from itertools import product

import cv2
import numpy as np
from scipy.interpolate import LinearNDInterpolator
from scipy.ndimage import distance_transform_edt

from .faces import PreparationError
from .motion_geometry import smooth

VERSION = 'rgbd-perspective/1'
SETTINGS = {'grid': 128, 'focal_image_fraction': 1.5, 'residual_face_fraction': .045,
            'face_blend_fraction': .10, 'bounds_degrees': [3., 1.2, .3],
            'maximum_depth_slope': 6.,
            'minimum_yaw_degrees': 2., 'minimum_area_ratio': .25}


def robust_affine(x, y):
    x, y = np.asarray(x, float), np.asarray(y, float)
    if len(x) < 40 or not np.isfinite([*x, *y]).all() or np.std(x) < 1e-7:
        raise PreparationError('Insufficient independent facial depth observations.')
    center, scale = np.median(x), np.std(x)
    design = np.column_stack(((x-center)/scale, np.ones(len(x))))
    weights = np.ones(len(x))
    for _ in range(12):
        coef = np.linalg.lstsq(design*weights[:,None]**.5, y*weights**.5, rcond=None)[0]
        residual = y-design@coef
        sigma = max(1e-5, 1.4826*np.median(np.abs(residual-np.median(residual))))
        weights = np.minimum(1., 1.345*sigma/np.maximum(np.abs(residual), 1e-9))
    slope = float(coef[0]/scale)
    if slope <= 0:
        raise PreparationError('Dense depth and facial anchors disagree in depth direction.')
    return slope, float(coef[1]-slope*center), {
        'anchors': len(x), 'slope': slope, 'offset': float(coef[1]-slope*center),
        'median_absolute_error_px': float(np.median(np.abs(residual))),
        'downweighted_anchors': int((weights < .5).sum()), 'iterations': 12,
    }


def sample(field, xy):
    return cv2.remap(np.asarray(field, np.float32), np.asarray(xy[:,0], np.float32).reshape(-1,1),
                     np.asarray(xy[:,1], np.float32).reshape(-1,1), cv2.INTER_LINEAR,
                     borderMode=cv2.BORDER_REPLICATE).ravel()


def calibrate(raw, points, classes, eye_rect, settings=SETTINGS):
    h,w = raw.shape
    p = np.asarray(points, float)[:468]
    if p.shape != (468,3) or not np.isfinite(raw).all() or not np.isfinite(p).all():
        raise PreparationError('Invalid full-precision depth or landmarks.')
    fw = float(np.linalg.norm(p[234,:2]-p[454,:2]))
    x,y = p[:,0],p[:,1]
    e = eye_rect
    eyes = (x>=e['x']) & (x<=e['x']+e['width']) & (y>=e['y']) & (y<=e['y']+e['height'])
    mouth = p[[61,291,0,17],:2]
    lo,hi = mouth.min(0)-fw*.035, mouth.max(0)+fw*.035
    lips = (x>=lo[0]) & (x<=hi[0]) & (y>=lo[1]) & (y<=hi[1])
    inside = (x>=0)&(x<w)&(y>=0)&(y<h)
    stable = inside & ~eyes & ~lips
    skin = cv2.resize(classes[3],(w,h))
    stable &= sample(skin,p[:,:2])>.15
    target = -p[:,2]+p[152,2]
    a,b,report = robust_affine(sample(raw,p[stable,:2]),target[stable])
    lo,hi=np.percentile(target[stable],[1,99]);margin=(hi-lo)*.10
    # Relative depth is not metric: extrapolation beyond the observed facial
    # range otherwise turns removed-background/neck edges into extreme cliffs.
    dense = np.clip(a*raw+b,lo-margin,hi+margin).astype(np.float32)
    report['depth_interval_px']=[float(lo-margin),float(hi+margin)]
    # Work at a bounded resolution for the edge-preserving low-frequency field.
    factor=min(1.,640/max(w,h));size=(max(2,round(w*factor)),max(2,round(h*factor)))
    low=cv2.bilateralFilter(cv2.resize(dense,size),9,fw*.035,4)
    low=cv2.resize(low,(w,h),interpolation=cv2.INTER_LINEAR)
    interpolator=LinearNDInterpolator(p[stable,:2],target[stable])
    # Evaluate only on the mesh later; the archived residual retains source detail.
    residual=np.clip(dense-low,-fw*settings['residual_face_fraction'],fw*settings['residual_face_fraction'])
    hull=np.zeros((h,w),np.uint8)
    cv2.fillConvexPoly(hull,cv2.convexHull(np.rint(p[stable,:2]).astype(np.int32)),1)
    blend=smooth(0,fw*settings['face_blend_fraction'],cv2.distanceTransform(hull,cv2.DIST_L2,5))
    report.update({'excluded_eye_anchors':int(eyes.sum()),'excluded_mouth_anchors':int(lips.sum()),
                   'stable_indices':np.flatnonzero(stable).tolist(), 'residual_limit_px':fw*settings['residual_face_fraction']})
    return dense, residual, blend, interpolator, report


def unproject(vertices, camera):
    v=np.asarray(vertices,float);size=np.asarray(camera['size']);f=camera['focal'];d=camera['distance']
    q=d-v[:,2]
    return np.column_stack(((v[:,:2]*size-np.asarray(camera['center']))*q[:,None]/f,v[:,2]))


def limit_slopes(z, size, grid, maximum):
    field=np.asarray(z,float).reshape(grid+1,grid+1).copy()
    for _ in range(256):
        for axis,spacing in [(1,size[0]/grid),(0,size[1]/grid)]:
            view=field if axis==1 else field.T
            for parity in [0,1]:
                left=view[:,parity:-1:2];right=view[:,parity+1::2]
                delta=right-left
                correction=(delta-np.clip(delta,-maximum*spacing,maximum*spacing))*.5
                left+=correction;right-=correction
        if all(np.max(abs(np.diff(field,axis=axis)))<=maximum*spacing+1e-4
               for axis,spacing in [(1,size[0]/grid),(0,size[1]/grid)]):
            return field.ravel()
    raise PreparationError('Depth slope conditioning did not converge.')


def project(vertices, camera, pivot, angles, breath=0):
    v=np.asarray(vertices,float);world=unproject(v,camera);p=world-np.asarray(pivot)
    yaw,pitch,roll=np.radians(angles);cy,sy=np.cos(yaw),np.sin(yaw);cp,sp=np.cos(pitch),np.sin(pitch);cr,sr=np.cos(roll),np.sin(roll)
    a=np.column_stack((cy*p[:,0]+sy*p[:,2],p[:,1],-sy*p[:,0]+cy*p[:,2]))
    b=np.column_stack((a[:,0],cp*a[:,1]-sp*a[:,2],sp*a[:,1]+cp*a[:,2]))
    rotated=np.column_stack((cr*b[:,0]-sr*b[:,1],sr*b[:,0]+cr*b[:,1],b[:,2]))
    world+=v[:,3,None]*(rotated-p)
    q=camera['distance']-world[:,2]
    xy=world[:,:2]*camera['focal']/q[:,None]+camera['center']
    xy[:,1]+=v[:,4]*breath
    xy[:,0]+=v[:,4]*(v[:,0]*camera['size'][0]-camera['pivot_px'][0])*breath/camera['size'][1]*.25
    return xy,q


def validate(vertices, indices, camera, pivot, bounds, face_height, minimum=.25):
    v=np.asarray(vertices);t=np.asarray(indices).reshape(-1,3)
    original=v[t,:2]*camera['size'];ab=original[:,1]-original[:,0];ac=original[:,2]-original[:,0]
    reference=ab[:,0]*ac[:,1]-ab[:,1]*ac[:,0]
    min_area=float('inf');near=float('inf');far=0.;outside=0
    for angles in product([-1.,0.,1.],repeat=3):
        for breath in [-1.,0.,1.]:
            xy,q=project(v,camera,pivot,np.asarray(angles)*bounds,breath*face_height*.006)
            tri=xy[t];ab=tri[:,1]-tri[:,0];ac=tri[:,2]-tri[:,0]
            ratio=(ab[:,0]*ac[:,1]-ab[:,1]*ac[:,0])/reference
            min_area=min(min_area,float(ratio.min()));near=min(near,float(q.min()));far=max(far,float(q.max()))
            outside+=int(((q<=camera['near'])|(q>=camera['far'])).sum())
    return {'valid':bool(np.isfinite(min_area) and min_area>minimum and outside==0),
            'configurations':81,'minimum_area_ratio':min_area,'minimum_distance':near,'maximum_distance':far,
            'clipped_vertices':outside,'bounds':np.asarray(bounds).tolist()}


def prepare(package, raw, points, classes, foreground, settings=SETTINGS):
    p=np.asarray(points,float);h,w=raw.shape;alpha=np.asarray(foreground)[:,:,3]
    if (w,h)!=(package['canvas']['width'],package['canvas']['height']) or alpha.shape!=(h,w):
        raise PreparationError('Depth, foreground and package dimensions differ.')
    dense,residual,blend,interp,calibration=calibrate(raw,p,classes,package['eyes']['crop'],settings)
    # A surface vertex just outside the matte must continue the subject's depth,
    # not import the removed background's depth into a hair-boundary triangle.
    fw=package['anatomy']['face_width']
    radius=max(1,round(fw*.025))
    seed=cv2.erode((alpha>=250).astype(np.uint8),cv2.getStructuringElement(cv2.MORPH_ELLIPSE,(radius*2+1,radius*2+1)))
    if not seed.any():raise PreparationError('No reliable interior for boundary-depth continuation.')
    transparent=seed==0
    if transparent.any():
        nearest=distance_transform_edt(transparent,return_distances=False,return_indices=True)
        dense[transparent]=dense[nearest[0][transparent],nearest[1][transparent]]
    n=settings['grid'];yy,xx=np.mgrid[0:n+1,0:n+1]
    uv=np.column_stack((xx.ravel()/n,yy.ravel()/n));xy=uv*[w,h]
    base=sample(dense,xy);facial=interp(xy);weight=sample(blend,xy)
    z=base.copy();valid=np.isfinite(facial)
    z[valid]=base[valid]*(1-weight[valid])+(facial[valid]+sample(residual,xy[valid]))*weight[valid]
    unbounded=z.copy()
    z=limit_slopes(z,[w,h],n,settings['maximum_depth_slope'])
    calibration['depth_regularization']={'maximum_slope':settings['maximum_depth_slope'],
        'adjusted_vertices':int((abs(z-unbounded)>.01).sum()),'maximum_adjustment_px':float(abs(z-unbounded).max())}
    # Existing automatically derived anatomy remains the source of motion weights.
    old=np.asarray(package['rig']['vertices']);old_n=package['rig']['grid']
    head=cv2.resize(old[:,3].reshape(old_n+1,old_n+1),(n+1,n+1),interpolation=cv2.INTER_LINEAR).ravel()
    head*=1-smooth(p[152,1]+package['anatomy']['face_height']*.35,
                   p[152,1]+package['anatomy']['face_height']*.55,xy[:,1])
    torso=cv2.resize(old[:,4].reshape(old_n+1,old_n+1),(n+1,n+1),interpolation=cv2.INTER_LINEAR).ravel()
    vertices=np.column_stack((uv,z,head,torso)).astype('<f4')
    triangles=[];fringe=[]
    for y in range(n):
        for x in range(n):
            x0,x1=round(x*w/n),min(w,round((x+1)*w/n)+1)
            y0,y1=round(y*h/n),min(h,round((y+1)*h/n)+1)
            patch=alpha[y0:y1,x0:x1]
            if patch.size==0 or patch.max()==0:continue
            a=y*(n+1)+x;b=a+1;c=a+n+1
            pair=[[a,b,c],[c,b,c+1]]
            triangles.extend(pair)
            if patch.min()<254:fringe.extend(pair)
    indices=np.asarray(triangles,dtype='<u2');edge=np.asarray(fringe,dtype='<u2')
    if len(indices)==0:raise PreparationError('No foreground geometry.')
    f=max(w,h)*settings['focal_image_fraction'];pivot_px=np.asarray(package['rig']['pivot'])
    camera={'size':[w,h],'center':[w/2,h/2],'focal':f,'distance':f,'near':f*.35,'far':f*1.65,'pivot_px':pivot_px.tolist()}
    pivot_z=float(sample(dense,pivot_px[None,:])[0]);pivot=[*((pivot_px-camera['center'])*(f-pivot_z)/f),pivot_z]
    bounds=np.asarray(settings['bounds_degrees']);attempts=[]
    for _ in range(8):
        report=validate(vertices,indices,camera,pivot,bounds,package['anatomy']['face_height'],settings['minimum_area_ratio'])
        attempts.append(report)
        if report['valid']:break
        bounds*=.85
    ready=report['valid'] and bounds[0]>=settings['minimum_yaw_degrees']
    neutral,_=project(vertices,camera,pivot,[0,0,0])
    calibration['neutral_registration_error_px']=float(np.max(np.abs(neutral-uv*[w,h])))
    metadata={'version':VERSION,'grid':n,'camera':camera,'pivot':np.asarray(pivot).tolist(),
              'bounds':{'yaw':float(bounds[0]),'pitch':float(bounds[1]),'roll':float(bounds[2])},
              'geometry':{'file':'projection.bin','vertices':len(vertices),'triangles':len(indices),'fringe_triangles':len(edge)},
              'calibration':calibration,'validation':report,'validation_attempts':attempts,
              'proposed_idle_degrees':[float(bounds[0]),package['behavior']['head']['idle_degrees'][1],package['behavior']['head']['idle_degrees'][2]],
              'manual_overrides':False}
    metadata['ready']=bool(ready)
    return metadata,vertices,indices,edge,{'calibrated-depth.npy':dense,'facial-residual.npy':residual,'face-blend.npy':blend}

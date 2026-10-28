"""Registered side-surface support for the existing projective parallax rig.

References are observations, not interchangeable portraits. The frontal image
remains canonical; generated eye/oral material is excluded from every view.
"""
from hashlib import sha256
import json
from pathlib import Path

import cv2
import numpy as np
from PIL import Image
from scipy.interpolate import LinearNDInterpolator

from .archive import encoded
from .faces import FaceDetector, PreparationError, MODEL_SHA256
from .projection_rig import limit_slopes, validate


def pose(matrix):
    m=np.asarray(matrix,float)[:3,:3]
    return np.degrees([np.arctan2(m[0,2],m[2,2]),np.arctan2(-m[1,2],np.hypot(m[1,0],m[1,1])),np.arctan2(m[1,0],m[1,1])])


def reference_foreground(image, points, models, output):
    from .motion_assets import subject
    model=models/'selfie-multiclass-256-float32-v1.tflite'
    pin=json.loads((models/'segmenter.json').read_bytes())
    if sha256(model.read_bytes()).hexdigest()!=pin['sha256']:
        raise PreparationError('Pinned segmentation model checksum mismatch.')
    directory=models/'vitmatte-small-composition-1k'
    pin=json.loads((directory/'revision.json').read_bytes())
    for name,digest in pin['files'].items():
        if not (directory/name).is_file() or sha256((directory/name).read_bytes()).hexdigest()!=digest:
            raise PreparationError('Existing pinned native hair models are required; do not download substitutes.')
    output.mkdir(parents=True,exist_ok=True)
    subject(image,points,models,output)
    return Image.open(output/'foreground.webp').convert('RGBA')


def register_reference(original, reference, source_points, source_pose, motion, oral, models, output, reference_cache, canonical_foreground, expression_regions, slot, policy):
    output.mkdir(parents=True)
    with FaceDetector(models/'face_landmarker.task') as detector:
        if sha256((models/'face_landmarker.task').read_bytes()).hexdigest()!=MODEL_SHA256:
            raise PreparationError('Face model checksum mismatch.')
        image=Image.open(reference).convert('RGB')
        measured=detector.detect(image)
    (output/'landmarks.json').write_bytes(encoded(measured.as_dict()))
    achieved=pose(measured.matrix)-source_pose
    w,h=original.size;target=measured.points[:,:2]*[w/image.width,h/image.height]
    from .reference_policy import assess_pose
    qualification=assess_pose(policy,slot,achieved,source_points,target,(w,h))
    if not qualification['accepted']:
        (output/'registration.json').write_bytes(encoded(qualification));return None,qualification
    axis=qualification['axis']
    points=np.asarray(source_points,float)
    anchors=motion['projection']['calibration']['stable_indices']
    # Canvas support pins camera/torso framing while the measured head landmarks
    # register the side material into common source coordinates.
    support=np.array([[0,0],[w/2,0],[w-1,0],[w-1,h/2],[w-1,h-1],[w/2,h-1],[0,h-1],[0,h/2]])
    a=np.vstack([points[anchors,:2],support]);b=np.vstack([target[anchors],support])
    yy,xx=np.mgrid[0:h,0:w]
    # Evaluate a coarse smooth inverse map, then retain full-resolution UVs as
    # evidence. No image-to-image dissolve or manual placement is involved.
    gy,gx=np.mgrid[0:h:complex(0,385),0:w:complex(0,289)]
    interp=LinearNDInterpolator(a,b,fill_value=np.nan)
    mapping=interp(np.stack([np.minimum(gx,w-1),np.minimum(gy,h-1)],-1))
    if not np.isfinite(mapping).all():raise PreparationError('Incomplete reference UV registration.')
    mapping=cv2.resize(mapping.astype(np.float32),(w,h),interpolation=cv2.INTER_LINEAR)
    foreground=np.asarray(reference_foreground(image,measured.points,models,reference_cache).resize((w,h),Image.Resampling.LANCZOS))
    alpha=foreground[:,:,3].astype(np.float32)/255
    premultiplied=foreground[:,:,:3].astype(np.float32)*alpha[:,:,None]
    alpha=cv2.remap(alpha,mapping[:,:,0],mapping[:,:,1],cv2.INTER_LINEAR)
    color=cv2.remap(premultiplied,mapping[:,:,0],mapping[:,:,1],cv2.INTER_LINEAR)
    color=np.rint(np.clip(color/np.maximum(alpha[:,:,None],1e-6),0,255)).astype(np.uint8)
    center=points[1,:2];fw=np.linalg.norm(points[234,:2]-points[454,:2]);fh=motion['anatomy']['face_height']
    lateral=np.clip((np.abs(xx-center[0])/max(1,fw*.5)-.65)/.25,0,1)
    vertical=np.clip((yy-(points[10,1]-fh*.3))/(fh*.12),0,1)*np.clip((points[152,1]+fh*.1-yy)/(fh*.12),0,1)
    confidence=alpha*lateral*vertical
    if axis==0:
        from .reference_policy import lateral_visibility
        confidence*=lateral_visibility(xx,center[0],fw,achieved[0])
    canonical=np.asarray(canonical_foreground)
    # A face-landmark registration does not measure a new hair silhouette.
    # Keep the original single boundary; reference color may support its
    # opaque interior, without introducing a second translucent contour.
    confidence*=np.clip((canonical[:,:,3].astype(np.float32)-250)/5,0,1)
    difference=np.max(np.abs(color.astype(float)-np.asarray(original).astype(float)),axis=2)
    consistency=np.clip((80-difference)/40,0,1)
    confidence*=np.where(canonical[:,:,3]>=250,consistency,1)
    # Protect complete existing live mouth/eye patches, including sampling
    # footprints; generated enamel/irises cannot enter active view materials.
    for crop in (oral['crop'],motion['eyes']['crop'],*expression_regions):
        pad=max(3,round(fw*.03));x0=max(0,crop['x']-pad);x1=min(w,crop['x']+crop['width']+pad)
        y0=max(0,crop['y']-pad);y1=min(h,crop['y']+crop['height']+pad);confidence[y0:y1,x0:x1]=0
    dx=np.gradient(mapping,axis=1);dy=np.gradient(mapping,axis=0)
    determinant=dx[:,:,0]*dy[:,:,1]-dx[:,:,1]*dy[:,:,0]
    unsupported=(determinant<=.1)&(confidence>.01)
    Image.fromarray(np.uint8(unsupported)*255).save(output/'unsupported-uv.png')
    # A compressed/occluded far-side region cannot supply a valid observation.
    # Exclude its sampling footprint and feather support inside the valid UV
    # domain, retaining the canonical source there rather than folding it.
    safe=np.uint8(determinant>.1)
    safe=cv2.erode(safe,np.ones((7,7),np.uint8))
    distance=cv2.distanceTransform(safe,cv2.DIST_L2,5)
    confidence*=np.clip(distance/max(2,fw*.01),0,1)
    if not np.any(confidence>.01):raise PreparationError('No valid registered side support remains.')
    if np.any(determinant[confidence>.01]<=0):raise PreparationError('Registered view folds inside active coverage.')
    rgba=np.dstack([color,np.rint(confidence*255).astype(np.uint8)])
    rgba[rgba[:,:,3]==0,:3]=0
    path=output/'registered-support.webp';Image.fromarray(rgba).save(path,lossless=True,exact=True)
    uv=output/'source-uv.npz';np.savez_compressed(uv,uv=mapping/np.array([w,h],np.float32))
    record=dict(**qualification,pose_method='relative-mediapipe-estimate',
        source_sha256=sha256(Path(reference).read_bytes()).hexdigest(),
        visibility_method='pose-facing-lateral-hemisphere/1' if axis==0 else 'observed-alpha/1',
        boundary_method='canonical-silhouette/1',reference_outline_active=False,
        registered_support_pixels=int((confidence>0).sum()),generated_eye_oral_pixels=0,
        camera=motion['projection']['camera'],uv_sha256=sha256(uv.read_bytes()).hexdigest(),
        material_sha256=sha256(path.read_bytes()).hexdigest(),model_sha256=MODEL_SHA256,
        minimum_active_uv_jacobian=float(determinant[confidence>.01].min()),
        excluded_unsupported_uv_pixels=int(unsupported.sum()))
    (output/'registration.json').write_bytes(encoded(record))
    return rgba,record


def extend_geometry(motion, directory, foreground, supports, output, maximum_yaw=8.):
    p=motion['projection'];g=p['geometry'];raw=(directory/g['file']).read_bytes()
    if sha256(raw).hexdigest()!=g['sha256']:raise PreparationError('Geometry source mismatch.')
    vertices=np.frombuffer(raw,'<f4',count=g['vertices']*5).reshape(-1,5).copy()
    original_triangles=g['triangles']
    n=p['grid'];w,h=p['camera']['size'];source_alpha=np.asarray(foreground)[:,:,3];alpha=source_alpha.copy()
    for support in supports:alpha=np.maximum(alpha,support[:,:,3])
    # Newly supported cells inherit the same head/torso attachment and depth
    # continuation as the nearest known foreground, never background weights.
    from scipy.ndimage import distance_transform_edt
    xs=np.clip(np.rint(vertices[:,0]*w).astype(int),0,w-1)
    ys=np.clip(np.rint(vertices[:,1]*h).astype(int),0,h-1)
    unknown=(source_alpha[ys,xs]<254).reshape(n+1,n+1)
    if unknown.all():raise PreparationError('No opaque source support for side-surface attachment.')
    nearest=distance_transform_edt(unknown,return_distances=False,return_indices=True)
    known_ids=(nearest[0]*(n+1)+nearest[1]).ravel();outside=unknown.ravel()
    vertices[outside,3:]=vertices[known_ids[outside],3:]
    # A larger turn needs one rigid head, including hair at the matte edge.
    # The same chin-relative C2 neck taper applies to every source; background
    # certainty must not split the head into differently moving patches.
    chin=motion['anatomy']['chin_y'];fh=motion['anatomy']['face_height']
    phase=np.clip((vertices[:,1]*h-(chin-fh*.08))/(fh*.58),0,1)
    vertices[:,3]=1-(phase**3*(phase*(phase*6-15)+10))
    triangles=[];fringe=[]
    for y in range(n):
        for x in range(n):
            patch=alpha[round(y*h/n):min(h,round((y+1)*h/n)+1),round(x*w/n):min(w,round((x+1)*w/n)+1)]
            if not patch.size or not patch.max():continue
            a=y*(n+1)+x;b=a+1;c=a+n+1;pair=[[a,b,c],[c,b,c+1]];triangles.extend(pair)
            original=source_alpha[round(y*h/n):min(h,round((y+1)*h/n)+1),round(x*w/n):min(w,round((x+1)*w/n)+1)]
            if original.min()<254:fringe.extend(pair)
    indices=np.asarray(triangles,dtype='<u2');edge=np.asarray(fringe,dtype='<u2')
    attempts=[];chosen=None
    # Strengthen depth conditioning rather than relax the original area gate.
    # This changes estimated relief, never frontal UVs or canonical image RGB.
    for slope in (3.,2.,1.5,1.):
        v=vertices.copy();v[:,2]=limit_slopes(v[:,2],p['camera']['size'],n,slope)
        selected_yaw=min(8.,maximum_yaw)
        for bounds in ([12.,6.,2.],[10.,4.,1.2],[selected_yaw,4.,1.2]):
            report=validate(v,indices,p['camera'],p['pivot'],bounds,motion['anatomy']['face_height'])
            attempts.append(dict(maximum_depth_slope=slope,**report))
            if report['valid'] and bounds[0]==selected_yaw:
                chosen=(v,bounds,report,slope);break
        if chosen:break
    (output/'geometry-attempts.json').write_bytes(encoded(attempts))
    if chosen is None:raise PreparationError('No expanded safe geometry; preserve attempts without activation.')
    v,bounds,report,slope=chosen
    binary=v.astype('<f4').tobytes()+indices.tobytes()+edge.tobytes()
    (output/'projection.bin').write_bytes(binary)
    p['geometry'].update(sha256=sha256(binary).hexdigest(),triangles=len(indices),fringe_triangles=len(edge))
    p['bounds']=dict(zip(('yaw','pitch','roll'),bounds));p['validation']=report;p['validation_attempts']=attempts
    p['calibration']['expanded_depth_conditioning']=dict(maximum_slope=slope,changed_vertices=int((abs(v[:,2]-vertices[:,2])>.01).sum()),maximum_adjustment_px=float(abs(v[:,2]-vertices[:,2]).max()),attachment_profile='rigid-head-neck/1')
    return dict(bounds=bounds,attempts=attempts,added_surface_triangles=len(indices)-original_triangles,extended_attachment_vertices=int(outside.sum()),hidden_geometry_measured=False)

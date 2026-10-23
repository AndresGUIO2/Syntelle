"""Register a joint lip/cheek/eye companion with opaque canonical dentition.

This offline raster sequence samples one validated landmark deformation. No
generated enamel, manual mask, character placement override or provider call.
"""
from hashlib import sha256
import json
from pathlib import Path
import shutil

import cv2
import numpy as np
from PIL import Image
from scipy.interpolate import LinearNDInterpolator
from scipy.spatial import Delaunay

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.detection import PreparationDetector
from app.preparation.eyes import align_eyes, EYE_RINGS
from app.preparation.faces import PreparationError

INNER = [78,191,80,81,82,13,312,311,310,415,308,324,318,402,317,14,87,178,88,95]
OUTER = [61,146,91,181,84,17,314,405,321,375,291,409,270,269,267,0,37,39,40,185]
EYES = [[33,160,158,133,153,144], [362,385,387,263,373,380]]


def digest(path): return sha256(path.read_bytes()).hexdigest()


def polygon(shape, points):
    mask=np.zeros(shape,np.uint8)
    cv2.fillPoly(mask,[np.rint(points*4).astype(np.int32)],255,shift=2)
    return mask.astype(np.float32)/255


def run():
    root=ROOT/'.tools/character-performance-34'
    reference=root/('reference/correction' if (root/'reference/correction/run.json').exists() else 'reference/proposal')
    config=json.loads((ROOT/'docs/characters/laughter-reference-proposal.json').read_text())
    record=json.loads((reference/'run.json').read_text())
    raw=reference/record['file']
    review=json.loads((reference/'visual-review.json').read_text())
    if not review['accepted'] or review['raw_sha256']!=digest(raw):
        raise PreparationError('Inspect the received laughter reference before composition.')
    out=root/('companion-corrected-v2' if reference.name=='correction' else 'companion');out.mkdir(exist_ok=True)
    source=ROOT/config['original'];landmarks=ROOT/config['cached_landmarks']
    expressions=ROOT/'apps/web/src/experiments/motion/expressions/syntelle/expressions.json'
    oral_dir=ROOT/'apps/web/src/experiments/viseme-a/characters/syntelle'
    oral=json.loads((oral_dir/'package.json').read_text())
    previous=json.loads(expressions.read_text());crop=previous['crop']
    paths=[Path(__file__),source,landmarks,raw,expressions,oral_dir/'package.json',oral_dir/'upper-teeth.webp',oral_dir/'lower-teeth.webp',oral_dir/'ei/cavity.webp']
    binding=sha256(encoded({str(p):digest(p) for p in paths})).hexdigest()
    if (out/'laughter.json').exists():
        cached=json.loads((out/'laughter.json').read_text())
        if cached['binding']!=binding:raise PreparationError('Preserve the previous composition before rebuilding.')
        print(json.dumps({'cached':True,'output':str(out)}));return
    inputs=out/'inputs';inputs.mkdir(exist_ok=True)
    for index,path in enumerate(paths):shutil.copyfile(path,inputs/f'{index:02d}-{path.name}')
    db=Archive();db.archive_tree('character-performance-34/companion-inputs',inputs)
    original=Image.open(source).convert('RGB')
    points=np.asarray(json.loads(landmarks.read_text())['landmarks_px'])[:478,:2]
    with PreparationDetector(ROOT/'.tools/models/face_landmarker.task') as detector:
        face=detector.detect(Image.open(raw))
    (out/'generated-landmarks.json').write_bytes(encoded(face.as_dict()))
    aligned,target,registration=align_eyes(original,Image.open(raw).convert('RGB'),points,face.points,original.width/Image.open(raw).width)
    target=target[:478,:2]
    aligned.save(out/'aligned-reference.png')
    x0,y0,w,h=(crop[k] for k in ('x','y','width','height'))
    shift=np.array([x0,y0]);src=points-shift;dst=target-shift
    # Original iris/pupil samples remain the eye material. Registration may
    # narrow lids, but cannot import a generated iris or synthesize hidden iris.
    dst[468:478]=src[468:478]
    center=(src[61]+src[291])/2;mw=np.linalg.norm(src[61]-src[291])
    yy,xx=np.indices((h,w),np.float32);xy=np.stack([xx,yy],-1)
    radius=((xx-center[0])/(mw*1.25))**2+((yy-(src[168,1]+src[17,1])*.5)/(mw*1.8))**2
    mask=np.clip((1-radius)/.35,0,1);mask=mask*mask*(3-2*mask)
    mask[[0,-1]]=0;mask[:,[0,-1]]=0
    # Common geometric support is automatic and shared, with fixed crop edges.
    support=np.array([[0,0],[w-1,0],[w-1,h-1],[0,h-1],[w/2,0],[w/2,h-1],[0,h/2],[w-1,h/2]])
    # Dense cheek/iris samples introduce almost-coincident triangles unrelated
    # to expression topology. Use semantic MediaPipe rings and stable anchors.
    # The closed neutral aperture has almost-coincident/crossed detector points.
    # It is a changing hole, not skin to triangulate. Only the outer lip seam
    # enters the skin mesh; its independently measured inner contour clips the
    # rigid dental/cavity composite after deformation.
    indices=sorted(set(OUTER+list(EYE_RINGS[0])+list(EYE_RINGS[1])+
        [10,338,297,332,284,251,389,356,454,323,361,288,397,365,379,378,400,377,152,148,176,149,150,136,172,58,132,93,234,127,162,21,54,103,67,109,
         168,6,197,195,5,4,2,98,327,50,280,101,330,205,425,187,411,70,63,105,66,107,336,296,334,293,300]))
    source_points=np.vstack([src[indices],support]);target_points=np.vstack([dst[indices],support])
    eye_indices=[[indices.index(i) for i in ring] for ring in EYE_RINGS]
    triangles=Delaunay(source_points).simplices
    def area(p):
        a=p[triangles[:,1]]-p[triangles[:,0]];b=p[triangles[:,2]]-p[triangles[:,0]]
        return a[:,0]*b[:,1]-a[:,1]*b[:,0]
    original_area=area(source_points)
    # Very thin unobserved triangles outside the crop must not create a false
    # gate. All facial triangles intersecting the crop remain checked.
    centers=source_points[triangles].mean(1)
    relevant=(centers[:,0]>=0)&(centers[:,0]<w)&(centers[:,1]>=0)&(centers[:,1]<h)&(abs(original_area)>.05)
    reduction=1.
    for reduction in [1.,.85,.7225,.614125,.52200625,.4437053125,.377149515625]:
        end=source_points+(target_points-source_points)*reduction
        ratios=[area(source_points+(end-source_points)*a)[relevant]/original_area[relevant] for a in np.linspace(0,1,25)]
        if min(float(r.min()) for r in ratios)>.04:break
    else:
        (out/'rejected-topology.json').write_bytes(encoded({'minimum_triangle_ratio':min(float(r.min()) for r in ratios),'triangles':triangles.tolist(),'indices':indices,'reduction':reduction}))
        raise PreparationError('Joint laughter deformation inverted; preserve evidence without installation.')
    original_crop=np.asarray(original)[y0:y0+h,x0:x0+w]
    reference_crop=np.asarray(aligned)[y0:y0+h,x0:x0+w]
    # Dental sheets remain byte-identical canonical textures, only translated.
    teeth=[np.asarray(Image.open(oral_dir/name).convert('RGBA')) for name in ['upper-teeth.webp','lower-teeth.webp']]
    oc=oral['crop'];oral_shift=np.array([oc['x']-x0,oc['y']-y0])
    ei=np.asarray(oral['poses']['EI']['points'])[:478,:2]+oral_shift
    cavity=np.asarray(Image.open(oral_dir/'ei/cavity.webp').convert('RGB'))
    files=[];checks=[]
    for index,amount in enumerate(np.linspace(0,1,25)):
        mid=source_points+(end-source_points)*amount
        map_source=LinearNDInterpolator(mid,source_points,fill_value=np.nan)(xy).astype(np.float32)
        map_target=LinearNDInterpolator(mid,target_points,fill_value=np.nan)(xy).astype(np.float32)
        map_source=np.where(np.isfinite(map_source),map_source,xy)
        map_target=np.where(np.isfinite(map_target),map_target,xy)
        a=cv2.remap(original_crop,map_source[...,0],map_source[...,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        b=cv2.remap(reference_crop,map_target[...,0],map_target[...,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        material_mask=mask.copy()
        for eye in eye_indices:
            protected=polygon((h,w),mid[eye]);protected=cv2.dilate(protected,np.ones((9,9),np.uint8))
            material_mask*=1-protected
        blend=material_mask*amount
        color=np.rint(a*(1-blend[...,None])+b*blend[...,None]).astype(np.uint8)
        inner=src[INNER]+(dst[INNER]-src[INNER])*(amount*reduction)
        aperture=polygon((h,w),inner)
        # All generated dental pixels are discarded by the observed inner-lip
        # aperture. Analytic geometry owns visibility, never a dental opacity fade.
        if amount>0:
            # Cavity is soft tissue, sampled only inside its own observed hole.
            # Reusing the rectangular EI crop directly would import EI lips at
            # the wider laugh corners and create a second dental-looking seam.
            cavity_map=LinearNDInterpolator(np.vstack([inner,inner.mean(0)]),
                np.vstack([ei[INNER]-oral_shift,(ei[INNER]-oral_shift).mean(0)]),fill_value=0)(xy).astype(np.float32)
            interior=cv2.remap(cavity,cavity_map[...,0],cavity_map[...,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REPLICATE).astype(float)
            shifts=[inner[5]-ei[13],inner[15]-ei[14]]
            for texture,delta in zip(teeth,shifts):
                material=texture.astype(np.float32);material[...,:3]*=material[...,3:4]/255
                tex=cv2.remap(material,(xx-oral_shift[0]-delta[0]).astype(np.float32),(yy-oral_shift[1]-delta[1]).astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_CONSTANT)
                alpha=tex[...,3:4].astype(float)/255
                interior=interior*(1-alpha)+tex[...,:3]
            color=np.rint(color*(1-aperture[...,None])+interior*aperture[...,None]).astype(np.uint8)
        else:color=original_crop.copy()
        # Preserve every pixel outside automatic facial support exactly.
        color=np.rint(original_crop*(1-mask[...,None])+color*mask[...,None]).astype(np.uint8)
        rgba=np.dstack([color,np.uint8(np.rint(mask*255))])
        name=f'frame-{index:02d}.webp';Image.fromarray(rgba).save(out/name,lossless=True,exact=True,method=6)
        files.append({'file':name,'sha256':digest(out/name)})
        checks.append({'amount':float(amount),'minimum_triangle_ratio':float(ratios[index].min()),'outside_support_exact':bool(np.array_equal(color[mask==0],original_crop[mask==0]))})
        if index in (0,6,12,18,24):
            preview=original.copy();preview.paste(Image.fromarray(color),(x0,y0));preview.save(out/f'preview-{index:02d}.png')
    package={'version':'registered-laughter/1','binding':binding,'source_sha256':digest(source),
             'motion_build_id':previous['motion_build_id'],'oral_build_id':previous['oral_build_id'],
             'crop':crop,'frames':files,'amplitude_reduction':reduction,'registration':registration,
             'canonical_dental_sha256':[digest(oral_dir/n) for n in ['upper-teeth.webp','lower-teeth.webp']],
             'generated_dental_installed':False,'manual_masks':False,'iris_material':'immutable-original',
             'validation':{'valid':all(c['outside_support_exact'] for c in checks),'frames':checks},
             'review_status':'composition-review-required'}
    (out/'laughter.json').write_bytes(encoded(package))
    print(json.dumps({'output':str(out),'amplitude_reduction':reduction,'archive':db.archive_tree('character-performance-34/companion',out)}))


if __name__=='__main__':run()

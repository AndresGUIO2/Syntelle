"""Prepare reusable joint expression geometry from explicit immutable inputs."""
import argparse
import json
import shutil
import platform
from pathlib import Path

import numpy as np
import cv2
from scipy.interpolate import LinearNDInterpolator
from PIL import Image

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.detection import PreparationDetector
from app.preparation.eyes import align_eyes
from app.preparation.joint_expressions import prepare_pairs,prepare_field,validate_combined
from app.workflows.motion.prepare_motion import digest


def prepare_skin_frames(kind,output,resolve,original,points,oral,package):
    crop=package['crop'];x,y,w,h=[crop[k] for k in ('x','y','width','height')]
    foreground=np.asarray(Image.open(resolve('foreground')).convert('RGBA'))
    field=np.load(output/kind/'field.npy')
    yy,xx=np.indices((h,w),np.float32)
    reference=Image.open(resolve(kind)).convert('RGB')
    generated=np.asarray(json.loads((output/kind/'landmarks.json').read_text())['landmarks_px'])
    aligned,target,_=align_eyes(original,reference,points,generated,original.width/reference.width)
    xy=np.stack([xx+x,yy+y],-1)
    material_map=LinearNDInterpolator(points[:468,:2],target[:468,:2],fill_value=np.nan)(xy)
    material_map=np.where(np.isfinite(material_map),material_map,xy).astype(np.float32)
    material=cv2.remap(np.asarray(aligned),material_map[...,0],material_map[...,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
    mouth_width=float(np.linalg.norm(points[291,:2]-points[61,:2]))
    center=(points[291,:2]+points[61,:2])/2
    ellipse=((xy[...,0]-center[0])/(mouth_width*1.05))**2+((xy[...,1]-center[1])/(mouth_width*.8))**2
    tissue=np.clip((1-ellipse)/.5,0,1)
    oc=oral['crop']
    outside=np.maximum(np.maximum(oc['x']-xy[...,0],xy[...,0]-(oc['x']+oc['width'])),np.maximum(oc['y']-xy[...,1],xy[...,1]-(oc['y']+oc['height'])))
    # Exclude the complete oral patch with a padded, soft outside transition.
    # Generated lips, teeth, cavity and tongue cannot enter the skin material.
    tissue*=np.clip((outside-4)/(mouth_width*.1),0,1)
    skin=foreground[y:y+h,x:x+w].copy()
    frames=[]
    for index,amount in enumerate(np.linspace(0,1,25)):
        sx,sy=xx.copy(),yy.copy()
        for _ in range(5):
            delta=cv2.remap(field.astype(np.float32),((sx+.5)*field.shape[1]/w-.5).astype(np.float32),((sy+.5)*field.shape[0]/h-.5).astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_CONSTANT)
            sx=xx-delta[...,0]*amount;sy=yy-delta[...,1]*amount
        rgba=cv2.remap(foreground,(sx+x).astype(np.float32),(sy+y).astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        normalized=np.rint(skin[...,:3]*(1-tissue[...,None]*amount)+material*tissue[...,None]*amount).astype(np.uint8)
        appearance=cv2.remap(normalized,sx.astype(np.float32),sy.astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101)
        rgba[...,:3]=appearance
        name=f'{kind}-skin-{index:02d}.webp'
        Image.fromarray(rgba).save(output/name,lossless=True,exact=True,method=6)
        frames.append({'file':name,'sha256':digest(output/name)})
    return frames


def run(manifest, output):
    settings=json.loads(manifest.read_text())
    resolve=lambda name: ROOT/settings[name]
    original=Image.open(resolve('original')).convert('RGB')
    points=np.asarray(json.loads(resolve('landmarks').read_text())['landmarks_px'])
    expressions=resolve('expressions')
    previous=json.loads((expressions/'expressions.json').read_text())
    oral=json.loads(resolve('oral').read_text())
    identity=digest(resolve('original'))
    if previous.get('source_sha256')!=identity or oral.get('source_sha256')!=identity:
        raise ValueError('Expression and oral packages must bind the immutable original.')
    if original.size!=(oral['canvas']['width'],oral['canvas']['height']) or Image.open(resolve('foreground')).size!=original.size:
        raise ValueError('Prepared source canvases differ.')
    if points.ndim!=2 or points.shape[0]<478 or points.shape[1]<2 or not np.isfinite(points).all():
        raise ValueError('Complete finite cached MediaPipe landmarks are required.')
    output.mkdir(parents=True,exist_ok=True)
    if (output/'inputs.json').exists():raise ValueError('Preserve existing preparation; use a new output directory.')
    (output/'inputs.json').write_bytes(manifest.read_bytes())
    inputs=output/'inputs';inputs.mkdir()
    for name in ('original','landmarks','oral','foreground','smile','laugh'):
        path=resolve(name);shutil.copyfile(path,inputs/(name+path.suffix))
    sources=output/'sources';sources.mkdir()
    for path in (Path(__file__),ROOT/'services/processor/app/preparation/joint_expressions.py',ROOT/'services/processor/app/preparation/expressions.py'):
        shutil.copyfile(path,sources/path.name)
    (output/'bindings.json').write_bytes(encoded({'inputs':{name:digest(resolve(name)) for name in ('original','landmarks','oral','foreground','smile','laugh')},'python':platform.python_version(),'mediapipe_model_sha256':digest(ROOT/'.tools/models/face_landmarker.task')}))
    joint={}
    db=Archive()
    for kind in ('smile','laugh'):
        reference=resolve(kind)
        out=output/kind;out.mkdir(exist_ok=True)
        with PreparationDetector(ROOT/'.tools/models/face_landmarker.task') as detector:
            generated=detector.detect(Image.open(reference))
        (out/'landmarks.json').write_bytes(encoded(generated.as_dict()))
        ref=Image.open(reference).convert('RGB')
        _,target,registration=align_eyes(original,ref,points,generated.points,original.width/ref.width)
        result=prepare_field(points,target,previous,Image.open(expressions/'field.png'),out)
        field=np.load(out/'field.npy')
        joint[kind]=prepare_pairs(oral,field,result['crop'],target,points)
        joint[kind]['registration']=registration
        joint[kind]['reference_sha256']=digest(reference)
        # Generated teeth and lip texture are never installed as a face overlay.
        Image.new('RGBA',(result['crop']['width'],result['crop']['height'])).save(out/'smile-material.png')
        result['smile']['closed_mouth_only']=False
        result['smile']['material']['sha256']=digest(out/'smile-material.png')
        result['field']['sha256']=digest(out/'field.png')
        result['smile']['reference_sha256']=digest(reference)
        (out/'expressions.json').write_bytes(encoded(result))
    package=json.loads((output/'smile/expressions.json').read_text())
    combined=validate_combined(oral,joint['smile'],joint['laugh'])
    package['version']='local-expressions/3'
    package['joint_articulation']={**joint['smile'],'laugh':joint['laugh'],'validation':{'valid':True,'combined':combined},'review_status':'composition-review-required'}
    package['source_sha256']=digest(resolve('original'))
    package['joint_articulation']['skin_frames']=prepare_skin_frames('smile',output,resolve,original,points,oral,package)
    package['joint_articulation']['laugh']['skin_frames']=prepare_skin_frames('laugh',output,resolve,original,points,oral,package)
    package['field']={'file':'field.png','sha256':digest(output/'smile/field.png')}
    for name in ('field.png','smile-material.png'):
        (output/name).write_bytes((output/'smile'/name).read_bytes())
    (output/'expressions.json').write_bytes(encoded(package))
    print(json.dumps({'prepared':str(output),'archive':db.archive_tree('expressive-performance-67/joint-preparation',output)}))


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args()
    run(args.manifest.resolve(),args.output.resolve())

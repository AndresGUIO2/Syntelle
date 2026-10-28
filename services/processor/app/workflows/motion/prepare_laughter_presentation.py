"""Derive a jaw-neutral laughter companion from frozen native resources only."""
import argparse,json,shutil
from hashlib import sha256
from pathlib import Path
import cv2
import numpy as np
from scipy.spatial import cKDTree
from app.preparation.faces import PreparationError
from app.preparation.reference_laughter import FACE_OVAL,mesh_maps,filled_polygon
from app.preparation.expressions import validate

def validate_reference_field(field,crop,iterations):
    # Large registered expressions need a Newton inverse, rather than the
    # small-expression fixed-point contraction assumption. Geometry gates stay.
    original=validate(field,crop,inverse_iterations=iterations)
    data=field[:,:,:2].astype(np.float32);gy,gx=np.indices(data.shape[:2],np.float32)
    source_x=gx+data[:,:,0];source_y=gy+data[:,:,1];target_x=source_x.copy();target_y=source_y.copy()
    def sample(x,y):return cv2.remap(data,x.astype(np.float32),y.astype(np.float32),cv2.INTER_LINEAR,borderMode=cv2.BORDER_CONSTANT)
    for _ in range(iterations):
        delta=sample(source_x,source_y);rx=source_x+delta[:,:,0]-target_x;ry=source_y+delta[:,:,1]-target_y
        xp,xm,yp,ym=sample(source_x+.5,source_y),sample(source_x-.5,source_y),sample(source_x,source_y+.5),sample(source_x,source_y-.5)
        a=1+xp[:,:,0]-xm[:,:,0];b=yp[:,:,0]-ym[:,:,0];c=xp[:,:,1]-xm[:,:,1];d=1+yp[:,:,1]-ym[:,:,1];det=a*d-b*c
        if np.any(det<=.2):return {**original,'valid':False,'solver':'newton','failure':'singular inverse iterate'}
        sx=(d*rx-b*ry)/det;sy=(-c*rx+a*ry)/det;scale=np.maximum(1,np.hypot(sx,sy)/32);source_x-=sx/scale;source_y-=sy/scale
    residual=float(np.hypot(source_x-gx,source_y-gy).max())
    return {**original,'valid':original['minimum_jacobian']>=.45 and original['maximum_gradient']<1.5 and residual<=.2,'solver':'newton','fixed_point_residual_px':original['maximum_inverse_residual_px'],'maximum_inverse_residual_px':residual}

def prepare(source:Path,output:Path,joint_field:Path):
    if output.exists():raise PreparationError('Preserve earlier preparation; choose a new output.')
    output.mkdir(parents=True)
    try:
        m=json.loads((source/'manifest.json').read_bytes());motion=m['packages']['motion'];reference=m['packages']['expressions']['reference_laughter'];rig=motion['mandible'];crop=reference['crop'];x0,y0,w,h=[crop[k] for k in ('x','y','width','height')]
        def image(key):
            item=m['textures'][key];return np.frombuffer((source/item['file']).read_bytes(),np.uint8).reshape(item['height'],item['width'],4).copy()
        fg=image('motion/'+motion['foreground']);height,width=fg.shape[:2]
        v=reference['validation'];a=np.asarray(v['source'],float);b=np.asarray(v['target'],float);triangles=np.asarray(v['triangles'],int);ids=v['indices'];g=motion['projection']['geometry'];raw=(source/m['geometry']['file']).read_bytes();vertices=np.frombuffer(raw[:g['vertices']*20],'<f4').reshape(-1,5).astype(float);xy=vertices[:,:2]*[width,height];z=vertices[:,2];camera=motion['projection']['camera'];center=np.asarray(camera['center']);distance=camera['distance'];focal=camera['focal'];world=np.column_stack(((xy-center)*(distance-z)[:,None]/focal,z));hinge=np.asarray(rig['hinge']);angle=-rig['maximum_angle_radians'];d=world-hinge;co,si=np.cos(angle),np.sin(angle);rotated=np.column_stack((d[:,0],co*d[:,1]-si*d[:,2],si*d[:,1]+co*d[:,2]))+hinge;world+=(rotated-world)*np.asarray(rig['weights'])[:,None];projected=world[:,:2]*focal/(distance-world[:,2,None])+center
        tree=cKDTree(xy);nearest=tree.query(a)[1];jaw=(projected-xy)[nearest];jaw[len(ids):]=0
        chin=ids.index(152);den=float(jaw[chin]@jaw[chin]);
        if den<1e-6:raise PreparationError('Qualified mandibular mode has no chin travel.')
        coefficient=max(0.,float((b[chin]-a[chin])@jaw[chin]/den));neutral=b-coefficient*jaw
        # Oral displacement owns its existing separately calibrated aperture.
        from app.preparation.mesh import OUTER
        for i,identifier in enumerate(ids):
            if identifier in OUTER:neutral[i]=b[i]
        def areas(points):
            p=points[triangles];ab=p[:,1]-p[:,0];ac=p[:,2]-p[:,0];return ab[:,0]*ac[:,1]-ab[:,1]*ac[:,0]
        reference_area=areas(a);minimum=min(float(np.min(areas(a*(1-q)+neutral*q)/reference_area)) for q in np.linspace(0,1,25))
        if minimum<=0:raise PreparationError('Jaw-neutral reference folds the source triangulation.')
        yy,xx=np.indices((h,w),np.float32);grid=np.stack((xx+x0,yy+y0),-1)
        fx0,fy0=np.maximum(0,np.floor(a.min(0)-2)).astype(int);fx1,fy1=np.minimum([width,height],np.ceil(a.max(0)+3)).astype(int)
        field_crop={'x':int(fx0),'y':int(fy0),'width':int(fx1-fx0),'height':int(fy1-fy0)}
        fyy,fxx=np.indices((fy1-fy0,fx1-fx0),np.float32);field_grid=np.stack((fxx+fx0,fyy+fy0),-1)
        _,forward=mesh_maps(a,a,neutral,triangles,field_grid)
        oval_a=np.asarray([a[ids.index(i)] for i in FACE_OVAL]);oval_b=np.asarray([b[ids.index(i)] for i in FACE_OVAL]);boundary=np.maximum(filled_polygon((width,height),oval_a),filled_polygon((width,height),oval_b));fh=motion['anatomy']['face_height'];fw=np.linalg.norm(a[ids.index(234)]-a[ids.index(454)]);jaw_boundary=np.vstack([oval_a,oval_b]);jaw_boundary=jaw_boundary[jaw_boundary[:,1]>a[chin,1]-fh*.2];neck=np.asarray([[a[chin,0]-fw*.28,max(a[chin,1],b[chin,1])+fh*.18],[a[chin,0]+fw*.28,max(a[chin,1],b[chin,1])+fh*.18]]);cv2.fillConvexPoly(boundary,cv2.convexHull(np.rint(np.vstack([jaw_boundary,neck])).astype(np.int32)),255)
        mask=np.clip(cv2.distanceTransform(boundary,cv2.DIST_L2,5)/max(2,fw*.035),0,1);mask=mask*mask*(3-2*mask);active=mask[y0:y0+h,x0:x0+w,None]
        delta=(forward-field_grid)
        # Select only a common ownership feather; never attenuate expression.
        oc=m['packages']['oral']['crop'];outside=np.maximum.reduce([oc['x']-field_grid[:,:,0],field_grid[:,:,0]-(oc['x']+oc['width']),oc['y']-field_grid[:,:,1],field_grid[:,:,1]-(oc['y']+oc['height']),np.zeros(field_grid.shape[:2])])
        raw_delta=delta.copy();validations=[]
        for feather in (.15,.2,.25,.3,.35,.4,.5):
            ownership=np.clip(outside/max(1,oc['width']*feather),0,1);ownership=ownership*ownership*(3-2*ownership);delta=raw_delta*ownership[:,:,None]
            quant=np.rint(delta*256).astype(np.int32)
            if np.abs(quant).max()>32767:raise PreparationError('Reference displacement exceeds lossless field encoding.')
            encoded=np.zeros((*field_grid.shape[:2],4),np.uint8)
            for axis in range(2):
                value=(quant[:,:,axis]+32768).astype(np.uint16);encoded[:,:,axis*2]=value&255;encoded[:,:,axis*2+1]=value>>8
            decoded=quant.astype(float)/256;field=np.dstack((decoded,np.zeros(field_grid.shape[:2])))
            np.savez_compressed(output/'field-diagnostic.npz',delta=delta,source=a,target=neutral,triangles=triangles)
            for iterations in (5,8,12,16):
                validation=validate_reference_field(field,field_crop,iterations);validations.append({'feather':feather,'iterations':iterations,**validation})
                if validation['valid'] or validation['minimum_jacobian']<.45 or validation['maximum_gradient']>=1.5:break
            if validation['valid']:break
        (output/'field-validation-attempts.json').write_text(json.dumps(validations,indent=2))
        if not validation['valid']:raise PreparationError('Jaw-neutral field fails existing Jacobian/inverse gates.')
        shutil.copytree(source,output/'package');package=output/'package'
        def texture(key,rgba):
            data=np.ascontiguousarray(rgba,np.uint8).tobytes();digest=sha256(data).hexdigest();file='data/'+digest+'.rgba';(package/file).write_bytes(data);m['textures'][key]={'file':file,'sha256':digest,'width':rgba.shape[1],'height':rgba.shape[0],'source':'cached-source-laughter-presentation/2'};return {'file':key.removeprefix('expressions/'),'sha256':digest}
        joint_hash=sha256(joint_field.read_bytes()).hexdigest()
        ex=m['packages']['expressions']
        if joint_hash!=ex['joint_articulation']['skin_appearance']['field_sha256']:raise PreparationError('Joint smile field differs from cached skin geometry.')
        joint=np.load(joint_field);ec=ex['crop']
        jx=((field_grid[:,:,0]-ec['x']+.5)*joint.shape[1]/ec['width']-.5).astype(np.float32)
        jy=((field_grid[:,:,1]-ec['y']+.5)*joint.shape[0]/ec['height']-.5).astype(np.float32)
        sampled=cv2.remap(joint[:,:,:2],jx,jy,cv2.INTER_LINEAR,borderMode=cv2.BORDER_CONSTANT)
        packed=np.rint(sampled*256).astype(np.int32)+32768
        joint_encoded=np.zeros_like(encoded)
        for axis in range(2):joint_encoded[:,:,axis*2]=packed[:,:,axis]&255;joint_encoded[:,:,axis*2+1]=packed[:,:,axis]>>8
        field_record=texture('expressions/laughter-field.rgba',np.concatenate((encoded,joint_encoded),axis=0))
        shutil.copy2(joint_field,output/'joint-field.npy')
        full=fg[:,:,:3].copy();full[y0:y0+h,x0:x0+w]=image('expressions/'+reference['frames'][24]['file'])[:,:,:3]
        for index,q in enumerate(np.linspace(0,1,25)):
            vertices=a*(1-q)+neutral*q;source_map,target_map=mesh_maps(vertices,a,b,triangles,grid);original=cv2.remap(fg[:,:,:3],source_map[:,:,0],source_map[:,:,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101);target=cv2.remap(full,target_map[:,:,0],target_map[:,:,1],cv2.INTER_LINEAR,borderMode=cv2.BORDER_REFLECT_101);rgb=original*(1-q)+target*q;rgba=fg[y0:y0+h,x0:x0+w].copy();rgba[:,:,:3]=np.rint(rgba[:,:,:3]*(1-active)+rgb*active).astype(np.uint8)
            if index==0:rgba=fg[y0:y0+h,x0:x0+w].copy()
            reference['frames'][index]=texture('expressions/'+reference['frames'][index]['file'],rgba)
        companion={'version':'laughter-presentation/2','source_sha256':motion['original_sha256'],'reference_sha256':reference['reference_sha256'],'geometry_sha256':g['sha256'],'mandible_sha256':sha256(json.dumps(rig,sort_keys=True,separators=(',',':')).encode()).hexdigest(),'field':field_record,'crop':field_crop,'units_per_byte':1/256,'inverse_iterations':iterations,'oral_ownership_feather':feather,'joint_field_sha256':joint_hash,'field_layers':2,'jaw_fit':{'coefficient':coefficient,'indices':[identifier for identifier in FACE_OVAL if a[ids.index(identifier),1]>a[chin,1]-fh*.2],'method':'cached-chin-projection-with-bilateral-oval-checks','source_chin_delta':(b[chin]-a[chin]).tolist(),'qualified_chin_delta':jaw[chin].tolist()},'validation':{'valid':True,'minimum_triangle_ratio':minimum,'gross_chin_residual_px':abs(float(neutral[chin,1]-a[chin,1])),'expression_amplitude':1,**validation},'manual_masks':False,'mandible_binding':rig,'frames_sha256':[frame['sha256'] for frame in reference['frames']]}
        reference['presentation']=companion
        (output/'companion.json').write_text(json.dumps(companion,indent=2));(output/'correspondences.json').write_text(json.dumps({'source':a.tolist(),'original_target':b.tolist(),'jaw_neutral_target':neutral.tolist(),'triangles':triangles.tolist()}))
        m['identity_files'].append({'path':'laughter-presentation/2','sha256':sha256(json.dumps(companion,sort_keys=True).encode()).hexdigest(),'bytes':len(json.dumps(companion))});m['package_hash']=sha256(json.dumps(m['identity_files'],separators=(',',':'),ensure_ascii=False).encode()).hexdigest();(package/'manifest.json').write_text(json.dumps(m,indent=2))
        report={'version':'laughter-presentation-preparation/2','package_hash':m['package_hash'],'provider_requests':0,'validation':companion['validation']};(output/'report.json').write_text(json.dumps(report,indent=2));return report
    except Exception as error:
        (output/'failure.json').write_text(json.dumps({'error':str(error),'provider_requests':0}));raise

def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--joint-field',type=Path,required=True);parser.add_argument('--source',type=Path,required=True);parser.add_argument('--output',type=Path,required=True);args=parser.parse_args();print(json.dumps(prepare(args.source,args.output,args.joint_field)))
if __name__=='__main__':main()

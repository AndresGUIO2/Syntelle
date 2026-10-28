"""Prepare an optional cached-source mandibular companion; never call providers."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
from itertools import product
from app.core.paths import ROOT
import numpy as np
from app.preparation.faces import PreparationError


def smooth(a,b,x):
    t=np.clip((x-a)/np.maximum(1e-8,b-a),0,1);return t*t*(3-2*t)


def mandibular_support(xy,points,oral_top):
    """Limit the articulation to the measured lower face and adjoining skin.

    The cached face oval supplies the local under-jaw boundary. A rectangular
    falloff from the chin used to rotate the visible neck as a second jaw.
    No neck joint is animated by this channel.
    """
    chin=points[152,:2];fh=chin[1]-points[10,1]
    fw=np.linalg.norm(points[234,:2]-points[454,:2])
    lower_oval=points[[234,93,132,58,172,136,150,149,176,148,152,377,400,378,379,365,397,288,361,323,454],:2]
    lower_oval=lower_oval[np.argsort(lower_oval[:,0])]
    x,y=xy[:,0],xy[:,1]
    boundary=np.interp(x,lower_oval[:,0],lower_oval[:,1])
    return ((1-smooth(fw*.42,fw*.68,np.abs(x-chin[0])))
            *smooth(oral_top-fh*.06,chin[1]-fh*.1,y)
            *(1-smooth(boundary+fh*.01,boundary+fh*.18,y)))


def prepare(source,landmarks,output):
    if output.exists():raise PreparationError('Preserve earlier candidates; use a fresh output.')
    m=json.loads((source/'manifest.json').read_bytes());motion=m['packages']['motion'];oral=m['packages']['oral'];ref=m['packages']['expressions']['reference_laughter']
    points=np.asarray(json.loads(landmarks.read_bytes())['landmarks_px'],float)
    if points.shape!=(478,3) or not np.isfinite(points).all():raise PreparationError('Complete cached landmarks required.')
    if not np.allclose([points[152,1],points[152,1]-points[10,1]],[motion['anatomy']['chin_y'],motion['anatomy']['face_height']],atol=.001):raise PreparationError('Cached anatomy belongs to another source.')
    g=motion['projection']['geometry'];raw=(source/m['geometry']['file']).read_bytes();vertices=np.frombuffer(raw[:g['vertices']*20],'<f4').reshape(-1,5).astype(float)
    indices=np.frombuffer(raw[g['vertices']*20:g['vertices']*20+g['triangles']*6],'<u2').reshape(-1,3)
    w,h=motion['canvas']['width'],motion['canvas']['height'];xy=vertices[:,:2]*[w,h];z=vertices[:,2];cam=motion['projection']['camera'];center=np.asarray(cam['center']);f=cam['focal'];distance=cam['distance'];fh=motion['anatomy']['face_height'];fw=np.linalg.norm(points[234,:2]-points[454,:2]);chin=points[152,:2]
    hp=(points[234,:2]+points[454,:2])*.5;hz=min(z[np.argmin(np.sum((xy-hp)**2,axis=1))],z[np.argmin(np.sum((xy-chin)**2,axis=1))])-fw*.75
    hinge=np.array([*((hp-center)*(distance-hz)/f),hz])
    world=np.column_stack(((xy-center)*(distance-z)[:,None]/f,z))
    def projected(opening,angle,head=(0,0,0)):
        theta=-angle*opening;d=world-hinge;co,si=np.cos(theta),np.sin(theta)
        rotated=np.column_stack((d[:,0],co*d[:,1]-si*d[:,2],si*d[:,1]+co*d[:,2]))+hinge
        deformed=world+(rotated-world)*weights[:,None]
        yaw,pitch,roll=np.deg2rad(head);cy,sy,cp,sp,cr,sr=np.cos(yaw),np.sin(yaw),np.cos(pitch),np.sin(pitch),np.cos(roll),np.sin(roll)
        pivot=np.asarray(motion['projection']['pivot']);d=deformed-pivot;ax,az=cy*d[:,0]+sy*d[:,2],-sy*d[:,0]+cy*d[:,2];by,bz=cp*d[:,1]-sp*az,sp*d[:,1]+cp*az
        deformed+= (np.column_stack((cr*ax-sr*by,sr*ax+cr*by,bz))-d)*vertices[:,3,None]
        return deformed[:,:2]*f/(distance-deformed[:,2,None])+center
    crop=oral['crop'];left,top,right,bottom=crop['x'],crop['y'],crop['x']+crop['width'],crop['y']+crop['height']
    x,y=xy[:,0],xy[:,1]
    weights=mandibular_support(xy,points,top)
    # Oral geometry already includes its calibrated gross displacement. Protect
    # that frame and apply only its separately computed residual at rendering.
    lip_points=np.asarray([v for pair in oral['pairs'].values() for phase in ('rest','open') for v in pair['mesh'][phase][:40]] + [v for contour in ref['oral']['outer'] for v in contour])
    lip_min=lip_points.min(axis=0)+[left,top];lip_max=lip_points.max(axis=0)+[left,top]
    outside=np.maximum.reduce([lip_min[0]-x,x-lip_max[0],lip_min[1]-y,y-lip_max[1],np.zeros(len(x))])
    weights*=smooth(0,fw*.12,outside)
    nearest=int(np.argmin(np.sum((xy-chin)**2,axis=1)))
    angle=.01
    while angle<.3 and projected(1,angle)[nearest,1]-xy[nearest,1]<fh*.02:angle+=.001
    if angle>=.3:raise PreparationError('No useful estimated jaw hinge.')
    # Keep the original projection geometry and validate its area under the
    # optional deformation together with accepted head and breathing bounds.
    baseline=xy[indices];ab,ac=baseline[:,1]-baseline[:,0],baseline[:,2]-baseline[:,0];area0=ab[:,0]*ac[:,1]-ab[:,1]*ac[:,0]
    attempts=[]
    for _ in range(16):
        minimum=1.
        for opening in [0,.25,.5,.75,1]:
            bounds=motion['projection']['bounds']
            for direction in product([-1,0,1],repeat=3):
                head=np.asarray(direction)*[bounds['yaw'],bounds['pitch'],bounds['roll']]
                projected_xy=projected(opening,angle,head)
                for breath in (-1,0,1):
                    shifted=projected_xy.copy();shifted[:,1]+=vertices[:,4]*breath*fh*.006;shifted[:,0]+=vertices[:,4]*(xy[:,0]-cam['pivot_px'][0])*breath*fh*.006/h*.25
                    p=shifted[indices];ab,ac=p[:,1]-p[:,0],p[:,2]-p[:,0];area=ab[:,0]*ac[:,1]-ab[:,1]*ac[:,0];minimum=min(minimum,float(np.min(area/area0)))
        attempts.append({'angle':angle,'minimum_area_ratio':minimum})
        if minimum>.25:break
        angle*=.9
    if minimum<=.25:
        output.parent.mkdir(parents=True,exist_ok=True);(output.parent/(output.name+'-rejected.json')).write_text(json.dumps(attempts))
        raise PreparationError('Mandible fails existing minimum area gate.')
    travel=float((projected(1,angle)[nearest,1]-xy[nearest,1])*720/h)
    if travel<3:raise PreparationError('Jaw motion is not visible at the standard presentation.')
    gap={}
    for pose in oral['poses']:
        mesh=oral['pairs']['Rest-'+pose]['mesh'] if pose!='Rest' else oral['pairs']['Rest-A']['mesh']
        pts=np.asarray(mesh['open'] if pose!='Rest' else mesh['rest']);gap[pose]=max(0.,float(pts[35,1]-pts[25,1]))
    origin=gap['Rest'];maximum=max(gap.values())-origin
    targets={pose:float(np.clip((value-origin)/max(1,maximum),0,1)) for pose,value in gap.items()}
    # Contacts constrain lip closure independently of gross jaw coarticulation.
    targets['MBP']=0.;targets['FV']=min(.2,targets['FV'])
    mouth=np.array([(left+right)/2,(top+bottom)/2]);mi=int(np.argmin(np.sum((xy-mouth)**2,axis=1)))
    # The oral region is protected in the skin grid; its attachment travel is
    # projected from the same unweighted estimated hinge.
    d=world[mi]-hinge;co,si=np.cos(-angle),np.sin(-angle);mw=hinge+np.array([d[0],co*d[1]-si*d[2],si*d[1]+co*d[2]])
    oral_travel=float((mw[1]*f/(distance-mw[2])+center[1])-xy[mi,1])
    # Per-column opaque enamel clearance; the stop is relative to the upper row.
    upper,lower=[np.frombuffer((source/m['textures']['oral/'+row+'-teeth.webp']['file']).read_bytes(),np.uint8).reshape(crop['height'],crop['width'],4)[:,:,3] for row in ('upper','lower')]
    stops=[]
    for column in range(crop['width']):
        uy=np.flatnonzero(upper[:,column]>127);ly=np.flatnonzero(lower[:,column]>127)
        if len(uy) and len(ly):stops.append(float(uy.max()+1-ly.min()))
    if not stops:raise PreparationError('Opposing enamel clearance unavailable.')
    corners=np.asarray([np.asarray(pair['mesh'][phase])[[0,10]] for pair in oral['pairs'].values() for phase in ('rest','open')]);corner_travel=float(np.max(np.linalg.norm(corners-corners[0],axis=2)))
    rig={'version':'mandibular-rig/2','source_sha256':motion['original_sha256'],'motion_build_id':motion['build_id'],'oral_build_id':oral['build_id'],'geometry_sha256':g['sha256'],'hinge':hinge.tolist(),'chin':chin.tolist(),'maximum_angle_radians':angle,'oral_travel_px':max(0,oral_travel),'corner_travel_px':corner_travel,'reference_gap_px':ref['oral']['inner'][24][15][1]-ref['oral']['inner'][24][5][1],'weights':weights.tolist(),'pose_jaw':targets,'pose_gap':gap,'lower_stop_px':max(stops),'validation':{'valid':True,'minimum_area_ratio':minimum,'chin_travel_720_px':travel},'provenance':{'landmarks_sha256':sha256(landmarks.read_bytes()).hexdigest(),'estimated_hinge':True,'chin_fraction':.02,'hinge_depth_prior_face_width':.75,'protected_region':'union-of-observed-lip-contours','support':'cached-mandibular-oval/1','maximum_submental_fraction':.18,'joint_configurations':405,'manual_masks':False}}
    output.mkdir(parents=True);shutil.copytree(source,output/'package')
    data=json.dumps(rig,separators=(',',':')).encode();(output/'mandibular-rig.json').write_bytes(data);motion['mandible']=rig
    m['identity_files'].append({'path':(output/'mandibular-rig.json').resolve().relative_to(ROOT).as_posix(),'bytes':len(data),'sha256':sha256(data).hexdigest()})
    m['package_hash']=sha256(json.dumps(m['identity_files'],separators=(',',':'),ensure_ascii=False).encode()).hexdigest()
    (output/'package/manifest.json').write_text(json.dumps(m,indent=2),encoding='utf-8')
    report={'version':'mandibular-preparation/1','package_hash':m['package_hash'],'validation':rig['validation'],'pose_jaw':targets,'validation_attempts':attempts,'oral_travel_px':oral_travel,'provider_requests':0}
    (output/'report.json').write_text(json.dumps(report,indent=2),encoding='utf-8');return report


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--source',type=Path,required=True);p.add_argument('--landmarks',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();print(json.dumps(prepare(a.source,a.landmarks,a.output)))

if __name__=='__main__':main()

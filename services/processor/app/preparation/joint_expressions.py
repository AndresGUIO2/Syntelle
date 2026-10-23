"""Source-bound expression geometry with independently sampled rigid dentition."""
import cv2
import numpy as np
from PIL import Image
from scipy.interpolate import LinearNDInterpolator

from .faces import PreparationError
from .expressions import field_from_image, validate

VERSION = 'joint-expression-articulation/1'


def validate_combined(oral,smile,laugh):
    reports=[]
    def area(vertices,t):
        a=vertices[t[:,1]]-vertices[t[:,0]];b=vertices[t[:,2]]-vertices[t[:,0]]
        return a[:,0]*b[:,1]-a[:,1]*b[:,0]
    for name,entry in smile['pairs'].items():
        mesh=oral['pairs'][name]['mesh']
        a,b=np.asarray(mesh['rest']),np.asarray(mesh['open'])
        sa,sb=np.asarray(entry['rest_delta']),np.asarray(entry['open_delta'])
        la,lb=np.asarray(laugh['pairs'][name]['rest_delta']),np.asarray(laugh['pairs'][name]['open_delta'])
        t=np.asarray(entry['triangles']).copy()
        def valid(candidate,positive=False):
            for u in (0,.25,.5,.75,1):
                base=a*(1-u)+b*u
                source_area=area(base,candidate)
                if positive and np.any(source_area<=0):return False
                sign=np.ones(len(candidate)) if positive else np.sign(source_area)
                for blend in (0,.25,.5,.75,1):
                    delta=(sa*(1-u)+sb*u)*(1-blend)+(la*(1-u)+lb*u)*blend
                    for amount in (0,.25,.5,.75,1):
                        if np.any(area(base+delta*amount,candidate)*sign<=0):return False
            return True
        start=len(t)-18-40
        for i in range(20):
            j=(i+1)%20
            choices=([[i,j,20+i],[j,20+j,20+i]],[[i,j,20+j],[i,20+j,20+i]])
            for choice in choices:
                choice=np.asarray(choice)
                if valid(choice,True):t[start+i*2:start+i*2+2]=choice;break
            else:raise PreparationError('No joint smile/laughter lip diagonal passes the common envelope.')
        if not valid(t):raise PreparationError('Combined smile/laughter geometry inverts.')
        entry['triangles']=t.tolist();laugh['pairs'][name]['triangles']=t.tolist()
        reports.append({'pair':name,'configurations':125,'valid':True})
    return reports


def prepare_field(points, target, previous, previous_image, output):
    """Regularize landmark noise before measuring the decoded displacement."""
    crop=previous['crop']
    field=field_from_image(previous_image,previous['units_per_byte'])
    h,w=field.shape[:2]
    yy,xx=np.indices((h,w),float)
    xy=np.stack([crop['x']+(xx+.5)*crop['width']/w,crop['y']+(yy+.5)*crop['height']/h],-1)
    width=np.linalg.norm(points[61,:2]-points[291,:2])
    center=(points[61,:2]+points[291,:2])/2
    distance=((xy[...,0]-center[0])/(width*1.05))**2+((xy[...,1]-center[1])/(width*.8))**2
    mask=np.clip(1-distance,0,1)**2
    mask*=np.clip((xy[...,1]-points[2,1])/(width*.2),0,1)
    target_width=np.linalg.norm(target[291,:2]-target[61,:2])
    growth=float(np.clip(target_width/width-1,-.1,.25))
    lift=float(np.mean(target[[61,291],1]-points[[61,291],1])-np.mean(target[[0,17],1]-points[[0,17],1]))
    local=(xy[...,0]-center[0])/(width/2)
    corner=np.clip((np.abs(local)-.35)/.65,0,1)
    corner=corner*corner*(3-2*corner)
    field[...,0]=(xy[...,0]-center[0])*growth*mask
    field[...,1]=lift*corner*mask
    field[[0,-1],:,:2]=0;field[:,[0,-1],:2]=0
    for reduction in (1.,.85,.7225,.614125,.52200625,.4437053125,.377149515625,.32057708828125,.2724905250390625):
        trial=field.copy();trial[...,:2]*=reduction
        # Smile geometry is consumed offline as float32 and baked into skin
        # frames. Preserve the existing byte-exact brow field for runtime.
        units=previous['units_per_byte']
        packed=np.asarray(previous_image.convert('RGB'))
        decoded=trial.astype(np.float32)
        report=validate(decoded,crop)
        if report['valid']:break
    else:raise PreparationError('Regularized expression failed quantized deformation checks.')
    report['geometry_encoding']='float32-preparation'
    Image.fromarray(packed).save(output/'field.png')
    np.save(output/'field.npy',decoded)
    Image.new('RGBA',(crop['width'],crop['height'])).save(output/'smile-material.png')
    return {**previous,'version':'local-expressions/3','units_per_byte':units,'validation':report,
            'smile':{'version':VERSION,'closed_mouth_only':False,'amplitude_reduction':reduction,
                     'material':{'file':'smile-material.png'},'reference_landmarks':target.tolist()}}


def prepare_pairs(oral, field, crop, target_points, source_points):
    """All coordinates derive from landmarks and the supplied common oral rig.

    Inner lips share one vertical curve, preserving opposing contact. Dental
    rows receive only a common rigid translation; RGB/alpha materials are not
    resampled by the expression deformation.
    """
    origin = np.array([oral['crop']['x'], oral['crop']['y']], float)
    source_width = np.linalg.norm(source_points[291, :2]-source_points[61, :2])
    target_width = np.linalg.norm(target_points[291, :2]-target_points[61, :2])
    growth = np.clip(target_width/source_width-1, -.1, .25)
    center_delta = np.array([0.,0.])
    # A common reduction is selected from the actual quantized mesh geometry.
    rest_opening=max(0.,float(target_points[14,1]-target_points[13,1])-float(source_points[14,1]-source_points[13,1]))
    rest_opening=min(rest_opening,source_width*.35)
    def displacement(vertices,pose):
        xy = vertices+origin
        fx = ((xy[:,0]-crop['x']+.5)*field.shape[1]/crop['width']-.5).astype(np.float32)
        fy = ((xy[:,1]-crop['y']+.5)*field.shape[0]/crop['height']-.5).astype(np.float32)
        delta = cv2.remap(field[:,:,:2].astype(np.float32), fx[None], fy[None], cv2.INTER_LINEAR, borderMode=cv2.BORDER_CONSTANT)[0]
        margin=np.minimum.reduce([vertices[:,0]/(oral['crop']['width']*.16),
            (oral['crop']['width']-vertices[:,0])/(oral['crop']['width']*.16),
            vertices[:,1]/(oral['crop']['height']*.16),
            (oral['crop']['height']-vertices[:,1])/(oral['crop']['height']*.16)])
        margin=np.clip(margin,0,1);delta*= (margin*margin*(3-2*margin))[:,None]
        if pose=='Rest':
            for ring in (0,20):
                for index in range(20):
                    sign=-1 if index<=10 else 1
                    phase=index/10 if index<=10 else (20-index)/10
                    delta[ring+index,1]+=sign*rest_opening*.5*np.sin(np.pi*phase)
        # Shared patch perimeter is stationary; it joins the original portrait.
        boundary = (vertices[:,0]<1)|(vertices[:,1]<1)|(vertices[:,0]>oral['crop']['width']-1)|(vertices[:,1]>oral['crop']['height']-1)
        delta[boundary] = 0
        return delta
    pairs = {}
    for name, pair in oral['pairs'].items():
        if not pair.get('safe'):
            continue
        mesh = pair['mesh']
        a, b = np.asarray(mesh['rest'],float), np.asarray(mesh['open'],float)
        left,right=name.split('-')
        pairs[name] = (a,b,displacement(a,left),displacement(b,right),np.asarray(mesh['triangles']).copy())
    def area(v,t):
        a,b = v[t[:,1]]-v[t[:,0]],v[t[:,2]]-v[t[:,0]]
        return a[:,0]*b[:,1]-a[:,1]*b[:,0]
    report = []
    for reduction in (1.,.85,.7225,.614125,.52200625,.4437053125):
        report=[]
        for name,(a,b,da,db,t) in pairs.items():
            cavity_start=len(t)-18
            # The original triangulation was selected for neutral articulation.
            # Select each lip-quad diagonal against the joint expression domain.
            # This changes connectivity, never source points or material RGB.
            start=cavity_start-40
            for i in range(20):
                j=(i+1)%20
                candidates=([[i,j,20+i],[j,20+j,20+i]],[[i,j,20+j],[i,20+j,20+i]])
                for candidate in candidates:
                    candidate=np.asarray(candidate)
                    if all((area(a*(1-u)+b*u+(da*(1-u)+db*u)*v*reduction,candidate)>0).all()
                           for u in (0,.25,.5,.75,1) for v in (0,.25,.5,.75,1)):
                        t[start+i*2:start+i*2+2]=candidate
                        break
            minimum=1.
            for articulation in (0,.5,1):
                base=a*(1-articulation)+b*articulation
                denominator=area(base,t)
                relevant=np.abs(denominator)>.05
                for smile in (0,.25,.5,.75,1):
                    changed=base+(da*(1-articulation)+db*articulation)*smile*reduction
                    ratios=area(changed,t)[relevant]/denominator[relevant]
                    minimum=min(minimum,float(ratios.min()))
            report.append({'pair':name,'minimum_triangle_ratio':minimum})
        if min(r['minimum_triangle_ratio'] for r in report)>.2:
            break
    else:
        worst=min(report,key=lambda row:row['minimum_triangle_ratio'])
        raise PreparationError(f'Joint expression geometry inverted; no package may be installed: {worst}.')
    return {'version':VERSION,'amplitude_reduction':reduction,
            'dental_translation':(center_delta*reduction).tolist(),
            'rest_opening':rest_opening*reduction,'rest_lower_translation':[0.,rest_opening*.5*reduction],
            'pairs':{name:{'rest_delta':(da*reduction).tolist(),'open_delta':(db*reduction).tolist(),'triangles':t.tolist()} for name,(_,_,da,db,t) in pairs.items()},
            'validation':{'valid':True,'pairs':report},'manual_masks':False,'generated_dental_installed':False}

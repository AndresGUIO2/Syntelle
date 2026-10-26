"""Joint lip controls: move lip geometry while canonical teeth remain rigid."""
from itertools import product

import numpy as np

from app.preparation.faces import PreparationError


def prepare_emotion_articulation(points, oral, expressions, states, diagnostics=None):
    points=np.asarray(points,float)[:,:2]
    origin=np.array([oral['crop']['x'],oral['crop']['y']])
    center=(points[61]+points[291])*.5-origin
    width=float(np.linalg.norm(points[61]-points[291]))
    # Downward corners, horizontal tension. These are common action settings,
    # not character-specific placements; MediaPipe supplies their proportions.
    actions={'sad':(1.,-.15),'angry':(.55,-.35),'fearful':(.1,1.7),'surprised':(0.,.2),'disgusted':(.12,-.10)}
    joint=expressions['joint_articulation']['pairs']
    def area(v,t):
        a=v[t[:,1]]-v[t[:,0]];b=v[t[:,2]]-v[t[:,0]]
        return a[:,0]*b[:,1]-a[:,1]*b[:,0]
    def displacement(vertices,action,state):
        delta=np.zeros_like(vertices)
        # A continuous piecewise affine shear keeps adjacent lip vertices
        # attached, including the very thin triangles at closed mouth corners.
        x=vertices[:40,0]-center[0]
        extent=np.maximum(abs(x)-width*.15,0.)
        delta[:40,1]=extent*.26*action[0]
        delta[:40,0]=np.sign(x)*extent*.10*action[1]
        if state == 'sad':
            # Keep the centre/medial lip still; concentrate a modest extra
            # down-turn at the actual corners instead of bowing the whole lip.
            delta[:40,1]=np.maximum(abs(x)-width*.28,0.)*.47
        if state == 'disgusted':
            # A local outer-lip curl can pass triangle gates while stretching
            # vermilion against stationary skin. Preserve the canonical lip
            # shape; nasal/lid actions and prepared resting articulation carry
            # this expression without an independent mouth deformation.
            delta[:40]=0
        return delta
    topology={name:np.asarray(joint[name]['triangles']) for name,pair in oral['pairs'].items() if pair.get('safe')}
    result={};reports={}
    for state in states:
        if state not in actions:continue
        pairs={}
        for name,pair in oral['pairs'].items():
            if not pair.get('safe'):continue
            mesh=pair['mesh'];a=np.asarray(mesh['rest'],np.float32).astype(float);b=np.asarray(mesh['open'],np.float32).astype(float)
            pairs[name]=(a,b,displacement(a,actions[state],state),displacement(b,actions[state],state))
        for reduction in (1.,.85,.7225,.614125,.52200625,.4437053125,.377149515625,.32057708828125,.2724905250390625,.2316169462832031,.1968744043407226,.1673432436896142,.1422417571361721,.1209054935657463):
            minimum=1.;worst=None
            for name,(a,b,da,db) in pairs.items():
                j=joint[name];triangles=topology[name];ja=np.asarray(j['rest_delta']);jb=np.asarray(j['open_delta'])
                for u,smile,emotion in product((0.,.25,.5,.75,1.),(0.,.5,1.),(0.,.5,1.)):
                    base=a*(1-u)+b*u+(ja*(1-u)+jb*u)*smile*(1-.75*emotion)
                    denom=area(base,triangles);relevant=abs(denom)>.05
                    changed=base+(da*(1-u)+db*u)*emotion*reduction
                    ratios=area(changed,triangles)[relevant]/denom[relevant]
                    lowest=int(np.argmin(ratios))
                    if float(ratios[lowest])<minimum:
                        minimum=float(ratios[lowest]);index=int(np.flatnonzero(relevant)[lowest])
                        worst={'pair':name,'articulation':u,'smile':smile,'emotion':emotion,'triangle':triangles[index].tolist(),'base_area':float(denom[index]),'ratio':minimum}
            if diagnostics is not None:
                import json
                with diagnostics.open('a',encoding='utf-8') as stream:stream.write(json.dumps({'state':state,'reduction':reduction,'worst':worst})+'\n')
            if minimum>.2:break
        else:raise PreparationError('Emotion articulation fails the unchanged joint triangle ratio gate: '+state)
        reports[state]={'minimum_triangle_ratio':minimum,'amplitude_reduction':reduction,'pairs':len(pairs),'configurations_per_pair':45}
        if any(np.any(da) or np.any(db) for _,_,da,db in pairs.values()):
            result[state]={name:{'rest_delta':(da*reduction).tolist(),'open_delta':(db*reduction).tolist()} for name,(_,_,da,db) in pairs.items()}
        else:
            # Absent geometry uses the renderer's canonical UV/contact path.
            # A present all-zero entry would still enable joint resampling.
            reports[state]['canonical_geometry']=True
    return {'version':'joint-emotion-articulation/1','states':result,'triangles':{name:t.tolist() for name,t in topology.items()},'validation':{'valid':True,'states':reports},
            'central_contact_pinned':True,'dental_translation':[0.,0.],'tooth_warping':False,'competing_smile_attenuation':.75}

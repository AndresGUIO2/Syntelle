"""Qualify a stronger coherent emotional range from immutable native resources."""
from copy import deepcopy
from hashlib import sha256
from itertools import product, combinations
import argparse
import json
from pathlib import Path
import shutil

import numpy as np
from PIL import Image
from app.preparation.faces import PreparationError
from app.preparation.expressions import validate
from app.workflows.motion.prepare_speech_jaw import prepare as prepare_jaw

STATES = ('neutral','joyful','playful','affectionate','curious','sad','angry','fearful','surprised','disgusted')
SMILES = (0,1,.85,.55,.12,0,0,0,0,0)

def coherent_controls(rig, index, gain):
    smile = rig.get('smile_amounts',SMILES)[index]*gain
    lid = rig['eyelid_tension'][index]*gain
    opening = rig['rest_opening'][index]*gain
    return smile <= 1+1e-9 and lid <= .4+1e-9 and opening <= .4+1e-9

def oral_check(manifest, state, gain):
    oral = manifest['packages']['oral']
    expression = manifest['packages']['expressions']
    rig = expression['emotion_rig']['oral']
    minimum = 1.
    for name,deltas in rig['states'].get(state,{}).items():
        pair = oral['pairs'][name]
        tri = np.asarray(rig['triangles'][name])
        a,b = [np.asarray(pair['mesh'][key],np.float32).astype(float) for key in ('rest','open')]
        da,db = [np.asarray(deltas[key])*gain for key in ('rest_delta','open_delta')]
        ja,jb = [np.asarray(expression['joint_articulation']['pairs'][name][key]) for key in ('rest_delta','open_delta')]
        def area(v):
            x=v[tri[:,1]]-v[tri[:,0]];y=v[tri[:,2]]-v[tri[:,0]]
            return x[:,0]*y[:,1]-x[:,1]*y[:,0]
        for u,smile,amount in product((0.,.25,.5,.75,1.),(0.,.5,1.),(0.,.2,.4,.5,.75,1.)):
            base=a*(1-u)+b*u+(ja*(1-u)+jb*u)*smile*(1-.75*amount)
            denominator=area(base);relevant=abs(denominator)>.05
            ratios=area(base+(da*(1-u)+db*u)*amount)[relevant]/denominator[relevant]
            minimum=min(minimum,float(ratios.min()))
    return {'valid':minimum>.2,'minimum_triangle_ratio':minimum}

def prepare(source, landmarks, output):
    if output.exists(): raise PreparationError('Choose a new output directory; preserve rejected preparations.')
    manifest=json.loads((source/'manifest.json').read_bytes())
    if manifest['packages']['motion'].get('emotional_presence'):
        raise PreparationError('Gain must be applied once from a legacy source, never compounded.')
    output.mkdir(parents=True)
    (output/'input-manifest.json').write_bytes((source/'manifest.json').read_bytes())
    expression=manifest['packages']['expressions'];rig=expression['emotion_rig']
    texture=manifest['textures']['expressions/'+rig['field']['file']]
    raw=np.frombuffer((source/texture['file']).read_bytes(),np.uint8).reshape(texture['height'],texture['width'],4)
    width,height=rig['field_size']
    original=(raw[:,:,:2].reshape(10,height,width,2).astype(float)-128)*rig['units_per_byte']
    legacy_rec=manifest['textures']['expressions/'+expression['field']['file']]
    legacy=(np.frombuffer((source/legacy_rec['file']).read_bytes(),np.uint8).reshape(height,width,4)[:,:,:3].astype(float)-128)*expression['units_per_byte']
    # Preserve native quantization precision. A coarser shared scale can create
    # derivative steps even for a nominal1.00 fallback; such drafts stay rejected.
    units=rig['units_per_byte']
    fields=[];gains=[];checks={};attempts=[]
    for i,state in enumerate(STATES):
        if not i: fields.append(original[i]);gains.append(1.);continue
        for step in range(125,99,-1):
            gain=step/100
            controls=coherent_controls(rig,i,gain)
            result={'valid':False,'reason':'prepared-smile-lid-opening-range'}
            oral={'valid':False}
            if controls:
                packed=np.rint(original[i]*gain/units)+128
                if packed.min()<0 or packed.max()>255:
                    attempts.append({'state':state,'gain':gain,'field':{'valid':False,'reason':'quantized-range'},'oral':oral})
                    continue
                decoded=(packed-128)*units
                field=np.zeros((height,width,3));field[:,:,:2]=decoded;field[:,:,2]=legacy[:,:,2]
                result=validate(field,rig['crop'],inverse_iterations=12,brow_attenuation=.75)
                oral=oral_check(manifest,state,gain)
            attempts.append({'state':state,'gain':gain,'field':result,'oral':oral})
            if controls and result['valid'] and oral['valid']:
                fields.append(decoded);gains.append(gain);checks[state]={'field':result,'oral':oral};break
        else:
            (output/'rejected-attempts.json').write_text(json.dumps(attempts,indent=2))
            raise PreparationError('No unchanged-gate emotional gain qualifies: '+state)
    fields=np.asarray(fields)
    transitions={}
    for a,b in combinations(range(10),2):
        field=np.zeros((height,width,3));field[:,:,:2]=(fields[a]+fields[b])*.5;field[:,:,2]=legacy[:,:,2]
        result=validate(field,rig['crop'],inverse_iterations=12,brow_attenuation=.75)
        transitions[STATES[a]+'-'+STATES[b]]=result
        if not result['valid']:
            (output/'rejected-transitions.json').write_text(json.dumps(transitions,indent=2))
            raise PreparationError('Qualified-state transition failed unchanged field gate.')
    package=output/'before-jaw/package'
    shutil.copytree(source,package)
    atlas=np.full((height*10,width,4),128,np.uint8);atlas[:,:,2]=0;atlas[:,:,3]=255
    atlas[:,:,:2]=np.rint(fields/units+128).astype(np.uint8).reshape(height*10,width,2)
    png=output/'emotion-fields.png';Image.fromarray(atlas).save(png)
    content=atlas.tobytes();digest=sha256(content).hexdigest();relative='data/'+digest+'.rgba'
    (package/relative).write_bytes(content)
    manifest['textures']['expressions/'+rig['field']['file']]={**texture,'file':relative,'sha256':digest,'source':png.as_posix(),'source_sha256':sha256(png.read_bytes()).hexdigest()}
    rig['field']['sha256']=sha256(png.read_bytes()).hexdigest();rig['units_per_byte']=units
    rig['smile_amounts']=[value*gain for value,gain in zip(rig.get('smile_amounts',SMILES),gains)]
    for key in ('eyelid_tension','rest_opening'):
        rig[key]=[value*gain for value,gain in zip(rig[key],gains)]
    for state,pairs in rig['oral']['states'].items():
        gain=gains[STATES.index(state)]
        for deltas in pairs.values():
            for key in ('rest_delta','open_delta'): deltas[key]=(np.asarray(deltas[key])*gain).tolist()
    rig['oral']['validation']={'valid':True,'states':{state:value['oral'] for state,value in checks.items()}}
    rig['prepared_gain']={state:gain for state,gain in zip(STATES,gains)}
    rig['calibration']='coherent-emotional-range/1'
    rig['validation']={'valid':True,'states':checks,'transition_midpoints':transitions}
    presence={'version':'emotional-presence/1','source_sha256':rig['source_sha256'],
              'minimum_automatic_intensity':.4,'inactivity_ms':180000,'recovery_ms':60000,
              'gains':rig['prepared_gain'],'validation':{'valid':True}}
    manifest['packages']['motion']['emotional_presence']=presence
    manifest['packages']['motion'].pop('speech_jaw',None)
    manifest['identity_files']=[v for v in manifest['identity_files'] if v['path'] not in ('speech-jaw/1','speech-jaw/2')]
    encoded=json.dumps({'presence':presence,'rig':rig,'texture':digest},sort_keys=True,separators=(',',':')).encode()
    manifest['identity_files'].append({'path':'emotional-presence/1','sha256':sha256(encoded).hexdigest(),'bytes':len(encoded)})
    manifest['package_hash']=sha256(json.dumps(manifest['identity_files'],separators=(',',':'),ensure_ascii=False).encode()).hexdigest()
    (package/'manifest.json').write_text(json.dumps(manifest,indent=2))
    (output/'qualification.json').write_text(json.dumps({'gains':presence['gains'],'attempts':attempts,'transitions':transitions,'provider_requests':0},indent=2))
    jaw=prepare_jaw(package,landmarks,output/'prepared')
    report={'version':'coherent-emotional-range/1','package_hash':jaw['package_hash'],'gains':presence['gains'],
            'source_package_hash':json.loads((source/'manifest.json').read_bytes())['package_hash'],
            'geometry_unchanged':True,'oral_materials_unchanged':True,'laughter_resource_unchanged':True,'provider_calls':0}
    (output/'report.json').write_text(json.dumps(report,indent=2));return report

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ('source','landmarks','output'): parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args();print(json.dumps(prepare(args.source,args.landmarks,args.output)))

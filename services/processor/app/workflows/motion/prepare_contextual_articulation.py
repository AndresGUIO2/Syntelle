"""Prepare optional source-bound articulation modes without provider requests."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
import numpy as np

def prepare(source: Path, output: Path):
    if output.exists():
        raise ValueError('Preserve existing preparation; select a new output')
    manifest=json.loads((source/'manifest.json').read_bytes())
    original=manifest['package_hash']
    checks={}
    for name,pair in manifest['packages']['oral']['pairs'].items():
        mesh=pair.get('mesh')
        if not mesh or not pair.get('safe'): continue
        rest=np.asarray(mesh['rest'],dtype=np.float32).astype(float); opened=np.asarray(mesh['open'],dtype=np.float32).astype(float)
        delta=opened-rest
        triangles=np.asarray(mesh['triangles'])
        def areas(points):
            p=points[triangles]; a=p[:,1]-p[:,0];b=p[:,2]-p[:,0]
            return a[:,0]*b[:,1]-a[:,1]*b[:,0]
        gain=1.; minimum=1.
        while gain>=0:
            minimum=1.
            for t in (0,.25,.5,.75,1):
                base=rest+(opened-rest)*t; a=areas(base); valid=np.abs(a)>1e-8
                for x in (-.05,.05):
                    for y in (-.05,.05):
                        support=(t if name.startswith("Rest-") else 1)*(1-t if name.endswith("-Rest") else 1)
                        b=areas(base+delta*np.array([x,y])*gain*support)
                        minimum=min(minimum,float(np.min(b[valid]/a[valid])))
            if minimum>=.95:break
            gain=round(gain-.01,2)
        if gain<0:raise ValueError('No safe residual mode: '+name)
        pair['residual_gain']=gain
        checks[name]={'gain':gain,'minimum_relative_area':minimum}
    motion=manifest['packages']['motion']['conversational_motion']
    motion.update(presence_version='conversational-presence/5',articulation_version='contextual-articulation/1')
    record={'version':'contextual-articulation/1','source_package_hash':original,
            'motion':motion,'maximum_source_fraction':.05,'pairs':checks,'provider_calls':0}
    encoded=json.dumps(record,sort_keys=True,separators=(',',':')).encode()
    manifest['identity_files'].append({'path':'contextual-articulation/1','bytes':len(encoded),'sha256':sha256(encoded).hexdigest()})
    manifest['package_hash']=sha256(json.dumps(manifest['identity_files'],separators=(',',':'),ensure_ascii=False).encode()).hexdigest()
    shutil.copytree(source,output)
    (output/'manifest.json').write_text(json.dumps(manifest,indent=2))
    return record

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    for name in ('source','output'):parser.add_argument('--'+name,type=Path,required=True)
    args=parser.parse_args()
    record=prepare(args.source,args.output)
    (args.output.parent/'articulation-calibration.json').write_text(json.dumps(record,indent=2))
    print(json.dumps({'qualified_pairs':len(record['pairs']),'provider_calls':0}))

"""Prepare a separate RGB-D candidate from an explicit manifest; never calls providers."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil
import time

import numpy as np
from PIL import Image

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.faces import PreparationError
from app.preparation.projection_rig import prepare, SETTINGS, VERSION
from app.workflows.visemes.viseme_trial import write_report


def verified_cache(out, binding):
    record=out/'record.json';inputs=out/'input-record.json'
    if not inputs.exists():return False
    if json.loads(inputs.read_text())['binding']!=binding:
        raise PreparationError('Preserve previous preparation and select a new output directory.')
    if not record.exists():
        raise PreparationError('Matching preparation previously failed; inspect its evidence rather than overwrite it.')
    package=json.loads((out/'package.json').read_text())
    stored=package.pop('build_id')
    if sha256(encoded(package)).hexdigest()!=stored or sha256((out/'projection.bin').read_bytes()).hexdigest()!=package['projection']['geometry']['sha256']:
        raise PreparationError('Cached projection outputs have changed; refusing installation.')
    return True


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest',type=Path,required=True)
    parser.add_argument('--output',type=Path,default=ROOT/'.tools/rgbd-motion-18/prepared-current')
    parser.add_argument('--install-preview',action='store_true')
    args=parser.parse_args();manifest=json.loads(args.manifest.read_text());db=Archive()
    implementation=ROOT/'services/processor/app/preparation/projection_rig.py'
    for item in manifest['characters']:
        name=item['id']
        if not name.replace('-','').isalnum():raise PreparationError('Invalid character identifier.')
        inputs={key:(ROOT/path).resolve() for key,path in item.items() if key!='id'}
        hashes={key:sha256(path.read_bytes()).hexdigest() for key,path in inputs.items()}
        binding=sha256(encoded({'inputs':hashes,'settings':SETTINGS,'implementation':sha256(implementation.read_bytes()).hexdigest(),
                               'workflow':sha256(Path(__file__).read_bytes()).hexdigest()})).hexdigest()
        out=args.output/name;out.mkdir(parents=True,exist_ok=True)
        cached=out/'record.json'
        if verified_cache(out,binding):
            print(name,'matching cached preparation',flush=True)
        else:
            if cached.exists():raise PreparationError('Preserve prior preparation and select a new output folder.')
            source=out/'inputs';source.mkdir(exist_ok=True)
            for key,path in inputs.items():shutil.copyfile(path,source/(key+path.suffix))
            write_report(out/'settings.json',SETTINGS)
            write_report(out/'input-record.json',{'hashes':hashes,'binding':binding,'version':VERSION})
            shutil.copyfile(implementation,out/'projection_rig.py')
            shutil.copyfile(Path(__file__),out/'prepare_projection_motion.py')
            db.archive_tree('rgbd-motion-18/'+name+'/inputs',source)
            started=time.perf_counter()
            package=json.loads(inputs['package'].read_text())
            if hashes['original']!=package['original_sha256']:
                raise PreparationError('Immutable original does not match the motion package.')
            try:
                rig,v,t,e,fields=prepare(package,np.load(inputs['depth']),
                    json.loads(inputs['landmarks'].read_text())['landmarks_px'],
                    np.load(inputs['classes'])['probabilities'],Image.open(inputs['foreground']).convert('RGBA'))
                binary=v.tobytes()+t.tobytes()+e.tobytes()
                (out/'projection.bin').write_bytes(binary)
                for file,values in fields.items():np.save(out/file,values)
                rig['geometry']['sha256']=sha256(binary).hexdigest()
                rig['source_sha256']=package['original_sha256'];rig['parent_build_id']=package['build_id']
                rig['preparation_binding']=binding
                package['projection']=rig
                package.pop('build_id')
                package['build_id']=sha256(encoded(package)).hexdigest()
                write_report(out/'package.json',package)
                if not rig['ready']:
                    raise PreparationError('No safe, perceptible head-turn envelope; inspect preserved validation attempts.')
                write_report(out/'record.json',{'binding':binding,'input_hashes':hashes,'settings':SETTINGS,
                    'implementation_sha256':sha256(implementation.read_bytes()).hexdigest(),
                    'seconds':time.perf_counter()-started,'version':VERSION,'new_provider_requests':0})
            except (PreparationError,ValueError) as failure:
                write_report(out/'failure.json',{'binding':binding,'error':str(failure),'seconds':time.perf_counter()-started})
                db.archive_tree('rgbd-motion-18/'+name+'/failed',out)
                raise
            print(name,db.archive_tree('rgbd-motion-18/'+name,out),flush=True)
        if args.install_preview:
            target=ROOT/'apps/web/src/experiments/motion/rgbd'/name
            target.mkdir(parents=True,exist_ok=True)
            for file in ['package.json','projection.bin']:shutil.copyfile(out/file,target/file)


if __name__=='__main__':main()

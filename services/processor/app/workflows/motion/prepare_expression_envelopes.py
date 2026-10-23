"""Check wider candidate envelopes against immutable cached RGB-D geometry."""
import argparse
import json
import shutil
from pathlib import Path

import numpy as np

from app.core.paths import ROOT
from app.preparation.archive import Archive,encoded
from app.preparation.projection_rig import validate
from app.workflows.motion.prepare_motion import digest


def run(manifest,output):
    settings=json.loads(manifest.read_text())
    if output.exists() and any(output.iterdir()):raise ValueError('Preserve existing envelope evidence; use a new output directory.')
    output.mkdir(parents=True,exist_ok=True)
    (output/'input-manifest.json').write_bytes(manifest.read_bytes())
    reports=[]
    for case in settings['characters']:
        path=ROOT/case['package']
        package=json.loads(path.read_text());p=package['projection']
        binary=path.parent/p['geometry']['file']
        inputs=output/'inputs'/case['id'];inputs.mkdir(parents=True)
        shutil.copyfile(path,inputs/'package.json')
        shutil.copyfile(binary,inputs/'geometry.bin')
        if digest(binary)!=p['geometry']['sha256']:raise ValueError('Cached geometry hash differs.')
        count=p['geometry']['vertices'];triangles=p['geometry']['triangles']
        data=binary.read_bytes()
        vertices=np.frombuffer(data[:count*20],'<f4').reshape(-1,5)
        indices=np.frombuffer(data[count*20:count*20+triangles*6],'<u2').reshape(-1,3)
        base=np.array([p['bounds'][key] for key in ('yaw','pitch','roll')])
        for scale in (1.25,1.5):
            report=validate(vertices,indices,p['camera'],p['pivot'],base*scale,package['anatomy']['face_height'])
            reports.append({'character':case['id'],'scale':scale,'source_build_id':package['build_id'],
                            'geometry_sha256':digest(binary),'validation':report,'status':'visual-review-required' if report['valid'] else 'rejected'})
    record={'version':'expression-envelopes/1','candidates':reports,'paid_provider_calls':0}
    (output/'envelopes.json').write_bytes(encoded(record))
    (output/'input-manifest.json').write_bytes(manifest.read_bytes())
    print(json.dumps({'candidates':reports,'archive':Archive().archive_tree('expressive-performance-67/envelopes',output)}))


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args();run(args.manifest.resolve(),args.output.resolve())

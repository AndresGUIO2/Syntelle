"""Build a separate, reproducible head-motion candidate from cached resources."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np
from PIL import Image

from app.preparation.archive import Archive, encoded
from app.preparation.head_rig import prepare
from app.core.paths import PREPARATION_DIR
from app.workflows.visemes.viseme_trial import ROOT, write_report


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input',type=Path,default=ROOT/'.tools/foreground-11/prepared')
    parser.add_argument('--output',type=Path,default=ROOT/'.tools/head-motion-14')
    args=parser.parse_args();db=Archive()
    for name in ['syntelle','cafe','woodland']:
        source=args.input/name;out=args.output/name;out.mkdir(parents=True,exist_ok=True)
        manifest=json.loads((source/'package.json').read_text())
        points=json.loads((source/'base-landmarks.json').read_text())['landmarks_px']
        classes=np.load(source/'class-confidence.npz')['probabilities']
        rig=prepare(points,classes,Image.open(source/'depth.webp').convert('L'),(manifest['canvas']['width'],manifest['canvas']['height']))
        rig['source_sha256']=manifest['original_sha256'];rig['previous_build_id']=manifest['build_id']
        rig['implementation_sha256']=sha256((PREPARATION_DIR/'head_rig.py').read_bytes()).hexdigest()
        if (out/'package.json').exists() and json.loads((out/'package.json').read_text()).get('rig',{}).get('version')!=rig['version']:
            raise SystemExit('Preserve the earlier rig and select a separate output directory.')
        result={**manifest,'rig':rig};result['build_id']=sha256(encoded(result)).hexdigest()
        write_report(out/'package.json',result)
        for file in ['base-landmarks.json','class-confidence.npz','depth.webp','foreground.webp']:
            shutil.copyfile(source/file,out/file)
        shutil.copyfile(Path(__file__),out/'prepare_head_motion.py')
        shutil.copyfile(PREPARATION_DIR/'head_rig.py',out/'head_rig.py')
        print(name,rig['geometry_validation'],flush=True)
        print(db.archive_tree(args.output.name+'/'+name,out),flush=True)


if __name__=='__main__':main()

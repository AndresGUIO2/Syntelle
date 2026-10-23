"""Offline motion evidence audit, PostgreSQL snapshot and real-file restoration."""
import base64
from hashlib import sha256
from importlib.metadata import version
import json
from pathlib import Path
import shutil
import sys
import subprocess

import numpy as np
from PIL import Image,ImageOps

from app.workflows.visemes.viseme_trial import ROOT,write_report
from app.preparation.archive import Archive,encoded
from app.preparation.faces import PreparationError
from app.preparation.motion_assets import cached


def digest(path):return sha256(path.read_bytes()).hexdigest()


def main():
    root=ROOT/'.tools/motion-09';config_path=ROOT/'docs/characters/motion-development.json'
    config=json.loads(config_path.read_text());db=Archive();budget=config['budget_id']
    db.budget(budget,digest(config_path),config['limits'])
    with db.connect() as connection:
        rows=connection.execute('SELECT request_key,kind,binding,status,usage FROM requests WHERE budget_id=%s ORDER BY request_key',(budget,)).fetchall()
    if len(rows)>6 or any(r['status']!='received' for r in rows):raise PreparationError('Motion request budget/status audit failed.')
    requests={row['request_key']:row for row in rows};cases={};oral_snapshots={}
    for name,item in config['cases'].items():
        folder=root/name;original=(config_path.parent/item['original']).resolve()
        for stage in ('proposal','correction'):
            if not (folder/stage/'run.json').exists():continue
            record=json.loads((folder/stage/'run.json').read_text());payload=json.loads((folder/stage/'request.json').read_text())
            assert base64.b64decode(payload['input'][1]['data'])==original.read_bytes()
            assert (folder/stage/'prompt.txt').read_text(encoding='utf-8')==payload['input'][0]['text']
            assert record['binding']==sha256(encoded(payload)).hexdigest()==requests[f'{name}/{stage}']['binding']
            assert digest(folder/stage/record['file'])==record['raw_sha256']
            assert record['attempts']==1 and record['settings']['retries']==0
        eyes=json.loads((folder/'eyes.json').read_text());rect=eyes['crop']
        base=np.asarray(ImageOps.exif_transpose(Image.open(original)).convert('RGB'))
        closed=np.asarray(Image.open(folder/'closed-composition.png').convert('RGB'))
        mask=np.zeros(base.shape[:2],bool);x,y=rect['x'],rect['y'];mask[y:y+rect['height'],x:x+rect['width']]=np.asarray(Image.open(folder/'eyes/mask.png'))>0
        assert np.array_equal(base[~mask],closed[~mask])
        report=json.loads((folder/'browser-review/report.json').read_text())
        assert report['build_id']==json.loads((folder/'package.json').read_text())['build_id']
        assert all(report[key]==0 for key in ('maximumOutsideEyeChange','maximumExposedIrisChange','maximumClosedError','maxEndpointJump'))
        assert len(report['checks'])==1215 and report['uploads']['fullUploads']==1
        for file in ('segmentation.json','depth.json'):
            key=json.loads((folder/file).read_text())['cache_binding'];assert cached(folder,file,key)
        cases[name]={'source_sha256':digest(original),'eyes_outside_mask_changed_pixels':0,'browser_frames':len(report['checks']),
            'motion_build_id':json.loads((folder/'package.json').read_text())['build_id'],'performance':report['performance']}
        oral_path=(config_path.parent/item['oral']).resolve()
        assert report['oral_build_id']==json.loads((oral_path/'package.json').read_text())['build_id']
        oral_snapshots[name]=db.archive_tree(budget+'/oral-dependency/'+name,oral_path)
    snapshot=root/'pipeline-snapshot'
    sources=[*(path for path in (ROOT/'services/processor/app').rglob('*') if path.suffix in ('.py','.sql')),ROOT/'services/processor/pyproject.toml',ROOT/'services/processor/uv.lock',
        ROOT/'services/processor/tests/test_motion_preparation.py',ROOT/'services/processor/tests/test_preparation_archive.py',ROOT/'apps/web/package.json',ROOT/'apps/web/package-lock.json',
        *(path for path in (ROOT/'apps/web/src').rglob('*') if path.suffix in ('.ts','.vue','.css')),ROOT/'apps/web/vite.config.ts',ROOT/'apps/web/playwright.config.ts',ROOT/'apps/web/tsconfig.json',
        ROOT/'apps/web/scripts/export-motion-review.mjs',ROOT/'apps/web/scripts/measure-motion.mjs',ROOT/'apps/web/tests/e2e/motion.spec.ts',config_path,ROOT/'docs/prompts/closed-eyes-v1.txt',
        ROOT/'docs/experiments/motion/motion-trial.md',ROOT/'README.md',ROOT/'AGENTS.md',ROOT/'specs/001-syntelle-demo/tasks.md']
    for source in sources:
        if not source.is_file():continue
        target=snapshot/source.relative_to(ROOT);target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(source,target)
    write_report(snapshot/'runtime-versions.json',{'python':sys.version,'node':subprocess.check_output(['node','--version'],text=True).strip(),
        **{name:version(name) for name in ['mediapipe','numpy','opencv-contrib-python','pillow','torch','transformers','psutil','psycopg']}})
    models=db.archive_tree(budget+'/models',ROOT/'.tools/models')
    result={'budget_id':budget,'request_counts':{'image':sum(r['kind']=='image' for r in rows),'analysis':sum(r['kind']=='analysis' for r in rows)},
        'limits':config['limits'],'requests':rows,'cases':cases,'model_snapshot':models,'oral_snapshots':oral_snapshots,'source_prompts_and_configuration_preserved':True}
    write_report(root/'audit.json',result)
    archived=db.archive_tree(budget+'/complete-evidence',root)
    restored=ROOT/'.tools/motion-09-restored'/str(archived['snapshot_id'])
    count=db.restore(archived['snapshot_id'],restored)
    for source in root.rglob('*'):
        if source.is_file() and '__pycache__' not in source.parts:assert digest(source)==digest(restored/source.relative_to(root))
    restored_models=ROOT/'.tools/motion-09-restored/models'/str(models['snapshot_id'])
    model_count=db.restore(models['snapshot_id'],restored_models)
    for source in (ROOT/'.tools/models').rglob('*'):
        if source.is_file():assert digest(source)==digest(restored_models/source.relative_to(ROOT/'.tools/models'))
    verification={'snapshot':archived,'restored_files':count,'destination':str(restored),'all_bytes_verified':True,
        'restored_model_files':model_count,'model_destination':str(restored_models),'all_model_bytes_verified':True}
    write_report(ROOT/'.tools/motion-09-postgres-restore.json',verification)
    db.event(budget,'verified-full-restoration',verification)
    print(json.dumps({'images':result['request_counts']['image'],'analyses':result['request_counts']['analysis'],'restored_files':count,'snapshot_id':archived['snapshot_id']},indent=2))


if __name__=='__main__':main()

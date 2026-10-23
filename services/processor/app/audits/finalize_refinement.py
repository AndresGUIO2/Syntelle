"""Audit and restore the offline foreground/refactor evidence without paid calls."""
from hashlib import sha256
from importlib.metadata import version
import json
from pathlib import Path
import shutil
import sys
import subprocess

import numpy as np
from PIL import Image, ImageOps

from app.preparation.archive import Archive
from app.preparation.foreground import cache_settings
from app.preparation.motion_assets import binding, cached
from app.preparation.faces import PreparationError
from app.workflows.visemes.viseme_trial import ROOT, write_report


def digest(path):
    return sha256(path.read_bytes()).hexdigest()


def require(condition, message):
    if not condition:
        raise PreparationError(message)


def source_snapshot(directory):
    sources=[]
    for folder, extensions in [
        ('apps/web/src', {'.ts','.vue','.css'}),
        ('apps/web/scripts', {'.mjs'}), ('apps/web/tests', {'.ts'}),
        ('services/processor/app', {'.py','.sql'}),
        ('services/processor/tests', {'.py'}), ('services/api', {'.go','.mod'}),
    ]:
        sources.extend(p for p in (ROOT/folder).rglob('*') if p.suffix in extensions)
    for name in ['apps/web/package.json','apps/web/package-lock.json','apps/web/tsconfig.json',
                 'apps/web/vite.config.ts','apps/web/playwright.config.ts','apps/web/eslint.config.js',
                 'apps/web/.prettierrc.json','apps/web/.prettierignore',
                 'services/processor/pyproject.toml','services/processor/uv.lock',
                 'docs/characters/motion-development.json','docs/development/code-organization.md',
                 'docs/experiments/motion/foreground-refinement.md','docs/experiments/motion/foreground-edge-review.md',
                 'specs/001-syntelle-demo/tasks.md','AGENTS.md']:
        sources.append(ROOT/name)
    for source in sources:
        if not source.is_file(): continue
        target=directory/source.relative_to(ROOT)
        target.parent.mkdir(parents=True,exist_ok=True)
        shutil.copyfile(source,target)
    write_report(directory/'runtime-versions.json',{'python':sys.version,
        'node':subprocess.check_output([str(ROOT/'.tools/node-v24.22.0-win-x64/node.exe'),'--version'],text=True).strip(),
        'go':subprocess.check_output([str(ROOT/'.tools/go/bin/go.exe'),'version'],text=True).strip(),
        **{name:version(name) for name in ['pymatting','numpy','numba','scipy','mediapipe','pillow','psycopg']}})


def main():
    db=Archive(); foreground=ROOT/'.tools/foreground-10'; organization=ROOT/'.tools/organization-10'
    config_path=ROOT/'docs/characters/motion-development.json'
    config=json.loads(config_path.read_text()); cases={}; oral_dependencies={}
    for name,item in config['cases'].items():
        folder=foreground/name
        original=(config_path.parent/item['original']).resolve()
        image=ImageOps.exif_transpose(Image.open(original)).convert('RGB')
        record=json.loads((folder/'segmentation.json').read_text())
        key=binding(image,{'segmentation_sha256':record['model_sha256'],'matting':cache_settings()})
        require(cached(folder,'segmentation.json',key) is not None,'Foreground cache binding is stale.')
        review=json.loads((folder/'foreground-review.json').read_text())
        require(review.get('accepted') and review.get('browser_review'),'Foreground lacks internal browser review.')
        require(review['foreground_sha256']==digest(folder/'foreground.webp'),'Foreground review hash changed.')
        manifest=json.loads((folder/'package.json').read_text())
        require(manifest['original_sha256']==digest(original),'Original source hash changed.')
        rgba=np.asarray(Image.open(folder/'foreground.png').convert('RGBA'))
        opaque=rgba[...,3]==255
        require(np.array_equal(rgba[opaque,:3],np.asarray(image)[opaque]),'Opaque source RGB changed.')
        runtime=ROOT/'apps/web/src/experiments/motion/assets'/name
        require(digest(runtime/'foreground.webp')==review['foreground_sha256'],'Runtime foreground differs from reviewed result.')
        require(digest(runtime/'package.json')==digest(folder/'package.json'),'Runtime motion manifest differs from preparation.')
        oral_path=(config_path.parent/item['oral']).resolve()
        require(manifest['oral_build_id']==json.loads((oral_path/'package.json').read_text())['build_id'],'Oral dependency differs.')
        oral_dependencies[name]=db.archive_tree('foreground-refinement-10/oral-dependency/'+name,oral_path)
        unchanged={}
        for relative in ['depth.webp','eyes/open.webp','eyes/closed.webp','eyes/mask.png']:
            before=ROOT/'.tools/motion-09'/name/relative
            require(digest(before)==digest(folder/relative)==digest(runtime/relative),'Existing eye/depth asset changed.')
            unchanged[relative]=digest(before)
        cases[name]={'source_sha256':digest(original),'foreground_sha256':review['foreground_sha256'],
                     'opaque_changed_pixels':0,'unchanged_dependencies':unchanged,
                     'seconds':record['seconds'],'peak_working_set_bytes':record['peak_working_set_bytes']}
    with db.connect() as conn:
        counts=conn.execute('SELECT budget_id,kind,count(*) AS n FROM requests GROUP BY budget_id,kind ORDER BY budget_id,kind').fetchall()
    motion=[r for r in counts if r['budget_id']==config['budget_id']]
    require(sum(r['n'] for r in motion)==3,'The motion request count changed.')
    pixels=json.loads((organization/'pixel-comparison.json').read_text())
    require(all(delta==0 for rows in pixels.values() for delta in rows.values()),'Rendering regression differs from previous pixels.')
    source_snapshot(organization/'source-snapshot')
    audit={'method':cache_settings(),'cases':cases,'oral_dependencies':oral_dependencies,'new_provider_calls':0,'request_counts':counts,
           'manual_masks':False,'public_upload_validation_complete':False,'owner_visual_acceptance':False,
           'unchanged_rendering_frames':sum(len(rows) for rows in pixels.values())}
    write_report(foreground/'audit.json',audit)
    results={}
    for name,directory in [('foreground-refinement-10',foreground),('code-organization-10',organization)]:
        archived=db.archive_tree(name,directory)
        restored=ROOT/'.tools/refinement-10-restored'/name/str(archived['snapshot_id'])
        count=db.restore(archived['snapshot_id'],restored)
        files=[p for p in directory.rglob('*') if p.is_file() and '__pycache__' not in p.parts and p.suffix not in ('.pyc','.tmp')]
        for path in files:
            require(digest(path)==digest(restored/path.relative_to(directory)),'Restored evidence differs.')
        results[name]={'snapshot':archived,'restored_files':count,'all_bytes_verified':True,'destination':str(restored)}
        db.event(name,'verified-restoration',results[name])
    write_report(ROOT/'.tools/refinement-10-postgres-restore.json',results)
    print(json.dumps(results,indent=2))


if __name__=='__main__':
    try: main()
    except PreparationError as error: raise SystemExit(str(error)) from None

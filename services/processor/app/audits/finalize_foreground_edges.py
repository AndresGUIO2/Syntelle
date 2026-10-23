"""Audit native hair preparation, then archive and verify PostgreSQL restoration."""
import json
from importlib.metadata import version

import numpy as np
from PIL import Image, ImageOps

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.preparation.archive import Archive
from app.preparation.foreground import cache_settings
from app.preparation.hair_matting import model_files
from app.preparation.motion_assets import binding, cached
from app.workflows.visemes.viseme_trial import ROOT, write_report


def main():
    db=Archive(); root=ROOT/'.tools/foreground-11'
    config_path=ROOT/'docs/characters/motion-development.json'
    config=json.loads(config_path.read_text()); cases={}
    model_directory,model_pin=model_files(ROOT/'.tools/models')
    for name,item in config['cases'].items():
        folder=root/'prepared'/name
        original=(config_path.parent/item['original']).resolve()
        image=ImageOps.exif_transpose(Image.open(original)).convert('RGB')
        record=json.loads((folder/'segmentation.json').read_text())
        key=binding(image,{'segmentation_sha256':record['model_sha256'],
                          'matting':cache_settings(),'hair_model':model_pin})
        require(cached(folder,'segmentation.json',key) is not None,'Native hair cache is stale.')
        review=json.loads((folder/'foreground-review.json').read_text())
        require(review.get('accepted') and review.get('browser_review'),'Current visual review is missing.')
        require(review['foreground_sha256']==digest(folder/'foreground.webp'),'Review hash differs.')
        rgba=np.asarray(Image.open(folder/'foreground.png').convert('RGBA'))
        opaque=rgba[...,3]==255
        require(np.array_equal(rgba[opaque,:3],np.asarray(image)[opaque]),'Opaque source texture changed.')
        manifest=json.loads((folder/'package.json').read_text())
        require(manifest['original_sha256']==digest(original),'Immutable portrait changed.')
        runtime=ROOT/'apps/web/src/experiments/motion/assets'/name
        for relative in ['foreground.webp','package.json']:
            require(digest(runtime/relative)==digest(folder/relative),'Installed foreground differs.')
        unchanged={}
        for relative in ['depth.webp','eyes/open.webp','eyes/closed.webp','eyes/mask.png']:
            before=ROOT/'.tools/foreground-10'/name/relative
            require(digest(before)==digest(folder/relative)==digest(runtime/relative),'Eye/depth dependency changed.')
            unchanged[relative]=digest(before)
        previous=json.loads((ROOT/'.tools/foreground-10'/name/'package.json').read_text())
        require(manifest['oral_build_id']==previous['oral_build_id'],'Existing mouth dependency changed.')
        cases[name]={'source_sha256':digest(original),'foreground_sha256':review['foreground_sha256'],
                     'opaque_changed_pixels':0,'unchanged_dependencies':unchanged,
                     'oral_build_id':manifest['oral_build_id'],
                     'seconds':record['seconds'],'peak_working_set_bytes':record['peak_working_set_bytes']}
    with db.connect() as conn:
        counts=conn.execute('SELECT budget_id,kind,count(*) AS n FROM requests GROUP BY budget_id,kind ORDER BY budget_id,kind').fetchall()
    require(sum(r['n'] for r in counts if r['budget_id']==config['budget_id'])==3,'Motion request count changed.')
    source_snapshot(root/'source-snapshot')
    write_report(root/'audit.json',{'method':cache_settings(),'model':model_pin,'cases':cases,
        'versions':{name:version(name) for name in ['torch','transformers','pymatting']},
        'request_counts':counts,'new_provider_calls':0,'manual_masks':False,
        'owner_visual_acceptance':False,'public_upload_validation_complete':False})
    results={}
    for name,directory in [('foreground-edge-11/model',model_directory),('foreground-edge-11',root)]:
        archived=db.archive_tree(name,directory)
        restored=ROOT/'.tools/foreground-11-restored'/str(archived['snapshot_id'])
        count=db.restore(archived['snapshot_id'],restored)
        for path in directory.rglob('*'):
            if not path.is_file() or '__pycache__' in path.parts or path.suffix in ('.pyc','.tmp'):continue
            require(digest(path)==digest(restored/path.relative_to(directory)),'Restored evidence differs.')
        results[name]={'snapshot':archived,'restored_files':count,'all_bytes_verified':True}
        db.event(name,'verified-restoration',results[name])
    write_report(ROOT/'.tools/foreground-11-postgres-restore.json',results)
    print(json.dumps(results,indent=2))


if __name__=='__main__':
    main()

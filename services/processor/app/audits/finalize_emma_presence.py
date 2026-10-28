"""Freeze phase-107 sources/results and verify PostgreSQL restoration; no providers."""
import json
import shutil
import subprocess
from pathlib import Path
from hashlib import sha256

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder=ROOT/'.tools/emma-presence-107';db=Archive()
    with db.connect() as conn:
        operations=conn.execute('SELECT request_key,status,binding,request,result,diagnostics FROM syntelle_conversation.operations WHERE kind=%s ORDER BY id',('owner-requested-emma-laughter-2026-10-27',)).fetchall()
    assert len(operations)==2 and all(row['status']=='received' for row in operations)
    assert sum(row['request']['characters'] for row in operations)==20
    (folder/'complete-operations.json').write_bytes(db.checked(encoded(operations)))
    for name in ['baseline-en-US-review','candidate-en-US-review','candidate-es-CO-review','laugh-en-US-review','laugh-es-CO-review','candidate-native-timing','webkit-laughter','installed-candidate','installed-en-US','installed-es-CO']:
        assert json.loads((folder/f'{name}.json').read_bytes())['passed'],name
    source=folder/'final-source'
    for name in ['packages/render-contract','services/api/cmd','services/api/internal','services/render-worker/cmd','services/render-worker/internal','services/render-worker/scripts','services/processor/app','services/processor/tests','apps/web/src','apps/web/tests','apps/web/config','specs/004-coordinated-performance']:
        shutil.copytree(ROOT/name,source/name,dirs_exist_ok=True,ignore=shutil.ignore_patterns('__pycache__','*.pyc','node_modules','.venv'))
    for name in ['AGENTS.md','compose.yaml','docs/README.md','docs/experiments/motion/emma-presence.md','services/api/go.mod','services/api/go.sum','services/render-worker/go.mod','services/render-worker/go.sum','apps/web/package.json','apps/web/package-lock.json','apps/web/vite.config.ts','services/processor/pyproject.toml']:
        target=source/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(ROOT/name,target)
    previous=ROOT/'.tools/av-sync-emma-106/final-source';unchanged={}
    for group in ['services/render-worker/internal/nativerender','services/render-worker/internal/nativemedia','services/render-worker/internal/nativeadapter','apps/web/src']:
        for path in (previous/group).rglob('*'):
            if not path.is_file():continue
            relative=path.relative_to(previous);current=ROOT/relative
            assert path.read_bytes()==current.read_bytes(),relative
            unchanged[relative.as_posix()]=sha256(current.read_bytes()).hexdigest()
    for name in ['services/render-worker/internal/nativeanimation/speech.go','services/render-worker/internal/nativeanimation/laughter_bridge.go','packages/render-contract/gestures.go','packages/render-contract/conversational_motion.go']:
        assert (ROOT/name).read_bytes()==(previous/name).read_bytes(),name
        unchanged[name]=sha256((ROOT/name).read_bytes()).hexdigest()
    images={};binaries=folder/'binaries';binaries.mkdir(exist_ok=True)
    for role,image in [('worker','syntelle-render-native:emma-presence-107'),('previous-worker','syntelle-render-native:before-emma-presence-107')]:
        images[role]=subprocess.check_output(['docker','image','inspect',image,'--format','{{.Id}}'],text=True).strip()
        with (binaries/role).open('wb') as output:subprocess.run(['docker','run','--rm','--pull','never','--network','none','--entrypoint','cat',image,'/usr/local/bin/syntelle-render-worker'],stdout=output,check=True)
    for role,image in [('api','syntelle-api:before-emma-presence-107'),('web','syntelle-web:before-emma-presence-107'),('probe','syntelle-laughter-probe:107')]:
        images[role]=subprocess.check_output(['docker','image','inspect',image,'--format','{{.Id}}'],text=True).strip()
    mounts=json.loads(subprocess.check_output(['docker','inspect','syntelle-api-1','--format','{{json .Mounts}}'],text=True))
    selected=next(m['Source'] for m in mounts if m['Destination']=='/opt/syntelle-performance')
    assert '/emma-presence-107/' in selected.replace('\\','/'), 'Current API is not the phase-107 profile'
    profile_name=Path(selected).name
    profile=json.loads((folder/profile_name/'profile.json').read_bytes());qualification=json.loads((folder/profile_name/'laughter-qualification.json').read_bytes())
    for language in ['en-US','es-CO']:
        if qualification[language]['accepted']:
            review=json.loads((folder/'audible-review'/(language+'.json')).read_bytes())
            assert review['accepted'] and review['audio_sha256']==qualification[language]['sha256']
            assert profile['voice']+'|'+language in profile['laughter']
    report={'version':'emma-presence/1','parent_snapshots':[753,754],'resource_snapshot':730,'installed_local_movement':True,'profile':profile_name,'voice':profile['voice'],'laughter_qualification':qualification,'enabled_laughter_languages':[key.split('|')[1] for key in profile['laughter']],'owner_audible_review':'accepted' if all(qualification.get(language,{}).get('accepted') for language in ['en-US','es-CO']) else 'pending','attempts':2,'submitted_characters':20,'provider_failures':0,'retries':0,'image_requests':0,'analysis_requests':0,'new_conversation_model_calls':0,'images':images,'unchanged_sources':unchanged,'mouth_timeline_changed':False,'audio_rate':1,'comparison':json.loads((folder/'comparison.json').read_bytes()),'isolated_30fps_qualification':'not established; unrelated GPU saturation','owner_naturalness':False}
    package=ROOT/'.tools/paired-parallax-100/prepared-v2/package';report['package_hash']=json.loads((package/'manifest.json').read_bytes())['package_hash']
    assert report['package_hash']=='eb1c1df5e15d773cab77da7f877f048447f3b2b1cffb6865524d69d645745da9'
    (folder/'activation.json').write_bytes(encoded(report))
    snapshot=db.archive_tree('emma-presence-107',folder,report)
    restored=ROOT/f'.tools/emma-presence-107-restored-{snapshot["snapshot_id"]}';count=db.restore(snapshot['snapshot_id'],restored)
    for path in restored.rglob('*'):
        if path.is_file():assert path.read_bytes()==(folder/path.relative_to(restored)).read_bytes()
    resources=ROOT/'.tools/emma-presence-107-resources-restored-730';resource_count=db.restore(730,resources)
    for path in resources.rglob('*'):
        if path.is_file():assert path.read_bytes()==(package/path.relative_to(resources)).read_bytes()
    receipt={**snapshot,'restored_files':count,'all_hashes_match':True,'enabled_laughter_languages':report['enabled_laughter_languages'],'owner_audible_review':report['owner_audible_review'],'unchanged_resources':{'snapshot_id':730,'restored_files':resource_count,'all_hashes_match':True}}
    db.event('emma-presence-107','restoration-verified',receipt)
    (ROOT/'.tools/emma-presence-107-archive.json').write_bytes(encoded(receipt));print(json.dumps(receipt))


if __name__=='__main__':main()

"""Archive and byte-verify the local A/V presentation and Emma correction."""
import json
import shutil
import subprocess
from hashlib import sha256

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder = ROOT / '.tools/av-sync-emma-106'
    db = Archive()
    with db.connect() as conn:
        operations = conn.execute('SELECT request_key,status,binding,request,result,diagnostics FROM syntelle_conversation.operations WHERE kind=%s ORDER BY id', ('owner-requested-emma-2026-10-27',)).fetchall()
    assert len(operations) == 2 and all(row['status'] == 'received' for row in operations)
    assert {row['result']['voice'] for row in operations} == {'en-US-Emma:DragonHDLatestNeural'}
    characters = sum(row['request']['characters'] for row in operations)
    assert characters <= 1000
    (folder/'speech/complete-operations.json').write_bytes(db.checked(encoded(operations)))
    required = ['candidate-native-timing', 'baseline-native-timing', 'candidate-review', 'installed-candidate', 'webkit-pcm']
    for name in required:
        assert json.loads((folder/f'{name}.json').read_bytes())['passed'], name
    unchanged = {}
    previous = ROOT/'.tools/brief-posture-105/final-source/services/render-worker/internal'
    for group in ['nativeanimation', 'nativeadapter']:
        for path in (previous/group).rglob('*'):
            if not path.is_file(): continue
            relative = path.relative_to(previous)
            current = ROOT/'services/render-worker/internal'/relative
            assert path.read_bytes() == current.read_bytes(), relative
            unchanged[relative.as_posix()] = sha256(current.read_bytes()).hexdigest()
    renderer = ROOT/'services/render-worker/internal/nativerender'
    old_renderer = ROOT/'.tools/coordinated-performance-103/final-source/services/render-worker/internal/nativerender'
    for path in old_renderer.rglob('*'):
        if path.is_file():
            current = renderer/path.relative_to(old_renderer)
            assert current.read_bytes() == path.read_bytes(), current
    source = folder/'final-source'
    for name in ['packages/render-contract','services/api/cmd','services/api/internal','services/render-worker/cmd','services/render-worker/internal','services/render-worker/scripts','services/processor/app','services/processor/tests','apps/web/src','apps/web/tests','apps/web/config','specs/004-coordinated-performance']:
        shutil.copytree(ROOT/name,source/name,dirs_exist_ok=True,ignore=shutil.ignore_patterns('__pycache__','*.pyc','node_modules','.venv'))
    for name in ['AGENTS.md','compose.yaml','docs/README.md','docs/experiments/motion/mouth-presentation-and-emma.md','services/api/go.mod','services/api/go.sum','services/render-worker/go.mod','services/render-worker/go.sum','apps/web/package.json','apps/web/package-lock.json','apps/web/vite.config.ts','services/processor/pyproject.toml']:
        target=source/name;target.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(ROOT/name,target)
    images={}
    binaries=folder/'binaries';binaries.mkdir(exist_ok=True)
    for role,image,executable in [('api','syntelle-api:av-sync-emma-106','syntelle-api'),('worker','syntelle-render-native:av-sync-emma-106','syntelle-render-worker'),('previous-api','syntelle-api:before-av-sync-emma-106','syntelle-api'),('previous-worker','syntelle-render-native:before-av-sync-emma-106','syntelle-render-worker')]:
        images[role]=subprocess.check_output(['docker','image','inspect',image,'--format','{{.Id}}'],text=True).strip()
        with (binaries/role).open('wb') as output:
            subprocess.run(['docker','run','--rm','--pull','never','--network','none','--entrypoint','cat',image,'/usr/local/bin/'+executable],stdout=output,check=True)
    for role,image in [('web','syntelle-web:av-sync-emma-106'),('previous-web','syntelle-web:before-av-sync-emma-106')]:
        images[role]=subprocess.check_output(['docker','image','inspect',image,'--format','{{.Id}}'],text=True).strip()
    report={'version':'av-presentation-emma/1','parents':[744,746],'resource_snapshot':730,'installed_local_review':True,'owner_acceptance':False,'voice':'en-US-Emma:DragonHDLatestNeural','attempts':2,'submitted_characters':characters,'provider_failures':0,'retries':0,'image_requests':0,'analysis_requests':0,'new_model_calls':0,'styles':{'en-US':[],'es-CO':[]},'laughter':{'en-US':False,'es-CO':False},'native_animation_sources_unchanged':unchanged,'mouth_timeline_changed':False,'audio_rate':1,'inline_recapture':False,'images':images,'timing':json.loads((folder/'timing-summary.json').read_bytes()),'full_webkit_streaming_and_physical_devices':'pending'}
    package=ROOT/'.tools/paired-parallax-100/prepared-v2/package'
    report['package_hash']=json.loads((package/'manifest.json').read_bytes())['package_hash']
    assert report['package_hash']=='eb1c1df5e15d773cab77da7f877f048447f3b2b1cffb6865524d69d645745da9'
    (folder/'activation.json').write_bytes(encoded(report))
    snapshot=db.archive_tree('av-sync-emma-106',folder,report)
    destination=ROOT/f'.tools/av-sync-emma-106-restored-{snapshot["snapshot_id"]}'
    count=db.restore(snapshot['snapshot_id'],destination)
    for path in destination.rglob('*'):
        if path.is_file():assert sha256(path.read_bytes()).digest()==sha256((folder/path.relative_to(destination)).read_bytes()).digest()
    resources=ROOT/f'.tools/av-sync-emma-106-resources-restored-730'
    resource_count=db.restore(730,resources)
    for path in resources.rglob('*'):
        if path.is_file():assert path.read_bytes()==(package/path.relative_to(resources)).read_bytes()
    receipt={**snapshot,'restored_files':count,'all_hashes_match':True,'attempts':2,'submitted_characters':characters,'unchanged_resources':{'snapshot_id':730,'restored_files':resource_count,'all_hashes_match':True}}
    db.event('av-sync-emma-106','restoration-verified',receipt)
    (ROOT/'.tools/av-sync-emma-106-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__=='__main__':main()

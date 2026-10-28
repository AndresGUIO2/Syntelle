"""Archive the bounded posture correction without provider or preparation calls."""
import json
import shutil
import subprocess
from hashlib import sha256

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder = ROOT/'.tools/brief-posture-105'
    db = Archive()
    for name in ['services/render-worker/internal/nativeanimation', 'services/render-worker/internal/nativeadapter']:
        shutil.copytree(ROOT/name,folder/'final-source'/name,dirs_exist_ok=True)
    for name in ['AGENTS.md','docs/README.md','docs/experiments/motion/brief-posture.md',
                 'specs/004-coordinated-performance/spec.md','specs/004-coordinated-performance/tasks.md','specs/004-coordinated-performance/verify.md',
                 'services/processor/app/audits/finalize_brief_posture.py']:
        (folder/'final-source'/name).parent.mkdir(parents=True,exist_ok=True)
        shutil.copy2(ROOT/name,folder/'final-source'/name)
    for name in ['baseline','candidate']:
        result=json.loads((folder/f'{name}-review.json').read_bytes())
        assert result['passed'] and result['provider_calls']==0
    before=json.loads((folder/'before.json').read_bytes())
    report={'version':'brief-posture/1','parent_snapshot':744,'runtime_changed':'native attention only',
            'images':{},'provider_calls':0,'owner_acceptance':False,
            'background_moving_frames_before':[672,871],'background_moving_frames_after':[139,871],
            'maximum_background_adjustment_seconds':.533,'mouth_sync_fixed':False}
    package=json.loads((ROOT/'.tools/paired-parallax-100/prepared-v2/package/manifest.json').read_bytes())
    report['package_hash']=package['package_hash']
    assert report['package_hash']=='eb1c1df5e15d773cab77da7f877f048447f3b2b1cffb6865524d69d645745da9'
    (folder/'binaries').mkdir(exist_ok=True)
    for name,image in [('before','syntelle-render-native:before-brief-posture-105'),('candidate','syntelle-render-native:brief-posture-105')]:
        report['images'][name]=subprocess.check_output(['docker','image','inspect',image,'--format','{{.Id}}'],text=True).strip()
        with (folder/'binaries'/name).open('wb') as output:
            subprocess.run(['docker','run','--rm','--pull','never','--network','none','--entrypoint','cat',image,'/usr/local/bin/syntelle-render-worker'],stdout=output,check=True)
    assert report['images']['before']==before['image']
    (folder/'activation.json').write_bytes(encoded(report))
    snapshot=db.archive_tree('brief-posture-105',folder,report)
    destination=ROOT/f'.tools/brief-posture-105-restored-{snapshot["snapshot_id"]}'
    count=db.restore(snapshot['snapshot_id'],destination)
    for path in destination.rglob('*'):
        if path.is_file():assert sha256(path.read_bytes()).digest()==sha256((folder/path.relative_to(destination)).read_bytes()).digest()
    receipt={**snapshot,'restored_files':count,'all_hashes_match':True}
    db.event('brief-posture-105','restoration-verified',receipt)
    (ROOT/'.tools/brief-posture-105-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__=='__main__':main()

"""Archive the offline expressive-articulation candidate and verify restoration."""
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.core.paths import ROOT
from app.preparation.archive import Archive


def main():
    folder=ROOT/'.tools/expressive-articulation-121'
    report=json.loads((folder/'validation-summary.json').read_bytes())
    assert report['provider_calls']==0
    for language in ('en-US','es-CO'):
        result=json.loads((folder/f'installed-{language}/report.json').read_bytes())
        assert result['technical_route_passed'] and result['cleanup_status']==204
    for relative in ('services/processor/app/workflows/motion/prepare_contextual_articulation.py',
                     'services/processor/app/audits/finalize_expressive_articulation.py',
                     'docs/experiments/motion/expressive-articulation.md',
                     'specs/004-coordinated-performance/spec.md',
                     'specs/004-coordinated-performance/plan.md',
                     'specs/004-coordinated-performance/tasks.md',
                     'specs/004-coordinated-performance/verify.md'):
        target=folder/'source'/relative
        target.parent.mkdir(parents=True,exist_ok=True)
        shutil.copyfile(ROOT/relative,target)
    parent=ROOT/'.tools/varied-presence-120/source'
    changed=[]
    for path in (folder/'source').rglob('*'):
        if not path.is_file():continue
        relative=path.relative_to(folder/'source');before=parent/relative
        if not before.exists() or path.read_bytes()!=before.read_bytes():
            changed.append({'file':relative.as_posix(),'sha256':sha256(path.read_bytes()).hexdigest()})
    (folder/'source-manifest.json').write_text(json.dumps(changed,indent=2))
    db=Archive()
    snapshot=db.archive_tree('expressive-articulation-121',folder,{**report,'parents':[802,803]})
    destination=ROOT/f'.tools/expressive-articulation-121-restored-{snapshot["snapshot_id"]}'
    count=db.restore(snapshot['snapshot_id'],destination)
    for path in destination.rglob('*'):
        if path.is_file():
            assert sha256(path.read_bytes()).digest()==sha256((folder/path.relative_to(destination)).read_bytes()).digest(),path
    receipt={**snapshot,'restored_files':count,'all_hashes_match':True,'provider_calls':0}
    db.event('expressive-articulation-121','restoration-verified',receipt)
    (ROOT/'.tools/expressive-articulation-121-archive.json').write_text(json.dumps(receipt,indent=2))
    print(json.dumps(receipt))


if __name__=='__main__':main()

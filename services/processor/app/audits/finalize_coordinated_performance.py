"""Freeze and restore the local performance candidate; never invoke providers."""
import json
import shutil
import subprocess
from hashlib import sha256

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder = ROOT / '.tools/coordinated-performance-103'
    db = Archive()
    identity = 'authorized-coordinated-serena-2026-10-27'
    with db.connect() as conn:
        operations = conn.execute('SELECT request_key,status,binding,request,result,diagnostics FROM syntelle_conversation.operations WHERE kind=%s ORDER BY id', (identity,)).fetchall()
    assert len(operations) == 11
    assert sum(row['request']['characters'] for row in operations) == 1131
    assert sum(row['status'] == 'received' for row in operations) == 10
    (folder / 'speech/complete-operations.json').write_bytes(db.checked(encoded(operations)))

    source = folder / 'final-source'
    groups = ['packages/render-contract', 'services/api/cmd', 'services/api/internal',
              'services/render-worker/cmd', 'services/render-worker/internal',
              'services/render-worker/scripts', 'services/processor/app', 'services/processor/tests',
              'apps/web/src', 'apps/web/config', 'specs/004-coordinated-performance']
    for name in groups:
        shutil.copytree(ROOT/name, source/name, dirs_exist_ok=True,
                        ignore=shutil.ignore_patterns('__pycache__', '*.pyc', 'node_modules', '.venv'))
    files = ['AGENTS.md', 'compose.yaml', 'docs/README.md',
             'docs/characters/coordinated-serena-experiment.json',
             'docs/development/coordinated-performance-plan.md',
             'docs/experiments/motion/coordinated-performance.md',
             'services/api/go.mod', 'services/api/go.sum',
             'services/render-worker/go.mod', 'services/render-worker/go.sum',
             'apps/web/package.json', 'apps/web/package-lock.json',
             'apps/web/vite.config.ts', 'services/processor/pyproject.toml']
    for name in files:
        if (ROOT/name).is_file():
            (source/name).parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT/name, source/name)

    unchanged = {}
    render = 'services/render-worker/internal/nativerender'
    for name in ['assets.go', 'renderer.go', 'gpu_kernel.cu', 'gpu_patches.cuh',
                 'gpu_types.h', 'reference_laughter.go', 'multiview.go']:
        original = (folder/'before-source'/render/name).read_bytes()
        assert (ROOT/render/name).read_bytes() == original, name
        unchanged[name] = sha256(original).hexdigest()
    images = {}
    binaries = folder / 'binaries'
    binaries.mkdir(exist_ok=True)
    for role, image, executable in [
        ('api', 'syntelle-api:coordinated-performance-103', 'syntelle-api'),
        ('worker', 'syntelle-render-native:coordinated-performance-103', 'syntelle-render-worker'),
        ('previous-api', 'syntelle-api:before-coordinated-performance-103', 'syntelle-api'),
        ('previous-worker', 'syntelle-render-native:before-coordinated-performance-103', 'syntelle-render-worker')]:
        images[role] = subprocess.check_output(['docker','image','inspect',image,'--format','{{.Id}}'], text=True).strip()
        with (binaries/role).open('wb') as target:
            subprocess.run(['docker','run','--rm','--pull','never','--network','none','--entrypoint','cat',image,'/usr/local/bin/'+executable], stdout=target, check=True)
    report = {'installed_local_review': True, 'owner_acceptance': False,
              'voice': 'en-US-Serena:DragonHDLatestNeural', 'explicit_styles': {'en-US':['reassuring','curious','amused'],'es-CO':[]},
              'laughter': {'en-US': True, 'es-CO': True},
              'attempts': 11, 'characters': 1131, 'image_requests': 0, 'analysis_requests': 0,
              'provider_failures': 0, 'qualification_complete': True, 'nonverbal_reconciled_without_retry': True, 'parent_snapshots': [676,678,679],
              'images': images, 'unchanged_renderer_sources': unchanged}
    (folder/'activation.json').write_bytes(encoded(report))
    package = ROOT/'.tools/paired-parallax-100/prepared-v2/package'
    resources = db.archive_tree('coordinated-performance-103/unchanged-resources', package)
    archive = db.archive_tree('coordinated-performance-103', folder, report)
    restored = ROOT / f'.tools/coordinated-performance-103-restored-{archive["snapshot_id"]}'
    count = db.restore(archive['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert sha256(path.read_bytes()).digest() == sha256((folder/path.relative_to(restored)).read_bytes()).digest()
    resources_restored = ROOT / f'.tools/coordinated-performance-103-resources-restored-{resources["snapshot_id"]}'
    resources_count = db.restore(resources['snapshot_id'], resources_restored)
    for path in resources_restored.rglob('*'):
        if path.is_file():
            assert sha256(path.read_bytes()).digest() == sha256((package/path.relative_to(resources_restored)).read_bytes()).digest()
    receipt = {**archive, 'restored_files': count, 'all_hashes_match': True,
               'unchanged_resources': {**resources, 'restored_files': resources_count, 'all_hashes_match': True}}
    db.event('coordinated-performance-103', 'restoration-verified', receipt)
    (ROOT/'.tools/coordinated-performance-103-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

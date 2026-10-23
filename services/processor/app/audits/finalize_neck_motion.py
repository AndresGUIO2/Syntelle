"""Archive a bounded neck/breath review; do not run historical experiment finalizers."""
import json
import argparse
import shutil
import xml.etree.ElementTree as ET

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.audits.finalize_projection_motion import verify_restoration
from app.core.paths import ROOT
from app.preparation.archive import Archive
from app.workflows.motion.prepare_projection_motion import verified_cache
from app.workflows.visemes.viseme_trial import write_report


def read(path):
    return json.loads(path.read_text())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--experiment', default='neck-motion-19')
    args = parser.parse_args()
    require(args.experiment.replace('-', '').isalnum(), 'Invalid experiment identifier.')
    root = ROOT/'.tools'/args.experiment
    runtime = ROOT/'apps/web/src/experiments/motion'
    cases = {}
    for name in ['syntelle', 'cafe', 'woodland']:
        folder = root/'prepared-current'/name
        p = read(folder/'package.json')
        record = read(folder/'record.json')
        require(verified_cache(folder, record['binding']), 'Neck cache changed.')
        for file, expected in record['algorithms'].items():
            require(digest(folder/file) == expected, 'Prepared source snapshot differs.')
        require(record['algorithms']['neck_rig.py'] == digest(ROOT/'services/processor/app/preparation/neck_rig.py') and
                record['algorithms']['prepare_neck_motion.py'] == digest(ROOT/'services/processor/app/workflows/motion/prepare_neck_motion.py'),
                'Prepared algorithm differs from current source.')
        parent = ROOT/'.tools/rgbd-motion-18/prepared-current'/name
        require(digest(parent/'package.json') == digest(runtime/'rgbd'/name/'package.json'),
                'Preserved RGB-D package changed.')
        previous = (parent/'projection.bin').read_bytes()
        require((folder/'projection.bin').read_bytes()[:len(previous)] == previous,
                'Existing depth/geometry data was replaced.')
        require(digest(folder/'package.json') == digest(runtime/'neck'/name/'package.json') and
                digest(folder/'projection.bin') == digest(runtime/'neck'/name/'projection.bin'),
                'Installed neck candidate differs.')
        require(record['new_provider_requests'] == 0 and p['projection']['articulation']['validation']['valid'],
                'Offline scope or geometry gate failed.')
        require(p['oral_build_id'] == read(parent/'package.json')['oral_build_id'], 'Oral identity changed.')
        cases[name] = {'build_id': p['build_id'], 'package_sha256': digest(folder/'package.json'),
                       'validation': p['projection']['articulation']['validation']}
    tests = ET.parse(root/'geometry-tests.xml').getroot()
    require(len(tests.findall('.//testcase')) == 3 and not tests.findall('.//failure') and
            not tests.findall('.//error'), 'Focused geometry checks failed.')
    for engine, count in [('chromium', 3), ('webkit', 1)]:
        results = read(root/'browser-review'/engine/'report.json')
        require(len(results) == count and all(not r['errors'] and r['new_provider_requests'] == 0 and
                r['faceBreathDelta'] == 0 and r['neutralDelta'] <= 1 and
                r['build_id'] == cases[r['character']]['build_id'] for r in results),
                'Bounded browser review failed.')
    source_snapshot(root/'source-snapshot')
    for p in [ROOT/'docs/characters/neck-inputs.json', ROOT/'docs/experiments/motion/neck-and-breathing.md',
              ROOT/'docs/README.md', *(runtime/'neck').glob('*/*')]:
        target = root/'source-snapshot'/p.relative_to(ROOT)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(p, target)
    write_report(root/'audit.json', {'cases': cases, 'new_provider_requests': 0,
        'reused_assets_archive': read(ROOT/'.tools/rgbd-motion-18-postgres-restore.json')['snapshot'],
        'verification_scope': 'Three geometry tests; Chromium three characters; WebKit Syntelle; frontend checks.',
        'full_regression_matrix_rerun': False, 'performance_remeasured': False, 'owner_acceptance': False})
    db = Archive()
    snapshot = db.archive_tree(args.experiment, root)
    restored = ROOT/'.tools'/f'{args.experiment}-restored'/str(snapshot['snapshot_id'])
    count = verify_restoration(db, snapshot, root, restored)
    result = {'snapshot': snapshot, 'restored_files': count, 'all_bytes_verified': True}
    db.event(args.experiment, 'verified-restoration', result)
    write_report(ROOT/'.tools'/f'{args.experiment}-postgres-restore.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()

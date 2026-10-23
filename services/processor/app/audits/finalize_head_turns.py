"""Archive and restore the rotation-only local candidate without provider calls."""
import json
import shutil

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.preparation.archive import Archive
from app.workflows.visemes.viseme_trial import ROOT, write_report


def main():
    root = ROOT / '.tools/head-motion-14'
    db = Archive()
    cases = {}
    for name in ['syntelle', 'cafe', 'woodland']:
        package = root / name / 'package.json'
        definition = json.loads(package.read_text())
        rig = definition['rig']
        require(rig['version'] == 'anatomical-head-rig/3' and rig['geometry_validation']['valid'],
                'Rotation geometry did not pass preparation.')
        runtime = ROOT / 'apps/web/src/experiments/motion'
        require(digest(package) == digest(runtime/'natural'/name/'package.json'),
                'Installed manifest differs from inspected rotation candidate.')
        require(digest(runtime/'natural-previous'/name/'package.json') ==
                digest(ROOT/'.tools/head-motion-12'/name/'package.json'),
                'Previous natural movement was not preserved.')
        unchanged = {}
        for file in ['foreground.webp', 'depth.webp', 'eyes/open.webp', 'eyes/closed.webp', 'eyes/mask.png']:
            current = runtime/'assets'/name/file
            require(digest(current) == digest(ROOT/'.tools/foreground-11/prepared'/name/file),
                    'Existing character resource changed.')
            unchanged[file] = digest(current)
        review = json.loads((root/name/'browser-review/report.json').read_text())
        require(len(review['checks']) == 1215 and all(review[k] == 0 for k in
                ['maximumOutsideEyeChange', 'maximumExposedIrisChange', 'maximumClosedError', 'maxEndpointJump']),
                'Rendering invariants failed.')
        cases[name] = {'package_sha256': digest(package), 'bounds': rig['bounds'],
                       'geometry': rig['geometry_validation'], 'unchanged': unchanged,
                       'oral_build_id': definition['oral_build_id']}
    renderer = 'apps/web/src/features/character/rendering/characterRenderer.ts'
    require(digest(ROOT/renderer) == digest(ROOT/'.tools/head-motion-13/before-source'/renderer),
            'Rotation-only correction unexpectedly changed the original renderer.')
    with db.connect() as conn:
        speech = conn.execute("SELECT count(*) AS requests,coalesce(sum(characters),0) AS characters FROM syntelle_speech.requests WHERE budget_id=%s",
                              ('authorized-dragon-motion-2026-10-19',)).fetchone()
        requests = conn.execute('SELECT budget_id,kind,count(*) AS requests FROM requests GROUP BY budget_id,kind ORDER BY budget_id,kind').fetchall()
    rejected = db.archive_tree('head-motion-13-rejected-lateral', ROOT/'.tools/head-motion-13',
                               {'accepted': False, 'reason': 'Owner requested yaw rotation, not lateral translation.'})
    source_snapshot(root/'source-snapshot')
    for path in (ROOT/'apps/web/src/experiments/motion').rglob('package.json'):
        target = root/'source-snapshot'/path.relative_to(ROOT)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, target)
    for name in ['docs/experiments/motion/head-turns.md', 'docs/experiments/motion/natural-head-speech.md']:
        target = root/'source-snapshot'/name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT/name, target)
    write_report(root/'audit.json', {'cases': cases, 'speech_usage_observed': speech,
        'image_analysis_usage_observed': requests, 'new_provider_calls_by_this_change': 0,
        'rejected_translation': rejected, 'original_renderer_sha256': digest(ROOT/renderer),
        'motion_trial_promoted_to_default': False, 'natural_candidate_updated': True,
        'owner_acceptance': False, 'real_device_validation': False})
    snapshot = db.archive_tree('head-motion-14-rotation', root)
    destination = ROOT/'.tools/head-motion-14-restored'/str(snapshot['snapshot_id'])
    restored = db.restore(snapshot['snapshot_id'], destination)
    for path in root.rglob('*'):
        if not path.is_file() or '__pycache__' in path.parts or path.suffix in ('.pyc', '.tmp'):
            continue
        require(digest(path) == digest(destination/path.relative_to(root)), 'Restored evidence differs.')
    result = {'snapshot': snapshot, 'restored_files': restored, 'all_bytes_verified': True}
    db.event('head-motion-14-rotation', 'verified-restoration', result)
    write_report(ROOT/'.tools/head-motion-14-postgres-restore.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()

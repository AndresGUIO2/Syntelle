"""Freeze and verify local speech-context motion; no providers or budget changes."""
import json
import shutil

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.preparation.archive import Archive
from app.workflows.visemes.viseme_trial import ROOT, write_report


def main():
    root = ROOT/'.tools/contextual-motion-15'
    db = Archive()
    settings = json.loads((ROOT/'docs/characters/contextual-motion.json').read_text())
    source_hash = digest(ROOT/'apps/web/src/features/character/animation/speechBehavior.ts')
    cases = {}
    runtime = ROOT/'apps/web/src/experiments/motion'
    for name in ['syntelle', 'cafe', 'woodland']:
        path = root/name/'package.json'
        package = json.loads(path.read_text())
        previous = ROOT/'.tools/head-motion-14'/name/'package.json'
        parent = json.loads(previous.read_text())
        require(package['rig'] == parent['rig'] and package['oral_build_id'] == parent['oral_build_id'],
                'Existing geometry or oral identity changed.')
        require(package['behavior'] == settings and package['behavior_source_sha256'] == source_hash,
                'Behavior settings/source binding differs.')
        require(digest(path) == digest(runtime/'natural'/name/'package.json') == digest(runtime/'contextual'/name/'package.json'),
                'Installed behavior differs from inspected preparation.')
        require(digest(previous) == digest(runtime/'natural-rotation'/name/'package.json'),
                'Previous rotation candidate was not preserved.')
        unchanged = {}
        for file in ['foreground.webp', 'depth.webp', 'eyes/open.webp', 'eyes/closed.webp', 'eyes/mask.png']:
            require(digest(runtime/'assets'/name/file) == digest(ROOT/'.tools/foreground-11/prepared'/name/file),
                    'Existing image resource changed.')
            unchanged[file] = digest(runtime/'assets'/name/file)
        report = json.loads((root/name/'browser-review/report.json').read_text())
        require(len(report['checks']) == 1215 and all(report[k] == 0 for k in
                ['maximumOutsideEyeChange', 'maximumExposedIrisChange', 'maximumClosedError', 'maxEndpointJump']),
                'Runtime image invariants failed.')
        cases[name] = {'build_id': package['build_id'], 'package_sha256': digest(path), 'unchanged': unchanged}
    renderer = 'apps/web/src/features/character/rendering/characterRenderer.ts'
    require(digest(ROOT/renderer) == digest(ROOT/'.tools/head-motion-13/before-source'/renderer),
            'The shared renderer changed unexpectedly.')
    tests = json.loads((root/'installed-browser-results.json').read_text())['stats']
    require(tests['unexpected'] == tests['flaky'] == tests['skipped'] == 0 and tests['expected'] == 84,
            'Installed browser regressions did not pass.')
    speech = json.loads((root/'cached-speech-review.json').read_text())
    require(len(speech) == 6 and all(not row['errors'] and row['new_provider_requests'] == 0 for row in speech),
            'Cached speech review did not pass.')
    dependencies = db.archive_tree('contextual-motion-15/cached-speech', ROOT/'.tools/head-motion-12/speech')
    with db.connect() as conn:
        usage = conn.execute("SELECT count(*) AS requests,coalesce(sum(characters),0) AS characters FROM syntelle_speech.requests WHERE budget_id=%s",
                             ('authorized-dragon-motion-2026-10-19',)).fetchone()
    source_snapshot(root/'source-snapshot')
    for source in [ROOT/'docs/experiments/motion/contextual-motion.md', ROOT/'docs/characters/contextual-motion.json',
                   *(runtime/'natural').glob('*/package.json'), *(runtime/'contextual').glob('*/package.json'),
                   *(runtime/'natural-rotation').glob('*/package.json')]:
        target = root/'source-snapshot'/source.relative_to(ROOT)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
    write_report(root/'audit.json', {'cases': cases, 'cached_speech_dependency': dependencies,
        'speech_usage_observed': usage, 'new_provider_calls_by_this_change': 0,
        'installed_browser_tests': tests, 'behavior_version': settings['version'],
        'natural_candidate_updated': True, 'default_motion_trial_promoted': False,
        'owner_acceptance': False, 'real_device_validated': False})
    archived = db.archive_tree('contextual-motion-15', root)
    destination = ROOT/'.tools/contextual-motion-15-restored'/str(archived['snapshot_id'])
    count = db.restore(archived['snapshot_id'], destination)
    for path in root.rglob('*'):
        if not path.is_file() or '__pycache__' in path.parts or path.suffix in ('.pyc', '.tmp'):
            continue
        require(digest(path) == digest(destination/path.relative_to(root)), 'Restored evidence differs.')
    result = {'snapshot': archived, 'restored_files': count, 'all_bytes_verified': True}
    db.event('contextual-motion-15', 'verified-restoration', result)
    write_report(ROOT/'.tools/contextual-motion-15-postgres-restore.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()

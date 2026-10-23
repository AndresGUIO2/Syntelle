"""Archive and restore asymmetric breathing/double-blink evidence without providers."""
import json
import shutil
import xml.etree.ElementTree as ET

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.core.paths import ROOT
from app.preparation.archive import Archive
from app.preparation.head_rig import validate
from app.workflows.visemes.viseme_trial import write_report


def main():
    root = ROOT/'.tools/vitality-motion-17'
    runtime = ROOT/'apps/web/src/experiments/motion'
    settings = json.loads((ROOT/'docs/characters/vitality-motion.json').read_text())
    db = Archive()
    cases = {}
    for name in ['syntelle', 'cafe', 'woodland']:
        path = root/name/'package.json'
        package = json.loads(path.read_text())
        previous = ROOT/'.tools/contextual-motion-15'/name/'package.json'
        parent = json.loads(previous.read_text())
        require(package['rig'] == parent['rig'] and package['oral_build_id'] == parent['oral_build_id'],
                'Existing geometry/oral identity changed.')
        require(package['animation_seed'] == int(parent['build_id'][:8], 16), 'Head animation seed changed.')
        require(package['behavior_parent_sha256'] == digest(previous), 'Parent binding differs.')
        require(package['breathing'] == settings['breathing'], 'Breathing configuration differs.')
        require(package['behavior'] == {k:v for k,v in settings.items() if k != 'breathing'},
                'Behavior configuration differs.')
        for file, expected in package['vitality_sources_sha256'].items():
            require(digest(ROOT/'apps/web/src/features/character'/file) == expected, 'Source binding differs.')
        require(digest(path) == digest(runtime/'natural'/name/'package.json') ==
                digest(runtime/'vitality'/name/'package.json'), 'Installed manifest differs.')
        require(digest(previous) == digest(runtime/'contextual'/name/'package.json'),
                'Previous behavior was not preserved.')
        unchanged = {}
        for file in ['foreground.webp', 'depth.webp', 'eyes/open.webp', 'eyes/closed.webp', 'eyes/mask.png']:
            source = runtime/'assets'/name/file
            require(digest(source) == digest(ROOT/'.tools/foreground-11/prepared'/name/file),
                    'Existing image resource changed.')
            unchanged[file] = digest(source)
        require(0 < package['motion']['breath_face_height_fraction'] <= .006,
                'Breathing exceeds validated displacement.')
        rig = package['rig']
        geometry = validate(rig['vertices'], rig['pivot'],
                            [package['canvas']['width'], package['canvas']['height']], rig['grid'],
                            rig['bounds']['speaking'], package['anatomy']['face_height'])
        require(geometry['valid'], 'Motion geometry inverts.')
        write_report(root/name/'geometry-validation.json', geometry)
        report = json.loads((root/name/'browser-review/report.json').read_text())
        require(len(report['checks']) == 1215 and all(report[k] == 0 for k in
                ['maximumOutsideEyeChange', 'maximumExposedIrisChange', 'maximumClosedError', 'maxEndpointJump']),
                'Rendered image invariants failed.')
        cases[name] = {'build_id': package['build_id'], 'package_sha256': digest(path),
                       'unchanged': unchanged, 'geometry': geometry}
    tests = json.loads((root/'installed-browser-results.json').read_text())['stats']
    require(tests['unexpected'] == tests['flaky'] == tests['skipped'] == 0 and tests['expected'] == 136,
            'Installed browser regressions did not pass.')
    processor = ET.parse(root/'processor-tests.xml').getroot()
    require(len(processor.findall('.//testcase')) == 160 and
            not processor.findall('.//failure') and not processor.findall('.//error'), 'Processor tests failed.')
    speech = json.loads((root/'cached-speech-review.json').read_text())
    require(len(speech) == 6 and all(not row['errors'] and row['new_provider_requests'] == 0 for row in speech),
            'Cached speech playback failed.')
    dependencies = {
        'motion_assets': db.archive_tree('vitality-motion-17/reused-motion-assets', runtime/'assets'),
        'speech': db.archive_tree('vitality-motion-17/cached-speech', ROOT/'.tools/head-motion-12/speech'),
    }
    source_snapshot(root/'source-snapshot')
    for source in [ROOT/'docs/characters/vitality-motion.json', ROOT/'docs/experiments/motion/vitality-motion.md',
                   ROOT/'docs/README.md', *(runtime/'natural').glob('*/package.json'),
                   *(runtime/'vitality').glob('*/package.json'), *(runtime/'contextual').glob('*/package.json')]:
        target = root/'source-snapshot'/source.relative_to(ROOT)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
    write_report(root/'audit.json', {'cases': cases, 'dependencies': dependencies,
        'new_provider_calls_by_this_change': 0, 'installed_browser_tests': tests,
        'behavior_version': settings['version'], 'breathing_version': settings['breathing']['version'],
        'natural_candidate_updated': True, 'default_motion_trial_promoted': False,
        'owner_acceptance': False, 'real_device_validated': False})
    archived = db.archive_tree('vitality-motion-17', root)
    destination = ROOT/'.tools/vitality-motion-17-restored'/str(archived['snapshot_id'])
    count = db.restore(archived['snapshot_id'], destination)
    for path in root.rglob('*'):
        if not path.is_file() or '__pycache__' in path.parts or path.suffix in ('.pyc', '.tmp'):
            continue
        require(digest(path) == digest(destination/path.relative_to(root)), 'Restored evidence differs.')
    result = {'snapshot': archived, 'restored_files': count, 'all_bytes_verified': True}
    db.event('vitality-motion-17', 'verified-restoration', result)
    write_report(ROOT/'.tools/vitality-motion-17-postgres-restore.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()

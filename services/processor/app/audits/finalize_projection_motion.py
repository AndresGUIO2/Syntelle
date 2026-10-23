"""Verify and archive the offline RGB-D candidate and its complete comparison evidence."""
from hashlib import sha256
import json
import shutil
import xml.etree.ElementTree as ET

import numpy as np

from app.audits.finalize_refinement import digest, require, source_snapshot
from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.preparation.projection_rig import SETTINGS, VERSION, project, validate
from app.workflows.motion.prepare_projection_motion import verified_cache
from app.workflows.visemes.viseme_trial import write_report


def read(path):
    return json.loads(path.read_text())


def verify_restoration(db, snapshot, source, destination):
    count = db.restore(snapshot['snapshot_id'], destination)
    for path in source.rglob('*'):
        if not path.is_file() or '__pycache__' in path.parts or path.suffix in ('.pyc', '.tmp'):
            continue
        require(digest(path) == digest(destination/path.relative_to(source)), 'Restored evidence differs.')
    return count


def performance(root):
    results = []
    for engine in ['chromium', 'webkit']:
        baseline = read(root/'performance/controlled/matched-legacy'/f'performance-{engine}.json')
        candidate = read(root/'performance/final/perspective'/f'performance-{engine}.json')
        require(len(baseline) == len(candidate) == 6, 'Incomplete serial performance sample.')
        for a, b in zip(baseline, candidate):
            require((a['character'], a['viewport']) == (b['character'], b['viewport']),
                    'Performance workloads differ.')
            ratio = b['average_fps']/a['average_fps']
            require(ratio >= .95, 'Material performance regression: inspect the preserved samples.')
            require(b['average_fps'] >= (50 if b['viewport'] == 'desktop' else 30),
                    'Local rendering is below the accepted candidate floor.')
            results.append({'engine': engine, 'character': b['character'], 'viewport': b['viewport'],
                'baseline_fps': a['average_fps'], 'candidate_fps': b['average_fps'],
                'ratio': ratio, 'p95_frame_ms': b['p95_frame_ms'],
                'strict_target_met': b['average_fps'] >= (60 if b['viewport'] == 'desktop' else 30)})
    return results


def main():
    root = ROOT/'.tools/rgbd-motion-18'
    runtime = ROOT/'apps/web/src/experiments/motion'
    db = Archive()
    manifest = read(ROOT/'docs/characters/projection-inputs.json')
    inspection = read(root/'visual-review/inspection.json')
    require(inspection['internal_visual_review'] and not inspection['owner_acceptance'],
            'Missing internal visual inspection record.')
    for file in ['rendering/characterRenderer.ts', 'animation/speechBehavior.ts',
                 'animation/breathing.ts', 'composables/useMotionPlayback.ts']:
        source = 'apps/web/src/features/character/'+file
        require(digest(ROOT/source) == digest(root/'before-source'/source),
                'Preserved renderer or behavior source changed.')
    cases = {}
    for item in manifest['characters']:
        name = item['id']
        folder = root/'prepared-current'/name
        path = folder/'package.json'
        package, record = read(path), read(folder/'record.json')
        require(verified_cache(folder, record['binding']), 'Preparation cache differs.')
        require(record['settings'] == SETTINGS and record['version'] == VERSION and
                record['new_provider_requests'] == 0, 'Preparation settings or offline scope differ.')
        require(record['implementation_sha256'] == digest(ROOT/'services/processor/app/preparation/projection_rig.py'),
                'Prepared implementation differs from current source.')
        require(digest(folder/'prepare_projection_motion.py') ==
                digest(ROOT/'services/processor/app/workflows/motion/prepare_projection_motion.py'),
                'Prepared workflow differs from current source.')
        for key, expected in record['input_hashes'].items():
            require(digest((ROOT/item[key]).resolve()) == expected, 'Immutable input bytes differ.')
        binding = sha256(encoded({'inputs': record['input_hashes'], 'settings': SETTINGS,
            'implementation': digest(folder/'projection_rig.py'),
            'workflow': digest(folder/'prepare_projection_motion.py')})).hexdigest()
        require(binding == record['binding'], 'Source/settings binding differs.')
        parent = read((ROOT/item['package']).resolve())
        retained = {k: v for k, v in package.items() if k not in ('projection', 'build_id')}
        require(retained == {k: v for k, v in parent.items() if k != 'build_id'},
                'Existing animation or oral identity changed.')
        oral = runtime.parent/'viseme-a/characters'/name
        require(read(oral/'package.json')['build_id'] == package['oral_build_id'],
                'Installed oral package differs from the referenced build.')
        require(digest(path) == digest(runtime/'natural'/name/'package.json') ==
                digest(runtime/'rgbd'/name/'package.json'), 'Installed candidate differs.')
        require(digest((ROOT/item['package']).resolve()) == digest(runtime/'vitality'/name/'package.json'),
                'Previous candidate was not preserved.')
        unchanged = {}
        for file in ['foreground.webp', 'depth.webp', 'eyes/open.webp', 'eyes/closed.webp', 'eyes/mask.png']:
            original = ROOT/'.tools/foreground-11/prepared'/name/file
            require(digest(original) == digest(runtime/'assets'/name/file), 'Reused image changed.')
            unchanged[file] = digest(original)
        rig = package['projection']
        require(rig['source_sha256'] == package['original_sha256'] and
                rig['parent_build_id'] == parent['build_id'] and not rig['manual_overrides'] and rig['ready'],
                'Invalid source/parent identity or manual override.')
        require(rig['grid'] <= 128 and rig['bounds'] == {'yaw': 3., 'pitch': 1.2, 'roll': .3},
                'Unexpected final movement envelope.')
        require(digest(folder/'projection.bin') == digest(runtime/'rgbd'/name/'projection.bin'),
                'Installed geometry differs.')
        raw = (folder/'projection.bin').read_bytes()
        count = rig['geometry']['vertices']
        vertices = np.frombuffer(raw, dtype='<f4', count=count*5).reshape(count, 5)
        indices = np.frombuffer(raw, dtype='<u2', count=rig['geometry']['triangles']*3, offset=count*20)
        geometry = validate(vertices, indices, rig['camera'], rig['pivot'],
            list(rig['bounds'].values()), package['anatomy']['face_height'])
        require(geometry['valid'] and geometry['minimum_area_ratio'] > .25, 'Geometry validation failed.')
        landmarks = read(folder/'inputs/landmarks.json')['landmarks_px']
        ids = [1, 234, 454]
        xy = np.asarray(landmarks)[ids, :2]
        uv = xy/np.asarray(rig['camera']['size'])
        grid = np.clip(np.rint(uv*rig['grid']).astype(int), 0, rig['grid'])
        sampled = vertices[grid[:, 1]*(rig['grid']+1)+grid[:, 0]]
        neutral, _ = project(sampled, rig['camera'], rig['pivot'], [0, 0, 0])
        turned, _ = project(sampled, rig['camera'], rig['pivot'], [3, 0, 0])
        displacement = turned-neutral
        relative = float(np.ptp(displacement[:, 0]))
        require(relative > package['anatomy']['face_width']*.001,
                'Turn behaves like a global displacement rather than depth-relative motion.')
        report = read(root/name/'browser-review/report.json')
        require(report['build_id'] == package['build_id'] and
                report['source_geometry_sha256'] == rig['geometry']['sha256'], 'Export source binding differs.')
        require(len(report['checks']) == 1215 and len(report['projectionChecks']) == 99 and
                all(report[k] == 0 for k in ['maximumOutsideEyeChange', 'maximumExposedIrisChange',
                    'maximumClosedError', 'maxEndpointJump']), 'Rendered image invariants failed.')
        require(inspection['characters'][name]['package_sha256'] == digest(path), 'Visual review is stale.')
        cases[name] = {'build_id': package['build_id'], 'package_sha256': digest(path),
            'geometry': geometry, 'unchanged_images': unchanged, 'preparation_seconds': record['seconds'],
            'nose_left_right_cheek_displacement_px': displacement.tolist(),
            'relative_horizontal_motion_px_at_3_degrees': relative}
    tests = read(root/'installed-browser-results.json')['stats']
    require(tests['unexpected'] == tests['flaky'] == tests['skipped'] == 0 and tests['expected'] == 176,
            'Installed browser regressions did not pass.')
    processor = ET.parse(root/'processor-tests.xml').getroot()
    require(len(processor.findall('.//testcase')) == 167 and not processor.findall('.//failure') and
            not processor.findall('.//error'), 'Processor tests failed.')
    speech = read(root/'cached-speech-review.json')
    require(len(speech) == 6 and all(not row['errors'] and row['new_provider_requests'] == 0 for row in speech),
            'Cached speech playback failed.')
    dependencies = {}
    for key, folder in {'motion_assets': runtime/'assets',
                           'oral_assets': runtime.parent/'viseme-a/characters',
                           'backgrounds': runtime/'backgrounds',
                           'speech': ROOT/'.tools/head-motion-12/speech'}.items():
        snapshot = db.archive_tree('rgbd-motion-18/reused-'+key, folder)
        restored = ROOT/'.tools/rgbd-motion-18-restored/dependencies'/key/str(snapshot['snapshot_id'])
        count = verify_restoration(db, snapshot, folder, restored)
        dependencies[key] = {'snapshot': snapshot, 'restored_files': count, 'all_bytes_verified': True}
    source_snapshot(root/'source-snapshot')
    for source in [ROOT/'docs/characters/projection-inputs.json',
                   ROOT/'docs/experiments/motion/depth-aware-parallax.md', ROOT/'docs/README.md',
                   *(runtime/'natural').glob('*/package.json'), *(runtime/'vitality').glob('*/package.json'),
                   *(runtime/'rgbd').glob('*/*')]:
        if not source.is_file():
            continue
        target = root/'source-snapshot'/source.relative_to(ROOT)
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
    write_report(root/'audit.json', {'cases': cases, 'dependencies': dependencies,
        'new_provider_calls_by_this_change': 0, 'installed_browser_tests': tests,
        'projection_version': VERSION, 'performance': performance(root),
        'natural_candidate_updated': True, 'default_motion_trial_promoted': False,
        'owner_acceptance': False, 'real_device_validated': False})
    archived = db.archive_tree('rgbd-motion-18', root)
    destination = ROOT/'.tools/rgbd-motion-18-restored'/str(archived['snapshot_id'])
    count = verify_restoration(db, archived, root, destination)
    result = {'snapshot': archived, 'restored_files': count, 'all_bytes_verified': True}
    db.event('rgbd-motion-18', 'verified-restoration', result)
    write_report(ROOT/'.tools/rgbd-motion-18-postgres-restore.json', result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()

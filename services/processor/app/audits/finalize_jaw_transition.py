"""Archive the focused jaw-skin transition candidate and byte-verify restoration.

This command performs no provider calls. Run only after rendered identity,
matched cached playback and local installation checks have completed.
"""
import base64
from hashlib import sha256
import json
import shutil
import subprocess

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


EXPERIMENT = 'continuous-jaw-transition-113'
PACKAGE_HASH = '70bf36a1d067c92c72d2418a7a77ae74faa9bedb6dd4f1498d40bc428afe4f0b'
IMAGE_ID = 'sha256:e83c2bc5ec0de7ca3d044e3add0ce1247563fda9ae8b8bda020628fa98762bb9'
SOURCE_DELTA = {
    'packages/render-contract/speech_jaw.go',
    'services/render-worker/internal/nativerender/speech_jaw.go',
    'services/render-worker/internal/nativerender/speech_jaw_test.go',
}


def read(path):
    return json.loads(path.read_bytes())


def copy_source(folder, name):
    target = folder / 'preparation-source' / name
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(ROOT / name, target)


def check_playback(folder, name, package_hash, expected_image):
    playback = read(folder / name / 'report.json')
    assert playback['technical_route_passed'] and playback['cleanup_status'] == 204, name
    assert playback['provider_calls'] == playback['microphone_calls'] == 0, name
    assert not playback['errors'] and playback['worker_image_id'] == expected_image, name
    presentation = playback['presentation']
    assert presentation['rate'] == 1 and sorted(presentation['tracks']) == ['audio', 'video'], name
    ready = [event['data'] for event in playback['events'] if event.get('type') == 'ready']
    assert ready and all(value['package_hash'] == package_hash for value in ready), name
    return playback


def main():
    folder = ROOT / '.tools/jaw-transition-113'
    previous = ROOT / '.tools/presence-continuity-112'
    manifest = read(folder / 'prepared-source-bound/package/manifest.json')
    base = read(previous / 'speech-jaw-protected/package/manifest.json')
    original = read(ROOT / '.tools/paired-parallax-100/prepared-v2/package/manifest.json')
    profile = read(folder / 'profile/profile.json')
    old_profile = read(previous / 'profile/profile.json')
    assert manifest['package_hash'] == PACKAGE_HASH
    assert manifest['textures'] == base['textures'] == original['textures']
    assert manifest['geometry'] == base['geometry'] == original['geometry']
    for name in ['oral', 'expressions', 'gaze']:
        assert manifest['packages'][name] == base['packages'][name] == original['packages'][name], name
    motion, old_motion = dict(manifest['packages']['motion']), dict(base['packages']['motion'])
    rig, old_rig = motion.pop('speech_jaw'), old_motion.pop('speech_jaw')
    assert motion == old_motion == original['packages']['motion']
    assert rig['version'] == 'speech-jaw/2' and rig['maximum_chin_fraction'] == .0075
    for name in ['source_sha256', 'motion_build_id', 'oral_build_id', 'geometry_sha256',
                 'hinge', 'chin', 'maximum_angle_radians', 'pose_opening',
                 'protected_oral_crop', 'lower_neck_y', 'maximum_chin_fraction']:
        assert rig[name] == old_rig[name], name
    assert rig['oral_support']['version'] == 'calibrated-outer-lip-support/1'
    assert rig['validation']['valid'] and rig['validation']['minimum_area_ratio'] > .25
    assert rig['validation']['maximum_neck_weight'] == rig['validation']['maximum_oral_weight'] == 0
    assert all(item['weight'] > 0 for item in rig['validation']['center_bridge'])
    assert profile['package_hash'] == manifest['package_hash']
    assert {key: value for key, value in profile.items() if key != 'package_hash'} == {
        key: value for key, value in old_profile.items() if key != 'package_hash'}
    assert profile['voice'] == 'en-US-Emma:DragonHDLatestNeural'
    assert set(profile['laughter']) == {
        'en-US-Emma:DragonHDLatestNeural|en-US', 'en-US-Emma:DragonHDLatestNeural|es-CO'}

    # The recorded source check compares against frozen phase112 and verifies
    # the deformation and speech-clock methods themselves remain byte-identical.
    source = read(folder / 'source-equivalence.json')
    assert source['passed'] and source['baseline_snapshot'] == 772
    assert set(source['changed']) == SOURCE_DELTA and source['motion_functions_unchanged']
    assert source['provider_calls'] == 0
    implementation = read(folder / 'design/implementation.json')
    assert implementation['package_hash'] == PACKAGE_HASH
    assert implementation['provider_model_image_requests'] == 0
    assert not implementation['hinge_angle_amplitude_timing_changed']
    field = read(folder / 'final-field-verification.json')
    assert field['passed'] and field['provider_calls'] == 0
    before, after = field['cases']
    assert before['package_hash'] == base['package_hash'] and before['moving_components'] == 3
    assert after['package_hash'] == PACKAGE_HASH and after['moving_components'] == 1
    assert before['maximum_angle_radians'] == after['maximum_angle_radians'] == rig['maximum_angle_radians']
    assert before['maximum_chin_fraction'] == after['maximum_chin_fraction'] == .0075

    render = read(folder / 'render-verification.json')
    assert render['passed'] and render['provider_calls'] == 0
    assert render['actual_support_from_cached_sources']
    assert render['minimum_cpu_cuda_psnr_db'] >= 45
    cases = {item['name']: item for item in render['cases']}
    states = read(folder / 'render-states.json')
    assert set(cases) == {item['Name'] for item in states}
    for item in states:
        name, state = item['Name'], item['State']
        case = cases[name]
        assert case['cpu_cuda_psnr_db'] >= 45, name
        inactive = state.get('SpeechJaw', 0) == 0 or state.get('Laugh', 0) > 0
        if inactive:
            assert case['full107_pixels_equal'], name
        if state.get('head', [0, 0, 0]) == [0, 0, 0]:
            assert case['actual_lip_and_enamel_pixels_equal'], name
            assert case['lower_neck_clothing_equal'], name
            if not inactive:
                assert case['middle_skin_changed_pixels'] > 0, name
    for name in ['rest', 'manual-smile', 'laugh-1', 'laugh-0.5', 'laugh-0.1', 'laugh-0.01', 'laugh-boundary']:
        assert cases[name]['full107_pixels_equal'], name

    expected = subprocess.check_output(
        ['docker', 'image', 'inspect', 'syntelle-render-native:jaw-transition-113', '--format', '{{.Id}}'],
        text=True).strip()
    assert expected == IMAGE_ID
    worker = read_container('syntelle-render-native-local')
    assert worker['Image'] == expected and worker['State']['Running']
    labels = worker['Config']['Labels']
    assert labels['io.syntelle.project'] == 'syntelle' and labels['io.syntelle.role'] == 'render-worker'
    volume = read(folder / 'install-volume-verification.json')
    assert volume['all_hashes_match'] and volume['package_hash'] == PACKAGE_HASH
    package_files = [path for path in (folder / 'prepared-source-bound/package').rglob('*') if path.is_file()]
    assert volume['files'] == len(package_files) == 120
    assert any(mount['Type'] == 'volume' and mount['Name'] == volume['volume']
               for mount in worker['Mounts'])
    for language in ['en-US', 'es-CO']:
        playback = check_playback(folder, 'candidate-fresh-' + language, PACKAGE_HASH, expected)
        assert playback['language'] == language
        assert {item['name'] for item in playback['cases']} == {
            'cached-speaking-head-disabled', 'complete-original-laughter-playful100'}
    check_playback(folder, 'installed-en-US', PACKAGE_HASH, expected)
    review = read(folder / 'browser-review-receipt.json')
    assert review['experiment'] == 'jaw-transition-113'
    assert review['visual_inspection']['inspected'] and review['visual_inspection']['limitations']
    assert {item['folder'] for item in review['reviews']} == {
        'baseline-en-US', 'baseline-es-CO', 'candidate-fresh-en-US', 'candidate-fresh-es-CO'}
    for item in review['reviews']:
        assert not item['errors'] and item['cleanup_status'] == 204
        assert item['provider_calls'] == item['microphone_calls'] == 0
        assert item['rate'] == 1 and sorted(item['tracks']) == ['audio', 'video']

    inputs = folder / 'inputs'
    inputs.mkdir(exist_ok=True)
    landmarks = ROOT / '.tools/foreground-11/prepared/syntelle/base-landmarks.json'
    assert sha256(landmarks.read_bytes()).hexdigest() == rig['provenance']['landmarks_sha256']
    shutil.copy2(landmarks, inputs / 'base-landmarks.json')
    shutil.copy2(previous / 'speech-jaw-protected/package/manifest.json', inputs / 'baseline112-manifest.json')
    shutil.copy2(ROOT / '.tools/paired-parallax-100/prepared-v2/package/manifest.json',
                 inputs / 'original107-manifest.json')
    shutil.copytree(previous / 'profile', inputs / 'baseline112-profile', dirs_exist_ok=True)
    for name in ['emma-presence-107-archive.json', 'laughter-restore-111-archive.json',
                 'presence-continuity-112-archive.json']:
        shutil.copy2(ROOT / '.tools' / name, inputs / name)
    for name in ['AGENTS.md', 'docs/README.md', 'docs/experiments/motion/continuous-jaw-transition.md',
                 'services/processor/app/workflows/motion/prepare_speech_jaw.py',
                 'services/processor/app/workflows/motion/prepare_mandible.py',
                 'services/processor/tests/test_prepare_speech_jaw.py',
                 'services/processor/app/audits/finalize_jaw_transition.py',
                 'specs/004-coordinated-performance/spec.md', 'specs/004-coordinated-performance/plan.md',
                 'specs/004-coordinated-performance/tasks.md', 'specs/004-coordinated-performance/verify.md']:
        copy_source(folder, name)
    binaries = folder / 'binaries'
    binaries.mkdir(exist_ok=True)
    for name in ['syntelle-render-worker', 'syntelle-render-frame', 'native-render-tests']:
        with (binaries / name).open('wb') as output:
            subprocess.run(['docker', 'run', '--rm', '--pull', 'never', '--network', 'none',
                            '--entrypoint', 'cat', 'syntelle-render-native:jaw-transition-113',
                            '/usr/local/bin/' + name], stdout=output, check=True)
    report = {
        'version': 'continuous-jaw-transition/1', 'worker_image': expected,
        'package_hash': PACKAGE_HASH, 'baseline_snapshot': 772,
        'original_runtime_snapshot': 766, 'original_resources_snapshot': 730,
        'restoration_snapshot': 771, 'speech_jaw_resource': rig['version'],
        'speech_jaw_maximum_fraction': rig['maximum_chin_fraction'],
        'lip_enamel_and_laughter_preserved': True, 'lower_neck_and_clothing_preserved': True,
        'head_idle_speech_timing_unchanged_from112': True,
        'connected_skin_components_before_after': [3, 1],
        'installed_asset_volume': volume,
        'minimum_cpu_cuda_psnr_db': render['minimum_cpu_cuda_psnr_db'],
        'audio_sha256': {key: sha256(base64.b64decode(value['audio_base64'])).hexdigest()
                         for key, value in profile['laughter'].items()},
        'provider_calls': 0, 'image_calls': 0, 'analysis_calls': 0, 'synthesis_calls': 0,
        'owner_naturalness': 'pending', 'new_performance_qualification': False,
        'scope': 'connected lower-face skin outside protected calibrated lip support',
    }
    (folder / 'activation.json').write_bytes(encoded(report))
    archive = Archive()
    result = archive.archive_tree(EXPERIMENT, folder, report)
    restored = ROOT / ('.tools/jaw-transition-113-restored-' + str(result['snapshot_id']))
    count = archive.restore(result['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert path.read_bytes() == (folder / path.relative_to(restored)).read_bytes(), path
    receipt = {**result, 'restored_files': count, 'all_hashes_match': True,
               'package_hash': PACKAGE_HASH, 'baseline_snapshot': 772}
    archive.event(EXPERIMENT, 'restoration-verified', receipt)
    (ROOT / '.tools/jaw-transition-113-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


def read_container(name):
    # Read only what the finalizer needs; credentials are never copied to evidence.
    return json.loads(subprocess.check_output(['docker', 'inspect', name], text=True))[0]


if __name__ == '__main__':
    main()

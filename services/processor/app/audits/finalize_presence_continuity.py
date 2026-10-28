"""Archive the scoped phase107-based speech/idle candidate and verify restoration."""
import json
import shutil
import subprocess

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder = ROOT / '.tools/presence-continuity-112'
    manifest = json.loads((folder / 'speech-jaw-protected/package/manifest.json').read_bytes())
    base = json.loads((ROOT / '.tools/paired-parallax-100/prepared-v2/package/manifest.json').read_bytes())
    profile = json.loads((folder / 'profile/profile.json').read_bytes())
    old_profile = json.loads((ROOT / '.tools/emma-presence-107/profile-v3/profile.json').read_bytes())
    assert manifest['textures'] == base['textures'] and manifest['geometry'] == base['geometry']
    for name in ['oral', 'expressions', 'gaze']:
        assert manifest['packages'][name] == base['packages'][name], name
    motion = dict(manifest['packages']['motion'])
    rig = motion.pop('speech_jaw')
    assert motion == base['packages']['motion']
    assert rig['version'] == 'speech-jaw/1' and rig['maximum_chin_fraction'] == .0075
    assert profile['package_hash'] == manifest['package_hash']
    assert profile['laughter'] == old_profile['laughter']
    assert profile['voice'] == old_profile['voice'] == 'en-US-Emma:DragonHDLatestNeural'
    source = json.loads((folder / 'source-equivalence.json').read_bytes())
    render = json.loads((folder / 'render-verification.json').read_bytes())
    assert source['passed'] and render['passed']
    assert all(v.get('actual107_full_pixels_equal', True) and v.get('actual107_mouth_pixels_equal', True)
               and v.get('lower_neck_and_clothing_pixels_equal', True) for v in render['cases'])
    worker = json.loads(subprocess.check_output(['docker', 'inspect', 'syntelle-render-native-local'], text=True))[0]
    expected = subprocess.check_output(['docker', 'image', 'inspect', 'syntelle-render-native:presence-continuity-112',
                                        '--format', '{{.Id}}'], text=True).strip()
    assert worker['Image'] == expected and worker['State']['Running']
    for name in ['candidate-compiled-en-US', 'candidate-compiled-es-CO']:
        playback = json.loads((folder / name / 'report.json').read_bytes())
        assert playback['technical_route_passed'] and playback['cleanup_status'] == 204
        assert playback['provider_calls'] == playback['microphone_calls'] == 0
        assert playback['worker_image_id'] == expected
    installed = json.loads((folder / 'installed-en-US/report.json').read_bytes())
    assert installed['technical_route_passed'] and installed['cleanup_status'] == 204
    inputs = folder / 'inputs'
    inputs.mkdir(exist_ok=True)
    shutil.copy2(ROOT / '.tools/foreground-11/prepared/syntelle/base-landmarks.json', inputs / 'base-landmarks.json')
    for name in ['emma-presence-107-archive.json', 'laughter-restore-111-archive.json']:
        shutil.copy2(ROOT / '.tools' / name, inputs / name)
    shutil.copytree(ROOT / '.tools/emma-presence-107/profile-v3', inputs / 'baseline-profile', dirs_exist_ok=True)
    for name in ['AGENTS.md', 'docs/experiments/motion/presence-continuity.md',
                 'services/processor/app/workflows/motion/prepare_speech_jaw.py',
                 'services/processor/app/workflows/motion/prepare_mandible.py',
                 'services/processor/tests/test_prepare_speech_jaw.py',
                 'services/processor/app/audits/finalize_presence_continuity.py',
                 'specs/004-coordinated-performance/spec.md', 'specs/004-coordinated-performance/plan.md',
                 'specs/004-coordinated-performance/tasks.md', 'specs/004-coordinated-performance/verify.md']:
        target = folder / 'preparation-source' / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, target)
    binaries = folder / 'binaries'
    binaries.mkdir(exist_ok=True)
    for name in ['syntelle-render-worker', 'syntelle-render-frame', 'native-render-tests']:
        with (binaries / name).open('wb') as output:
            subprocess.run(['docker', 'run', '--rm', '--pull', 'never', '--network', 'none',
                            '--entrypoint', 'cat', 'syntelle-render-native:presence-continuity-112',
                            '/usr/local/bin/' + name], stdout=output, check=True)
    report = {'version': 'restored-base-presence/1', 'worker_image': expected,
              'package_hash': manifest['package_hash'], 'baseline_runtime_snapshot': 766,
              'baseline_resources_snapshot': 730, 'restoration_snapshot': 771,
              'mouth_and_laughter_preserved': True, 'speech_jaw_maximum_fraction': .0075,
              'lower_neck_and_clothing_pixel_identity': True, 'provider_calls': 0,
              'minimum_cpu_cuda_psnr_db': render['minimum_psnr_db'],
              'owner_naturalness': 'pending', 'new_performance_qualification': False}
    (folder / 'activation.json').write_bytes(encoded(report))
    archive = Archive()
    result = archive.archive_tree('restored-base-presence-112', folder, report)
    restored = ROOT / ('.tools/presence-continuity-112-restored-' + str(result['snapshot_id']))
    count = archive.restore(result['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert path.read_bytes() == (folder / path.relative_to(restored)).read_bytes(), path
    receipt = {**result, 'restored_files': count, 'all_hashes_match': True,
               'package_hash': manifest['package_hash'], 'baseline_runtime_snapshot': 766}
    archive.event('restored-base-presence-112', 'restoration-verified', receipt)
    (ROOT / '.tools/presence-continuity-112-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

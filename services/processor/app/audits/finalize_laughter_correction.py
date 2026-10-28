"""Preserve the rejected baseline and byte-restore the offline regression repair."""
import json
import shutil
import subprocess
from pathlib import Path

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder = ROOT / '.tools/laughter-correction-110'
    manifest = json.loads((folder / 'prepared/package/manifest.json').read_bytes())
    previous = json.loads((ROOT / '.tools/laughter-recovery-109/prepared-joint/package/manifest.json').read_bytes())
    profile = json.loads((folder / 'profile/profile.json').read_bytes())
    old_profile = json.loads((ROOT / '.tools/laughter-recovery-109/profile/profile.json').read_bytes())
    assert profile['laughter'] == old_profile['laughter']
    assert profile['voice'] == old_profile['voice'] == 'en-US-Emma:DragonHDLatestNeural'
    assert profile['package_hash'] == manifest['package_hash']
    assert manifest['geometry'] == previous['geometry']
    assert manifest['packages']['oral'] == previous['packages']['oral']
    assert manifest['packages']['motion']['mandible']['version'] == 'mandibular-rig/2'
    review = json.loads((folder / 'internal-review.json').read_bytes())
    assert review['technical_checks_passed'] and review['provider_requests'] == 0
    source = folder / 'source'
    for name in ['packages/render-contract', 'services/render-worker/internal',
                 'services/render-worker/cmd', 'services/processor/app',
                 'services/processor/tests', 'specs/004-coordinated-performance']:
        shutil.copytree(ROOT / name, source / name, dirs_exist_ok=True,
                        ignore=shutil.ignore_patterns('__pycache__', '*.pyc', '.venv'))
    for name in ['AGENTS.md', 'docs/experiments/motion/laughter-regression-correction.md',
                 'services/render-worker/go.mod', 'services/render-worker/go.sum']:
        target = source / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, target)
    inputs = folder / 'inputs'
    inputs.mkdir(exist_ok=True)
    for original, name in [
        (ROOT / '.tools/foreground-11/prepared/syntelle/base-landmarks.json', 'base-landmarks.json'),
        (ROOT / '.tools/laughter-recovery-109-archive.json', 'phase109-restoration-receipt.json'),
        (ROOT / '.tools/emma-presence-107/owner-audible-confirmation.json', 'unchanged-audio-approval.json'),
        (Path('E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg'), 'immutable-original.jpg')]:
        shutil.copy2(original, inputs / name)
    images = {}
    for name in ['syntelle-render-native:laughter-recovery-109',
                 'syntelle-render-native:laughter-correction-110', 'syntelle-render-native:local']:
        images[name] = subprocess.check_output(
            ['docker', 'image', 'inspect', name, '--format', '{{.Id}}'], text=True).strip()
    assert images['syntelle-render-native:local'] == images['syntelle-render-native:laughter-correction-110']
    parity = json.loads((folder / 'mouth-parity-ordered.json').read_bytes())
    assert parity['passed'] and parity['minimum_psnr_db'] >= 45
    for name in review['production_reports']:
        playback = json.loads((folder / name).read_bytes())
        assert playback['technical_route_passed'] and playback['cleanup_status'] == 204
        assert playback['provider_calls'] == playback['microphone_calls'] == 0
        assert playback['worker_image_id'] == images['syntelle-render-native:laughter-correction-110']
    binaries = folder / 'binaries'
    binaries.mkdir(exist_ok=True)
    for name in ['syntelle-render-worker', 'syntelle-render-frame', 'native-render-tests']:
        with (binaries / name).open('wb') as output:
            subprocess.run(['docker', 'run', '--rm', '--pull', 'never', '--network', 'none',
                            '--entrypoint', 'cat', 'syntelle-render-native:laughter-correction-110',
                            '/usr/local/bin/' + name], stdout=output, check=True)
    metadata = {**review, 'package_hash': manifest['package_hash'], 'images': images,
                'parent_snapshot': 769, 'owner_visual_acceptance': 'pending',
                'original_audio_unchanged': True, 'geometry_unchanged': True}
    (folder / 'activation.json').write_bytes(encoded(metadata))
    archive = Archive()
    result = archive.archive_tree('laughter-regression-correction-110', folder, metadata)
    restored = ROOT / ('.tools/laughter-correction-110-restored-' + str(result['snapshot_id']))
    count = archive.restore(result['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert path.read_bytes() == (folder / path.relative_to(restored)).read_bytes(), path
    receipt = {**result, 'restored_files': count, 'all_hashes_match': True,
               'parent_snapshot': 769, 'package_hash': manifest['package_hash']}
    archive.event('laughter-regression-correction-110', 'restoration-verified', receipt)
    (ROOT / '.tools/laughter-correction-110-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

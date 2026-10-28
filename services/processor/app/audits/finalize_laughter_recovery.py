"""Archive complete local laughter-recovery evidence and verify exact restoration."""
import json
import shutil
import subprocess
from hashlib import sha256
from pathlib import Path
from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder = ROOT / '.tools/laughter-recovery-109'
    parent = ROOT / '.tools/jaw-articulation-108'
    manifest = json.loads((folder / 'prepared-joint/package/manifest.json').read_bytes())
    previous = json.loads((parent / 'prepared-final/package/manifest.json').read_bytes())
    profile = json.loads((folder / 'profile/profile.json').read_bytes())
    old_profile = json.loads((parent / 'profile/profile.json').read_bytes())
    assert profile['voice'] == old_profile['voice'] == 'en-US-Emma:DragonHDLatestNeural'
    assert profile['laughter'] == old_profile['laughter']
    assert profile['package_hash'] == manifest['package_hash']
    assert manifest['geometry'] == previous['geometry']
    assert manifest['packages']['motion'] == previous['packages']['motion']
    assert manifest['packages']['oral'] == previous['packages']['oral']
    assert manifest['packages']['gaze'] == previous['packages']['gaze']
    for key, value in previous['textures'].items():
        if not key.startswith('expressions/reference-face-'):
            # Derived reference frame names differ between older bundles; only
            # the explicit companion frame keys are allowed to change.
            frames = {'expressions/' + v['file'] for v in previous['packages']['expressions']['reference_laughter']['frames']}
            if key not in frames:
                assert manifest['textures'][key] == value, key
    for name in ['installed-en-US', 'installed-es-CO', 'installed-mobile-en-US', 'webkit-laughter',
                 'candidate-laugh-en-US-review', 'candidate-laugh-es-CO-review',
                 'candidate-controls-laugh-en-US-review', 'candidate-auto-en-US-review',
                 'candidate-auto-es-CO-review', 'baseline-en-US-review', 'candidate-en-US-review']:
        assert json.loads((folder / (name + '.json')).read_bytes())['passed'], name
    parity = json.loads((folder / 'cpu-cuda-final/packed720.json').read_bytes())
    assert len(parity) >= 40 and min(v['psnr_db'] for v in parity) >= 45
    pulse = json.loads((folder / 'pulse-validation.json').read_bytes())
    assert len(pulse) == 2 and all(v['one_frame_peak_gate'] for v in pulse)
    assert all(all(r['one_frame_gate'] for r in v['rendered_routes'].values()) for v in pulse)
    assert json.loads((folder / 'internal-visual-review.json').read_bytes())['technical_review_passed']
    inputs = folder / 'inputs'
    inputs.mkdir(exist_ok=True)
    shutil.copy2(Path('E:/Carpetas/Documentos/01-MisProyectos/Colors/Syntelle.jpg'), inputs / 'immutable-original.jpg')
    shutil.copy2(ROOT / '.tools/foreground-11/prepared/syntelle/base-landmarks.json', inputs / 'base-landmarks.json')
    shutil.copy2(ROOT / '.tools/paired-parallax-100/prepared-v2/multiview/expressions/registered-reference.png', inputs / 'registered-reference.png')
    shutil.copytree(parent / 'profile', inputs / 'phase108-profile', dirs_exist_ok=True)
    shutil.copy2(parent / 'prepared-final/package/manifest.json', inputs / 'phase108-native-manifest.json')
    shutil.copytree(ROOT / '.tools/emma-presence-107/audible-review', inputs / 'original-listening-receipts', dirs_exist_ok=True)
    for name in ['owner-audible-confirmation.json', 'complete-operations.json']:
        shutil.copy2(ROOT / '.tools/emma-presence-107' / name, inputs / name)
    def function(text, name):
        start = text.index('func ' + name + '(')
        end = text.find('\nfunc ', start + 1)
        return text[start:] if end < 0 else text[start:end]
    old = (parent / 'final-source/services/render-worker/internal/nativeanimation/speech.go').read_text(encoding='utf-8')
    current = (ROOT / 'services/render-worker/internal/nativeanimation/speech.go').read_text(encoding='utf-8')
    assert function(old, 'MouthAt') == function(current, 'MouthAt')
    source = folder / 'final-source'
    for name in ['packages/render-contract', 'services/render-worker/internal', 'services/render-worker/cmd',
                 'services/render-worker/scripts', 'services/api/internal', 'services/api/cmd',
                 'services/processor/app', 'services/processor/tests', 'apps/web/src', 'apps/web/config',
                 'specs/004-coordinated-performance']:
        shutil.copytree(ROOT / name, source / name, dirs_exist_ok=True,
                        ignore=shutil.ignore_patterns('__pycache__', '*.pyc', 'node_modules', '.venv'))
    for name in ['AGENTS.md', 'compose.yaml', 'docs/README.md', 'docs/experiments/motion/coordinated-performance.md',
                 'docs/experiments/motion/complete-laughter-recovery.md', 'apps/web/package.json', 'apps/web/package-lock.json',
                 'apps/web/vite.config.ts', 'apps/web/playwright.config.ts', 'services/api/go.mod', 'services/api/go.sum',
                 'services/render-worker/go.mod', 'services/render-worker/go.sum', 'services/processor/pyproject.toml']:
        target = source / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, target)
    images = {}
    binaries = folder / 'binaries'
    binaries.mkdir(exist_ok=True)
    for role, image, path in [
        ('worker', 'syntelle-render-native:laughter-recovery-109', '/usr/local/bin/syntelle-render-worker'),
        ('phase108-worker', 'syntelle-render-native:before-laughter-recovery-109', '/usr/local/bin/syntelle-render-worker'),
        ('api', 'syntelle-api:laughter-recovery-109', '/usr/local/bin/syntelle-api'),
        ('phase108-api', 'syntelle-api:before-laughter-recovery-109', '/usr/local/bin/syntelle-api')]:
        images[role] = subprocess.check_output(['docker', 'image', 'inspect', image, '--format', '{{.Id}}'], text=True).strip()
        with (binaries / role).open('wb') as out:
            subprocess.run(['docker', 'run', '--rm', '--pull', 'never', '--network', 'none', '--entrypoint', 'cat', image, path], stdout=out, check=True)
    worker = json.loads(subprocess.check_output(['docker', 'inspect', 'syntelle-render-native-local'], text=True))[0]
    assert worker['Image'] == images['worker'] and worker['State']['Running']
    images['web'] = subprocess.check_output(['docker', 'image', 'inspect', 'syntelle-web:laughter-recovery-109', '--format', '{{.Id}}'], text=True).strip()
    report = {'version': 'complete-laughter-recovery/1', 'parent_snapshot': 768, 'base_resource_snapshot': 730,
              'package_hash': manifest['package_hash'], 'images': images, 'installed': True,
              'provider_calls': 0, 'new_models': 0, 'audio_rate': 1, 'mouth_timing_function_unchanged': True,
              'geometry_unchanged': True, 'expression_amplitude': 1,
              'parity_minimum_psnr_db': min(v['psnr_db'] for v in parity),
              'serial_comparison': json.loads((folder / 'serial-comparison.json').read_bytes()),
              'approved_audio_sha256': {v['language']: v['audio_sha256'] for v in pulse},
              'audio_approval': 'unchanged original owner-accepted recordings',
              'owner_naturalness': 'pending', 'public_upload_validation': False}
    (folder / 'activation.json').write_bytes(encoded(report))
    db = Archive()
    result = db.archive_tree('complete-laughter-recovery-109', folder, report)
    restored = ROOT / ('.tools/laughter-recovery-109-restored-' + str(result['snapshot_id']))
    count = db.restore(result['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert path.read_bytes() == (folder / path.relative_to(restored)).read_bytes(), path
    receipt = {**result, 'restored_files': count, 'all_hashes_match': True, 'parent_snapshot': 768,
               'package_hash': manifest['package_hash']}
    db.event('complete-laughter-recovery-109', 'restoration-verified', receipt)
    (ROOT / '.tools/laughter-recovery-109-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

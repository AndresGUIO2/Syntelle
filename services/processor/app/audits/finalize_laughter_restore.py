"""Archive and byte-restore the owner's rollback to the pre-articulation runtime."""
import base64
import json
import shutil
import subprocess
from hashlib import sha256

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def main():
    folder = ROOT / '.tools/laughter-restore-111'
    package = ROOT / '.tools/paired-parallax-100/prepared-v2/package'
    profile = ROOT / '.tools/emma-presence-107/profile-v3'
    manifest = json.loads((package / 'manifest.json').read_bytes())
    direction = json.loads((profile / 'profile.json').read_bytes())
    assert direction['package_hash'] == manifest['package_hash'] == 'eb1c1df5e15d773cab77da7f877f048447f3b2b1cffb6865524d69d645745da9'
    assert direction['voice'] == 'en-US-Emma:DragonHDLatestNeural'
    assert manifest['packages']['motion'].get('mandible') is None
    assert manifest['packages']['expressions']['reference_laughter'].get('presentation') is None
    rejected = json.loads((ROOT / '.tools/laughter-correction-110/profile/profile.json').read_bytes())
    assert direction['laughter'] == rejected['laughter']
    review = json.loads((folder / 'restored-en-US/report.json').read_bytes())
    assert review['technical_route_passed'] and review['cleanup_status'] == 204
    images = {}
    for container, image in [('syntelle-render-native-local', 'syntelle-render-native:emma-presence-107'),
                             ('syntelle-api-1', 'syntelle-api:before-emma-presence-107'),
                             ('syntelle-web-1', 'syntelle-web:before-emma-presence-107')]:
        expected = subprocess.check_output(['docker', 'image', 'inspect', image, '--format', '{{.Id}}'], text=True).strip()
        actual = subprocess.check_output(['docker', 'inspect', container, '--format', '{{.Image}}'], text=True).strip()
        assert actual == expected
        images[container] = actual
    inputs = folder / 'inputs'
    inputs.mkdir(exist_ok=True)
    shutil.copytree(package, inputs / 'exact-phase107-package', dirs_exist_ok=True)
    shutil.copytree(profile, inputs / 'exact-phase107-profile', dirs_exist_ok=True)
    for name in ['emma-presence-107-archive.json', 'laughter-correction-110-archive.json']:
        shutil.copy2(ROOT / '.tools' / name, inputs / name)
    source = folder / 'source'
    for name in ['AGENTS.md', 'docs/experiments/motion/laughter-regression-correction.md',
                 'specs/004-coordinated-performance/tasks.md', 'specs/004-coordinated-performance/verify.md',
                 'services/processor/app/audits/finalize_laughter_restore.py']:
        target = source / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, target)
    metadata = {'version': 'pre-articulation-restoration/1', 'images': images,
                'restored_runtime_snapshot': 766, 'restored_resource_snapshot': 730,
                'rejected_snapshot': 770, 'package_hash': manifest['package_hash'],
                'voice': direction['voice'], 'provider_calls': 0,
                'withdrawn': ['phase108-110 mandibular/within-viseme/laughter-composition changes'],
                'preserved': ['phase106 A/V synchronization', 'phase107 movement', 'original Emma EN/ES audio'],
                'audio_sha256': {key: sha256(base64.b64decode(value['audio_base64'])).hexdigest()
                                 for key, value in direction['laughter'].items()},
                'owner_naturalness': 'not requalified; owner rejected newer candidates'}
    (folder / 'restoration.json').write_bytes(encoded(metadata))
    archive = Archive()
    result = archive.archive_tree('pre-articulation-restoration-111', folder, metadata)
    restored = ROOT / ('.tools/laughter-restore-111-restored-' + str(result['snapshot_id']))
    count = archive.restore(result['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert path.read_bytes() == (folder / path.relative_to(restored)).read_bytes(), path
    receipt = {**result, 'restored_files': count, 'all_hashes_match': True,
               'restored_runtime_snapshot': 766, 'rejected_snapshot': 770}
    archive.event('pre-articulation-restoration-111', 'restoration-verified', receipt)
    (ROOT / '.tools/laughter-restore-111-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

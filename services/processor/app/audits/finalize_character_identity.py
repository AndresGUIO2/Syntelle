"""Archive the private character identity change without provider requests."""
from hashlib import sha256
import json
import shutil
import subprocess

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded


def read(path):
    return json.loads(path.read_bytes())


def inspect(name, value='{{.Image}}'):
    return subprocess.check_output(['docker', 'inspect', name, '--format', value], text=True).strip()


def main():
    folder = ROOT / '.tools/character-identity-115'
    baseline = ROOT / '.tools/emotional-presence-114'
    before = read(folder / 'baseline.json')
    profile = read(folder / 'profile/profile.json')
    previous = read(baseline / 'profile-qualified/profile.json')
    identity = read(ROOT / 'docs/characters/syntelle/identity.json')
    assert profile['identity'] == identity
    assert {k: v for k, v in profile.items() if k != 'identity'} == previous
    assert (folder / 'profile/voice-capabilities.json').read_bytes() == (baseline / 'profile-qualified/voice-capabilities.json').read_bytes()
    assert inspect('syntelle-render-native-local') == before['images']['syntelle-render-native-local']
    assert inspect('syntelle-web-1') == before['images']['syntelle-web-1']
    api = inspect('syntelle-api-1')
    assert api == inspect('syntelle-api:character-identity-115', '{{.Id}}')
    assert read(folder / 'installation.json')['ready']
    player = read(folder / 'installed-en-US/report.json')
    assert player['technical_route_passed'] and player['cleanup_status'] == 204
    assert player['provider_calls'] == player['microphone_calls'] == 0 and not player['errors']
    assert player['presentation']['rate'] == 1
    assert sorted(player['presentation']['tracks']) == ['audio', 'video']
    assert player['worker_image_id'] == before['images']['syntelle-render-native-local']
    source = read(folder / 'source-identity.json')
    assert source['all_other_source_files_identical'] and len(source['changed']) == 5
    for name in source['changed']:
        assert (folder / 'source' / name).read_bytes() == (ROOT / name).read_bytes()
    sources = folder / 'preparation-source'
    names = ['AGENTS.md', 'docs/README.md', 'docs/development/character-identity.md',
             'docs/characters/syntelle/identity.json', 'services/processor/app/audits/finalize_character_identity.py']
    names += ['specs/005-character-identity/' + n for n in ('spec.md', 'plan.md', 'tasks.md', 'verify.md')]
    for name in names:
        target = sources / name; target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, target)
    inputs = folder / 'inputs'; inputs.mkdir(exist_ok=True)
    shutil.copytree(baseline / 'profile-qualified', inputs / 'phase114-profile', dirs_exist_ok=True)
    for name in ['emotional-presence-114-archive.json', 'emotional-presence-114-completion-archive.json']:
        shutil.copy2(ROOT / '.tools' / name, inputs / name)
    binary = folder / 'syntelle-api'
    with binary.open('wb') as output:
        subprocess.run(['docker', 'run', '--rm', '--pull', 'never', '--network', 'none', '--entrypoint', 'cat',
                        'syntelle-api:character-identity-115', '/usr/local/bin/syntelle-api'], stdout=output, check=True)
    report = {'version': 'configured-character-identity/1', 'baseline_snapshot': 794,
              'api_image': api, 'api_binary_sha256': sha256(binary.read_bytes()).hexdigest(),
              'identity': identity, 'package_hash': profile['package_hash'],
              'worker_image_unchanged': True, 'player_image_unchanged': True,
              'worker_restarted_for_replaced_api_network_namespace': True,
              'profile_fields_except_identity_unchanged': True, 'voice_capabilities_unchanged': True,
              'provider_calls': 0, 'synthesis_calls': 0, 'image_calls': 0, 'analysis_calls': 0,
              'live_answer_review': 'pending; offline context and fixture checks only'}
    (folder / 'activation.json').write_bytes(encoded(report))
    archive = Archive()
    result = archive.archive_tree('configured-character-identity-115', folder, report)
    restored = ROOT / ('.tools/character-identity-115-restored-' + str(result['snapshot_id']))
    count = archive.restore(result['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert path.read_bytes() == (folder / path.relative_to(restored)).read_bytes(), path
    receipt = {**result, 'restored_files': count, 'all_hashes_match': True, 'baseline_snapshot': 794}
    archive.event('configured-character-identity-115', 'restoration-verified', receipt)
    (ROOT / '.tools/character-identity-115-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

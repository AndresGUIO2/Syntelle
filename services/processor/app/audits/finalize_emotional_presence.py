"""Archive the qualified local emotion candidate and verify byte restoration.

Read-only toward providers: this command never synthesizes or analyzes images.
It requires installed playback, fixed-source identity and owner audio receipts.
"""
from hashlib import sha256
import json
import shutil
import subprocess

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded

EXPERIMENT = 'persistent-emotions-emma-114'
PACKAGE_HASH = 'f2a204d2ef6a122dc23dcac8e8b0a7112a4681435127750c9ba190e01d1a0949'
STYLES = ('sad', 'angry', 'reassuring', 'amused', 'curious')


def read(path):
    return json.loads(path.read_bytes())


def docker(*args):
    return subprocess.check_output(['docker', *args], text=True).strip()


def main():
    folder = ROOT / '.tools/emotional-presence-114'
    previous = ROOT / '.tools/jaw-transition-113'
    package = folder / 'range-v3/prepared/package'
    manifest = read(package / 'manifest.json')
    baseline = read(previous / 'prepared-source-bound/package/manifest.json')
    profile = read(folder / 'profile-qualified/profile.json')
    capability = read(folder / 'profile-qualified/voice-capabilities.json')
    assert manifest['package_hash'] == profile['package_hash'] == PACKAGE_HASH
    assert manifest['geometry'] == baseline['geometry']
    assert all(manifest['packages'][name] == baseline['packages'][name] for name in ('oral', 'gaze'))
    for key, value in baseline['textures'].items():
        if key != 'expressions/' + baseline['packages']['expressions']['emotion_rig']['field']['file']:
            assert manifest['textures'][key] == value, key
    presence = manifest['packages']['motion']['emotional_presence']
    assert presence['minimum_automatic_intensity'] == .4
    assert presence['inactivity_ms'] == 180000 and presence['recovery_ms'] == 60000
    assert manifest['packages']['motion']['speech_jaw']['maximum_chin_fraction'] == .0075
    assert profile['laughter'] == read(previous / 'profile/profile.json')['laughter']
    assert profile['voice'] == capability['voice'] == 'en-US-Emma:DragonHDLatestNeural'
    assert capability['region'] == 'canadacentral'
    assert set(capability['locales']['en-US']['styles']) == set(STYLES)
    assert capability['locales']['es-CO']['styles'] == []
    for style in ('fearful', *STYLES):
        review = read(folder / 'audible-review' / style / 'receipt.json')
        assert review == capability['style_reviews'][style]
        assert review['qualified'] == (style in STYLES)
        for mode in ('plain', 'directed'):
            audio = (folder / 'speech' / (style + '-' + mode) / 'audio.wav').read_bytes()
            assert sha256(audio).hexdigest() == review['audio_sha256'][mode]

    render = read(folder / 'render-verification.json')
    assert render['passed'] and len(render['cases']) == 176
    assert render['minimum_cpu_cuda_psnr_db'] > 45
    for case in render['cases']:
        assert case.get('lower_neck_clothing_equal', True)
        assert case.get('unaffected_full_pixels_equal', True)
    timing = read(folder / 'candidate-native-timing.json')
    assert timing['passed'] and len(timing['pulses']) == 5
    assert timing['presentation']['rate'] == 1 and not timing['errors']

    current_worker = docker('inspect', 'syntelle-render-native-local', '--format', '{{.Image}}')
    assert current_worker == docker('image', 'inspect', 'syntelle-render-native:emotional-presence-114', '--format', '{{.Id}}')
    current_api = docker('inspect', 'syntelle-api-1', '--format', '{{.Image}}')
    assert current_api == docker('image', 'inspect', 'syntelle-api:emotional-presence-114', '--format', '{{.Id}}')
    installed = read(folder / 'installed-en-US/report.json')
    assert installed['worker_image_id'] == current_worker
    playbacks = {}
    for name in ('candidate-normal-en-US', 'candidate-mobile-es-CO',
                 'candidate-chase-fullpcm-en-US', 'candidate-benchmark-en-US',
                 'baseline-benchmark-en-US', 'installed-en-US'):
        report = read(folder / name / 'report.json')
        assert report['technical_route_passed'] and report['cleanup_status'] == 204, name
        assert report['provider_calls'] == report['microphone_calls'] == 0, name
        assert not report['errors'] and report['presentation']['rate'] == 1, name
        assert sorted(report['presentation']['tracks']) == ['audio', 'video'], name
        playbacks[name] = {k: report.get(k) for k in ('worker_image_id', 'measurement', 'resources_before', 'resources_after')}
    volume = read(folder / 'install-volume-verification.json')
    assert volume['all_hashes_match'] and volume['files'] == 121 and volume['package_hash'] == PACKAGE_HASH

    # Preserve the exact preparation inputs and baseline restoration receipt.
    inputs = folder / 'inputs'; inputs.mkdir(exist_ok=True)
    for source, name in ((ROOT / '.tools/foreground-11/prepared/syntelle/base-landmarks.json', 'base-landmarks.json'),
                         (previous / 'prepared-source-bound/package/manifest.json', 'phase113-manifest.json'),
                         (ROOT / '.tools/jaw-transition-113-archive.json', 'snapshot774-receipt.json')):
        shutil.copy2(source, inputs / name)
    shutil.copytree(previous / 'profile', inputs / 'phase113-profile', dirs_exist_ok=True)
    changes = read(folder / 'source-changes.json')['changed_go']
    protected = {}
    worker_source = folder / 'source/services/render-worker'
    base_source = previous / 'source/services/render-worker'
    for path in worker_source.rglob('*'):
        if not path.is_file():
            continue
        name = 'services/render-worker/' + path.relative_to(worker_source).as_posix()
        old = base_source / path.relative_to(worker_source)
        if name not in changes:
            assert old.exists() and old.read_bytes() == path.read_bytes(), name
            protected[name] = sha256(path.read_bytes()).hexdigest()
    (folder / 'protected-source-identity.json').write_bytes(encoded({
        'passed': True, 'unchanged_worker_files': protected,
        'source_closure': 'frozen113worker+107api+explicit-patches',
        'no_rejected_shader_or_oral_changes': True,
    }))
    sources = folder / 'preparation-source'
    for name in ['AGENTS.md', 'docs/README.md', 'docs/experiments/motion/persistent-emotions-emma.md',
                 'docs/characters/emma-emotion-experiment.json',
                 'specs/004-coordinated-performance/spec.md', 'specs/004-coordinated-performance/plan.md',
                 'specs/004-coordinated-performance/tasks.md', 'specs/004-coordinated-performance/verify.md',
                 'specs/004-coordinated-performance/emma-emotion-coherence-plan.md',
                 'services/processor/app/workflows/motion/prepare_emotional_presence.py',
                 'services/processor/app/workflows/motion/prepare_speech_jaw.py',
                 'services/processor/app/workflows/speech/qualify_emotional_delivery.py',
                 'services/processor/tests/test_prepare_emotional_presence.py',
                 'services/processor/app/audits/finalize_emotional_presence.py']:
        target = sources / name; target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, target)

    binaries = folder / 'binaries'; binaries.mkdir(exist_ok=True)
    binary_hashes = {}
    for image, names in [('syntelle-render-native:emotional-presence-114',
                          ('syntelle-render-worker', 'syntelle-render-frame', 'syntelle-render-probe', 'native-render-tests')),
                         ('syntelle-api:emotional-presence-114', ('syntelle-api',))]:
        for name in names:
            target = binaries / name
            with target.open('wb') as output:
                subprocess.run(['docker', 'run', '--rm', '--pull', 'never', '--network', 'none',
                                '--entrypoint', 'cat', image, '/usr/local/bin/' + name], stdout=output, check=True)
            binary_hashes[name] = sha256(target.read_bytes()).hexdigest()
    # Player comparisons precede the addition of the private audit executable.
    # Prove they exercised the same production worker binary as installation.
    compared_worker = read(folder / 'candidate-normal-en-US/report.json')['worker_image_id']
    proof_path = folder / 'comparison-binary-identity.json'
    if proof_path.exists():
        proof = read(proof_path)
        assert proof['compared_worker_image'] == compared_worker
        assert proof['installed_worker_sha256'] == binary_hashes['syntelle-render-worker']
        assert proof['compared_worker_sha256'] == proof['installed_worker_sha256']
    else:
        retained = 'syntelle-emotional-presence114-assets'
        assert docker('inspect', retained, '--format', '{{.Image}}') == compared_worker
        old_binary = subprocess.check_output(['docker', 'exec', retained, 'cat', '/usr/local/bin/syntelle-render-worker'])
        assert sha256(old_binary).hexdigest() == binary_hashes['syntelle-render-worker']
        proof_path.write_bytes(encoded({'compared_worker_image': compared_worker,
                                       'compared_worker_sha256': sha256(old_binary).hexdigest(),
                                       'installed_worker_sha256': binary_hashes['syntelle-render-worker']}))

    archive = Archive()
    with archive.connect() as conn:
        rows = conn.execute('SELECT id,request_key,binding,status,request,result,usage FROM syntelle_conversation.operations WHERE kind=%s ORDER BY id',
                            ('owner-approved-emma-emotion-2026-10-28',)).fetchall()
    assert len(rows) == 12 and all(row['status'] == 'received' for row in rows)
    attempts = sum(row['request']['provider_attempt'] for row in rows)
    characters = sum(row['request']['characters'] for row in rows)
    assert attempts == 12 and characters == 2346
    (folder / 'complete-synthesis-operation-records.json').write_bytes(encoded(rows))
    review = read(folder / 'internal-visual-review.json')
    assert review['inspected'] and review['fearful_naturalness'] == 'still weak; not accepted'
    report = {'version': 'persistent-emotions-emma/1', 'baseline_snapshot': 774,
              'package_hash': PACKAGE_HASH, 'worker_image': current_worker, 'api_image': current_api,
              'binary_sha256': binary_hashes, 'compared_and_installed_worker_binary_identical': True,
              'qualified_gains': presence['gains'], 'English_styles': list(STYLES), 'Spanish_styles': [],
              'fearful_voice': 'weak; not enabled', 'fearful_face': 'still subtle; owner acceptance open',
              'synthesis_attempts': attempts, 'submitted_utf16_characters': characters,
              'image_requests': 0, 'analysis_requests': 0, 'model_requests': 0, 'retries': 0,
              'original_laughter_preserved': True, 'playbacks': playbacks,
              'owner_facial_naturalness': 'pending', 'strict_30fps': 'not qualified',
              'full_WebKit_streaming': 'not qualified', 'installed_assets': volume['volume']}
    (folder / 'activation.json').write_bytes(encoded(report))
    result = archive.archive_tree(EXPERIMENT, folder, report)
    restored = ROOT / ('.tools/emotional-presence-114-restored-' + str(result['snapshot_id']))
    count = archive.restore(result['snapshot_id'], restored)
    for path in restored.rglob('*'):
        if path.is_file():
            assert path.read_bytes() == (folder / path.relative_to(restored)).read_bytes(), path
    receipt = {**result, 'restored_files': count, 'all_hashes_match': True,
               'package_hash': PACKAGE_HASH, 'baseline_snapshot': 774}
    archive.event(EXPERIMENT, 'restoration-verified', receipt)
    (ROOT / '.tools/emotional-presence-114-archive.json').write_bytes(encoded(receipt))
    print(json.dumps(receipt))


if __name__ == '__main__':
    main()

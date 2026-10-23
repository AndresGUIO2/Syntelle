"""Bind reviewed expression capabilities and existing laughter to a private package.

This command performs no inference and makes no provider requests. The review
must refer to the exact exported package; owner acceptance remains separate.
"""
import argparse
import json
from hashlib import sha256
from pathlib import Path

from app.preparation.archive import encoded


def prepare(package: Path, review: Path, laughter: Path | None, output: Path):
    manifest = json.loads((package / 'manifest.json').read_text(encoding='utf-8'))
    inspected = json.loads(review.read_text(encoding='utf-8'))
    identity = manifest['package_hash']
    if inspected.get('package_hash') != identity or not inspected.get('composition_passed') or not inspected.get('playback_passed'):
        raise ValueError('Source-bound composition and playback review is required.')
    if output.exists():
        raise ValueError('Preserve previous profiles; choose a new output path.')
    packages = manifest['packages']
    expressions = packages.get('expressions', {})
    joint = expressions.get('joint_articulation', {})
    capabilities = ['nod', 'shake', 'emphasis', 'tilt', 'brows']
    if joint.get('version') == 'joint-expression-articulation/1' and joint.get('validation', {}).get('valid'):
        capabilities.append('smile')
    bank, sources = {}, []
    reference = expressions.get('reference_laughter')
    if reference is not None:
        motion, oral = packages.get('motion', {}), packages.get('oral', {})
        laugh_ready = (
            reference.get('version') == 'reference-laughter/1'
            and reference.get('validation', {}).get('valid') is True
            and reference.get('dental_ready') is True
            and len(reference.get('frames', [])) == 25
            and reference.get('source_sha256') == motion.get('original_sha256')
            and reference.get('motion_build_id') == motion.get('build_id')
            and reference.get('oral_build_id') == oral.get('build_id')
            and bool(reference.get('reference_sha256'))
        )
    else:
        # Historical joint packages remain replayable. Smile availability is
        # independent: a prepared laugh does not need a manual-smile extension.
        laugh_ready = joint.get('laughter_ready') is True
    if laughter and laugh_ready:
        for record in json.loads(laughter.read_text(encoding='utf-8'))['recordings']:
            path = (laughter.parent / record['file']).resolve()
            raw = path.read_bytes()
            if not record.get('approved') or sha256(raw).hexdigest() != record['sha256']:
                raise ValueError('Laughter needs an approved, exact cached source.')
            audio = json.loads(raw)
            if not 0 < audio['duration_ms'] <= 2500 or not audio.get('audio_base64'):
                raise ValueError('Laughter recording exceeds the bounded clip length.')
            key = audio['voice'] + '|' + audio['language']
            if key in bank:
                raise ValueError('Duplicate laughter voice/language binding.')
            bank[key] = audio
            sources.append({'voice_language': key, 'sha256': record['sha256'], 'approval': record['approval']})
        if bank:
            capabilities.append('laugh')
    result = {'version': 'character-performance/1', 'package_hash': identity,
              'reviewed': True, 'owner_acceptance': bool(inspected.get('owner_acceptance', False)),
              'capabilities': capabilities, 'laughter': bank,
              'review_sha256': sha256(review.read_bytes()).hexdigest(),
              'laughter_sources': sources, 'paid_provider_calls': 0}
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(encoded(result))
    return {'package_hash': identity, 'capabilities': capabilities, 'cached_recordings': len(bank), 'paid_provider_calls': 0}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--package', type=Path, required=True)
    parser.add_argument('--review', type=Path, required=True)
    parser.add_argument('--laughter', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(prepare(args.package.resolve(), args.review.resolve(), args.laughter.resolve() if args.laughter else None, args.output.resolve())))

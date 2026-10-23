"""Prepare versioned breathing/double-blink settings using cached rigs only."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.core.paths import ROOT
from app.preparation.archive import Archive, encoded
from app.workflows.visemes.viseme_trial import write_report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, default=ROOT/'.tools/contextual-motion-15')
    parser.add_argument('--output', type=Path, default=ROOT/'.tools/vitality-motion-17')
    args = parser.parse_args()
    settings = json.loads((ROOT/'docs/characters/vitality-motion.json').read_text())
    behavior = {key: value for key, value in settings.items() if key != 'breathing'}
    sources = [
        'animation/breathing.ts', 'animation/speechBehavior.ts',
        'composables/useMotionPlayback.ts', 'rendering/characterRenderer.ts',
    ]
    hashes = {name: sha256((ROOT/'apps/web/src/features/character'/name).read_bytes()).hexdigest()
              for name in sources}
    db = Archive()
    for name in ['syntelle', 'cafe', 'woodland']:
        parent = args.input/name/'package.json'
        package = json.loads(parent.read_text())
        if not package['rig']['geometry_validation']['valid']:
            raise SystemExit('Cannot use invalid geometry.')
        package['animation_seed'] = package.get('animation_seed', int(package['build_id'][:8], 16))
        package['behavior'] = behavior
        package['breathing'] = settings['breathing']
        package['vitality_sources_sha256'] = hashes
        package['behavior_source_sha256'] = hashes['animation/speechBehavior.ts']
        package['behavior_parent_sha256'] = sha256(parent.read_bytes()).hexdigest()
        package.pop('build_id')
        package['build_id'] = sha256(encoded(package)).hexdigest()
        out = args.output/name
        out.mkdir(parents=True, exist_ok=True)
        write_report(out/'package.json', package)
        write_report(out/'settings.json', settings)
        preview = ROOT/'apps/web/src/experiments/motion/vitality'/name/'package.json'
        preview.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(out/'package.json', preview)
        print(name, db.archive_tree('vitality-motion-17/'+name, out), flush=True)


if __name__ == '__main__':
    main()

"""Add shared speech/idle behavior settings to cached validated rigs; no providers."""
import argparse
from hashlib import sha256
import json
from pathlib import Path
import shutil

from app.preparation.archive import Archive, encoded
from app.workflows.visemes.viseme_trial import ROOT, write_report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, default=ROOT/'.tools/head-motion-14')
    parser.add_argument('--output', type=Path, default=ROOT/'.tools/contextual-motion-15')
    args = parser.parse_args()
    settings = json.loads((ROOT/'docs/characters/contextual-motion.json').read_text())
    source = ROOT/'apps/web/src/features/character/animation/speechBehavior.ts'
    db = Archive()
    for name in ['syntelle', 'cafe', 'woodland']:
        package_path = args.input/name/'package.json'
        package = json.loads(package_path.read_text())
        if not package['rig']['geometry_validation']['valid']:
            raise SystemExit('Cannot add behavior to invalid geometry.')
        package['behavior'] = settings
        package['behavior_source_sha256'] = sha256(source.read_bytes()).hexdigest()
        package['behavior_parent_sha256'] = sha256(package_path.read_bytes()).hexdigest()
        package.pop('build_id')
        package['build_id'] = sha256(encoded(package)).hexdigest()
        out = args.output/name
        out.mkdir(parents=True, exist_ok=True)
        write_report(out/'package.json', package)
        write_report(out/'settings.json', settings)
        preview = ROOT/'apps/web/src/experiments/motion/contextual'/name/'package.json'
        preview.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(out/'package.json', preview)
        print(name, db.archive_tree('contextual-motion-15/'+name, out), flush=True)


if __name__ == '__main__':
    main()

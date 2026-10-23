"""Source-bound laughter reference using the existing immutable image recorder."""
import argparse
import json
from pathlib import Path

from app.core.paths import ROOT
from app.preparation.archive import Archive
from app.preparation.faces import PreparationError
from app.workflows.motion.prepare_smile_reference import run as recorded_reference

IDENTITY = 'authorized-character-performance-2026-10-20'


def run(config: Path, output: Path, request=False, correction=None):
    db = Archive()
    with db.connect() as conn:
        probes = conn.execute('SELECT request_key,status FROM syntelle_conversation.operations WHERE kind=%s', (IDENTITY,)).fetchall()
    if len(probes) != 4 or any(p['status'] != 'received' for p in probes):
        raise PreparationError('All four bounded capability probes must succeed before an image request.')
    review = json.loads((ROOT/'.tools/character-performance-34/audio-review.json').read_text())
    if not review.get('en-US') or not review.get('es-CO') or review.get('reviewer') != 'owner':
        raise PreparationError('Audible owner review is required before image expenditure.')
    db.event(IDENTITY, 'audible-review', review)
    # Reuse the proven original-bytes-first recorder, including its two-image
    # transactional cap and rejected-review requirement for a sole correction.
    recorded_reference(config, output, request, correction)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--request', action='store_true')
    parser.add_argument('--correction-review', type=Path)
    args = parser.parse_args()
    run(ROOT/'docs/characters/laughter-reference-proposal.json',
        ROOT/'.tools/character-performance-34/reference', args.request, args.correction_review)

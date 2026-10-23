"""Build character-independent review sheets from actual browser exports.

These sheets are evidence for human inspection, never an automatic visual pass.
The semantic mouth bounds determine the view; no character coordinates are used.
"""
import argparse
import json
from pathlib import Path

import numpy as np
from PIL import Image, ImageDraw

from app.preparation.faces import PreparationError
from app.workflows.visemes.viseme_trial import write_report


def review(package_dir: Path, browser: Path):
    package = json.loads((package_dir / 'package.json').read_text())
    report = json.loads((browser / 'report.json').read_text())
    if report['build_id'] != package['build_id']:
        raise PreparationError('Review images belong to another package build.')
    crop = package['crop']
    base = Image.open(package_dir / 'base.webp').convert('RGBA').crop(
        (crop['x'], crop['y'], crop['x'] + crop['width'], crop['y'] + crop['height']))
    union = np.zeros((crop['height'], crop['width']), bool)
    for pose in package['poses']:
        union |= np.asarray(Image.open(package_dir / pose.lower() / 'outer_lips.png')) >= 128
    ys, xs = np.where(union)
    if not len(xs):
        raise PreparationError('No measured lips for visual inspection.')
    padding = max(3, round(np.ptp(xs) * .15))
    box = (max(0, int(xs.min()) - padding), max(0, int(ys.min()) - padding),
           min(crop['width'], int(xs.max()) + padding + 1),
           min(crop['height'], int(ys.max()) + padding + 1))
    tile_w = 320
    tile_h = max(100, round(tile_w * (box[3] - box[1]) / (box[2] - box[0])))

    def frame(file):
        rgba = Image.open(file).convert('RGBA')
        return Image.alpha_composite(base, rgba).convert('RGB').crop(box).resize(
            (tile_w, tile_h), Image.Resampling.LANCZOS)

    poses = list(package['poses'])
    sheet = Image.new('RGB', (tile_w * 3, (tile_h + 30) * 3), '#19181d')
    draw = ImageDraw.Draw(sheet)
    for i, pose in enumerate(poses):
        x, y = (i % 3) * tile_w, (i // 3) * (tile_h + 30)
        draw.text((x + 8, y + 5), pose, fill='white', font_size=18)
        sheet.paste(frame(browser / f'endpoint-{pose}.png'), (x, y + 30))
    sheet.save(browser / 'endpoints-sheet.png')
    pairs = [('EI', 'FV', .35), ('FV', 'A', .41), ('EI', 'SZ', .35),
             ('O', 'U', .5), ('MBP', 'A', .5), ('Rest', 'CHSHR', .5)]
    selected = Image.new('RGB', (tile_w * 3, (tile_h + 30) * len(pairs)), '#19181d')
    draw = ImageDraw.Draw(selected)
    for row, (a, b, amount) in enumerate(pairs):
        for col, (name, label) in enumerate([
            (f'endpoint-{a}', a), (f'{a}-{b}-{amount}', f'{a} to {b}: {amount:.0%}'),
            (f'endpoint-{b}', b)]):
            x, y = col * tile_w, row * (tile_h + 30)
            draw.text((x + 8, y + 5), label, fill='white', font_size=18)
            selected.paste(frame(browser / f'{name}.png'), (x, y + 30))
    selected.save(browser / 'reported-transitions.png')
    for amount in [.25, .35, .41, .5, .75]:
        matrix = Image.new('RGB', (tile_w * len(poses), (tile_h + 30) * len(poses)), '#19181d')
        draw = ImageDraw.Draw(matrix)
        for row, a in enumerate(poses):
            for col, b in enumerate(poses):
                name = f'endpoint-{a}' if a == b else f'{a}-{b}-{amount}'
                x, y = col * tile_w, row * (tile_h + 30)
                draw.text((x + 8, y + 5), f'{a} to {b}', fill='white', font_size=18)
                matrix.paste(frame(browser / f'{name}.png'), (x, y + 30))
        matrix.save(browser / f'all-pairs-{amount}.png')
    write_report(browser / 'visual-review.json', {'build_id': package['build_id'],
        'crop_from_measured_lips': list(box), 'manual_coordinates': False,
        'visual_acceptance': False, 'status': 'awaiting visual inspection',
        'frames_from': 'same browser compositor used at runtime'})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--package', type=Path, required=True)
    parser.add_argument('--browser', type=Path, required=True)
    args = parser.parse_args()
    review(args.package.resolve(), args.browser.resolve())

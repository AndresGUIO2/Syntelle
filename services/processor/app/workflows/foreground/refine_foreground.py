"""Offline foreground refinement using existing motion resources. No provider code runs."""
import argparse
from app.core.paths import PREPARATION_DIR
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np
from PIL import Image, ImageOps, ImageFilter, ImageDraw

from app.preparation.archive import Archive
from app.preparation.faces import PreparationError
from app.preparation.motion_assets import subject
from app.workflows.motion.prepare_motion import package, install
from app.workflows.visemes.viseme_trial import ROOT, write_report


def digest(path):
    return sha256(path.read_bytes()).hexdigest()


def comparisons(image, directory, previous, config, background_directory=None):
    before = Image.open(previous/'foreground.png').convert('RGBA')
    after = Image.open(directory/'foreground.png').convert('RGBA')
    w, h = image.size
    panel_height = min(540,round(400*h/w))+28
    y, x = np.indices((h,w))
    checker = np.where(((x//24+y//24)%2)[...,None], 190, 105).astype(np.uint8)
    checker = Image.fromarray(np.repeat(checker,3,axis=2)).convert('RGBA')
    backgrounds = [('black',Image.new('RGBA',image.size,'black')),('white',Image.new('RGBA',image.size,'white')),('checker',checker)]
    for item in config['backgrounds']:
        # Use the already archived originals, not temporary clipboard paths.
        source = (background_directory or previous.parent/'backgrounds/originals')/f'{item["id"]}.png'
        background = ImageOps.fit(Image.open(source).convert('RGBA'), image.size)
        backgrounds.extend([(item['id'],background),(item['id']+'-blur',background.filter(ImageFilter.GaussianBlur(12)))])
    thumbs = []
    for name, bg in backgrounds:
        pair = []
        for label, fg in [('before',before),('after',after)]:
            composed = Image.alpha_composite(bg,fg).convert('RGB')
            composed.save(directory/f'{label}-{name}.png')
            thumb = composed.copy(); thumb.thumbnail((400,540)); pair.append(thumb)
        panel = Image.new('RGB',(800,panel_height),'#242327'); draw=ImageDraw.Draw(panel)
        draw.text((10,8),f'{name}: before / after',fill='white')
        for i,thumb in enumerate(pair): panel.paste(thumb,(i*400,28))
        thumbs.append(panel)
    sheet = Image.new('RGB',(800*3,panel_height*3),'#242327')
    for i,thumb in enumerate(thumbs): sheet.paste(thumb,((i%3)*800,(i//3)*panel_height))
    sheet.save(directory/'comparison-sheet.jpg',quality=95)


def run(args):
    config=json.loads(args.config.read_text()); db=Archive()
    if args.phase=='install':
        # All characters must have reviewed hashes before any runtime replacement.
        for name in config['cases']:
            directory=args.output/name
            review=json.loads((directory/'foreground-review.json').read_text())
            if not review.get('accepted') or not review.get('browser_review') or review['foreground_sha256']!=digest(directory/'foreground.webp'):
                raise PreparationError('All three current foregrounds must pass visual inspection before installation.')
        for name in config['cases']: install(args.output/name,name)
        return
    for name in args.cases:
        if name!='syntelle':
            review_path=args.output/'syntelle/foreground-review.json'
            if not review_path.exists(): raise PreparationError('Inspect Syntelle before preparing other characters.')
            review=json.loads(review_path.read_text())
            if not review.get('accepted') or review['foreground_sha256']!=digest(args.output/'syntelle/foreground.webp'):
                raise PreparationError('Syntelle foreground review is absent or stale.')
        item=config['cases'][name]; previous=args.previous/name; directory=args.output/name
        if previous.resolve()==directory.resolve(): raise PreparationError('Preserve the previous preparation directory.')
        if not directory.exists(): shutil.copytree(previous,directory)
        original=(args.config.parent/item['original']).resolve()
        oral_path=(args.config.parent/item['oral']).resolve()
        oral=json.loads((oral_path/'package.json').read_text())
        if digest(original)!=oral['source_sha256']: raise PreparationError('Immutable source differs from the oral package.')
        image=ImageOps.exif_transpose(Image.open(original)).convert('RGB')
        sources=directory/'preparation-source'; sources.mkdir(exist_ok=True)
        for path in [Path(__file__),PREPARATION_DIR/'foreground.py',PREPARATION_DIR/'motion_assets.py',PREPARATION_DIR/'hair_matting.py']:
            shutil.copyfile(path,sources/path.name)
        points=np.asarray(json.loads((directory/'base-landmarks.json').read_text())['landmarks_px'])
        print(f'{name}: source-resolution matting',flush=True)
        try:
            subject(image,points,ROOT/'.tools/models',directory)
            face=type('Face',(),{'points':points})()
            package(config,{**item,'oral_path':oral_path},original,image,face,directory)
            comparisons(image,directory,previous,config,args.backgrounds)
        except Exception as error:
            write_report(directory/'foreground-failure.json',{'type':type(error).__name__,'message':str(error)[:500]})
            raise
        finally:
            result=db.archive_tree(args.experiment+'/'+name,directory)
            print(json.dumps(result),flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config',type=Path,default=ROOT/'docs/characters/motion-development.json')
    parser.add_argument('--previous',type=Path,default=ROOT/'.tools/foreground-10')
    parser.add_argument('--output',type=Path,default=ROOT/'.tools/foreground-11/prepared')
    parser.add_argument('--backgrounds',type=Path,default=ROOT/'.tools/motion-09/backgrounds/originals')
    parser.add_argument('--experiment',default='foreground-edge-11')
    parser.add_argument('--cases',nargs='+',default=['syntelle'])
    parser.add_argument('--phase',choices=['prepare','install'],default='prepare')
    args=parser.parse_args()
    try: run(args)
    except PreparationError as error: raise SystemExit(str(error)) from None

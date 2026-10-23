"""Assemble inspection sheets from actual browser-rendered frames, without generation."""
from pathlib import Path
import argparse
from PIL import Image,ImageDraw
from app.workflows.dentition.prepare_completed_oral import EXPERIMENT


def sheets(experiment=EXPERIMENT):
    folder=experiment/'browser-review'
    previous=experiment/('before-package' if (experiment/'before-package').exists() else 'previous-runtime')
    base=Image.open(previous/'base.webp').crop((734,998,1063,1219))
    poses=['Rest','A','EI','O','U','MBP','SZ','FV','CHSHR']
    def frame(path):
        overlay=Image.open(path).convert('RGBA');image=base.copy();image.paste(overlay,(0,0),overlay)
        return image.crop((55,30,275,145))
    sheet=Image.new('RGB',(1050,660),'#242127');draw=ImageDraw.Draw(sheet)
    for index,pose in enumerate(poses):
        x=(index%3)*350;y=(index//3)*220
        sheet.paste(frame(folder/f'endpoint-{pose}.png').resize((330,172)),(x,y+26));draw.text((x+8,y+6),pose,fill='white')
    sheet.save(folder/'endpoints-sheet.png')
    for t in [.25,.35,.5,.75]:
        sheet=Image.new('RGB',(9*225,9*135),'#242127');draw=ImageDraw.Draw(sheet)
        for row,a in enumerate(poses):
            for column,b in enumerate(poses):
                name=f'endpoint-{a}.png' if a==b else f'{a}-{b}-{t}.png'
                x,y=column*225,row*135
                sheet.paste(frame(folder/name),(x,y+20));draw.text((x+4,y+3),f'{a} > {b}',fill='white')
        sheet.save(folder/f'transitions-{t}.png')
    sheet=Image.new('RGB',(960,740),'#242127');draw=ImageDraw.Draw(sheet)
    for row,(name,label) in enumerate([('EI-FV-0.35','EI to FV at 35%'),('endpoint-FV','FV endpoint'),('EI-SZ-0.35','EI to SZ at 35%'),('endpoint-CHSHR','CHSHR endpoint')]):
        for column in (0,1):
            if column:path=folder/f'{name}.png'
            elif name.startswith('endpoint-'):path=previous/name[9:].lower()/'endpoint.webp'
            else:path=(experiment/'browser-review-previous'/f'{name}.png')
            if not path.exists():path=EXPERIMENT/'browser-review'/f'{name}.png'
            x,y=column*480,row*185
            sheet.paste(frame(path).resize((440,150)),(x+15,y+30))
            draw.text((x+15,y+8),f'{"After" if column else "Before"} - {label}',fill='white')
    sheet.save(folder/'before-after.png')


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--experiment',type=Path,default=EXPERIMENT)
    sheets(parser.parse_args().experiment)

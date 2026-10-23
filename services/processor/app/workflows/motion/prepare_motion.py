"""Private, reproducible motion preparation. Public API and oral assets stay unchanged."""
import argparse
from app.core.paths import PREPARATION_DIR
import base64
from hashlib import sha256
import json
from pathlib import Path
import shutil

import numpy as np
from PIL import Image, ImageDraw, ImageOps
from dotenv import dotenv_values

from app.preparation.archive import Archive, encoded
from app.preparation.recorded_provider import RecordedProvider
from app.preparation.detection import PreparationDetector
from app.preparation.faces import PreparationError
from app.preparation.mouth import export_lossless_pair
from app.preparation.eyes import face_crop,align_eyes,prepare_eyes,render_eyes
from app.preparation.motion_assets import subject,depth
from app.preparation.motion_geometry import validate as validate_motion
from app.workflows.visemes.viseme_trial import ROOT,write_report,save_provider_image
from app.workflows.contours.gemini_contour_trial import stamp

VERSION='syntelle-motion-preparation/1'
NETWORK={'endpoint':'https://generativelanguage.googleapis.com/v1beta/interactions',
         'timeout_seconds':240,'retries':0,'follow_redirects':False}


def digest(path):return sha256(path.read_bytes()).hexdigest()


def background_assets(config,output):
    directory=output/'backgrounds';originals=directory/'originals';originals.mkdir(parents=True,exist_ok=True)
    assets=ROOT/'apps/web/src/experiments/motion/backgrounds';assets.mkdir(parents=True,exist_ok=True)
    entries=[]
    for item in config['backgrounds']:
        source=Path(item['source']);target=originals/f'{item["id"]}{source.suffix}'
        if target.exists() and digest(target)!=digest(source):raise PreparationError('An original background changed; preserve its previous version.')
        shutil.copyfile(source,target)
        image=ImageOps.exif_transpose(Image.open(source)).convert('RGB')
        image.save(assets/f'{item["id"]}.webp',lossless=True,exact=True,method=6)
        image.thumbnail((256,144));image.save(assets/f'{item["id"]}-thumb.webp',lossless=True,exact=True,method=6)
        entries.append({'id':item['id'],'name':item['name'],'image':f'{item["id"]}.webp',
            'thumbnail':f'{item["id"]}-thumb.webp','source_sha256':digest(source)})
    write_report(assets/'catalog.json',{'version':1,'items':entries,'blur':config['background_blur']})
    shutil.copyfile(assets/'catalog.json',directory/'catalog.json')


def generation(original,image,points,directory,provider,config,request,correction=False):
    stage=directory/('correction' if correction else 'proposal');stage.mkdir(parents=True,exist_ok=True)
    box=face_crop(image,points);crop=image.crop(box);crop.save(stage/'face-context.png')
    shutil.copyfile(original,stage/f'original{original.suffix}')
    feedback=''
    if correction:
        if not (directory/'proposal/validation.json').exists():raise PreparationError('Inspect the initial proposal before its one visual correction.')
        feedback='Correct the previous result using the immutable original as identity. '+(directory/'proposal/validation.json').read_text(encoding='utf-8')
    prompt=(ROOT/'docs/prompts/closed-eyes-v1.txt').read_text(encoding='utf-8').replace('{{CORRECTION}}',feedback)
    payload={'model':config['model'],'input':[{'type':'text','text':prompt},
        {'type':'image','mime_type':Image.open(original).get_format_mimetype(),'data':base64.b64encode(original.read_bytes()).decode()},
        {'type':'image','mime_type':'image/png','data':base64.b64encode((stage/'face-context.png').read_bytes()).decode()}],
        'response_format':{'type':'image','mime_type':'image/jpeg','aspect_ratio':'1:1','image_size':'1K'},'store':False}
    if correction:
        previous=json.loads((directory/'proposal/run.json').read_text());raw=directory/'proposal'/previous['file']
        payload['input'].append({'type':'image','mime_type':'image/jpeg','data':base64.b64encode(raw.read_bytes()).decode()})
    binding=sha256(encoded(payload)).hexdigest();ledger=stage/'run.json'
    if ledger.exists():
        record=json.loads(ledger.read_text())
        if record['binding']!=binding or record['status']!='received':raise PreparationError('Closed-eye request cannot be retried or rebound.')
        if digest(stage/record['file'])!=record['raw_sha256']:raise PreparationError('Cached closed-eye generation changed.')
        return record
    if not request:raise PreparationError('No cached closed-eye image; --request is required.')
    write_report(stage/'request.json',payload);(stage/'prompt.txt').write_text(prompt,encoding='utf-8')
    record={'status':'started','binding':binding,'version':VERSION,'started_at':stamp(),'attempts':1,
        'original_sha256':digest(original),'context_box':list(box),'settings':{**NETWORK,'model':config['model'],'response_format':payload['response_format']}}
    write_report(ledger,record)
    key=dotenv_values(ROOT/'.env').get('GEMINI_API_KEY','')
    if not key:raise PreparationError('Configure the existing local Gemini key.')
    try:
        raw,status,_=provider(payload,NETWORK,key,stage)
        if not 200<=status<300:raise PreparationError(f'Provider HTTP {status}; motion batch stopped without retry.')
        body=json.loads(raw)
        parts=[body['output_image']] if body.get('output_image') else [p for step in body.get('steps',[]) for p in step.get('content',[]) if p.get('type')=='image' and p.get('data')]
        if len(parts)!=1:raise PreparationError('Expected one closed-eye output; preserve the raw response.')
        path=save_provider_image(base64.b64decode(parts[0]['data'],validate=True),stage,'closed-eyes')
        record.update(status='received',file=path.name,raw_sha256=digest(path),usage=body.get('usage',{}))
    except (ValueError,PreparationError) as error:
        record.update(status='failed',error=str(error).replace(key,'[redacted]')[:500]);raise PreparationError(record['error']) from None
    finally:
        record['finished_at']=stamp();write_report(ledger,record)
    return record


def eye_assets(image,face,directory,models):
    folder=directory/('correction' if (directory/'correction/run.json').exists() else 'proposal')
    record=json.loads((folder/'run.json').read_text())
    if record['status']!='received':raise PreparationError('No received blink output to prepare.')
    generated=Image.open(folder/record['file']).convert('RGB')
    with PreparationDetector(models/'face_landmarker.task') as detector:variant=detector.detect(generated)
    try:
        scale=(record['context_box'][2]-record['context_box'][0])/generated.width
        aligned,points,registration=align_eyes(image,generated,face.points,variant.points,scale)
        box,mask,curves,observations=prepare_eyes(image,aligned,face.points,points)
        assets=directory/'eyes';assets.mkdir(exist_ok=True)
        opened=image.crop(box);closed=aligned.crop(box)
        export_lossless_pair(opened,assets,'open');export_lossless_pair(closed,assets,'closed');mask.save(assets/'mask.png')
        previews=[]
        for amount in (0,.25,.5,.75,1):
            preview=render_eyes(opened,closed,mask,curves,amount);preview.save(assets/f'preview-{amount}.png');previews.append(preview)
        width=max(500,opened.width);height=round(opened.height*width/opened.width)
        sheet=Image.new('RGB',(width,(height+24)*len(previews)),'#19181d');draw=ImageDraw.Draw(sheet)
        for i,preview in enumerate(previews):
            sheet.paste(preview.resize((width,height),Image.Resampling.LANCZOS),(0,i*(height+24)+24));draw.text((8,i*(height+24)+4),f'Closure {i/4:.0%}',fill='white')
        sheet.save(directory/'blink-contact-sheet.png')
        rgba=np.asarray(image.convert('RGBA')).copy();before=rgba.copy();x,y,x1,y1=box
        rgba[y:y1,x:x1,:3]=np.asarray(previews[-1]);changed=np.any(rgba!=before,axis=2)
        allowed=np.zeros(image.size[::-1],bool);allowed[y:y1,x:x1]=np.asarray(mask)>0
        if (changed&~allowed).any():raise PreparationError('Blink changed original pixels outside its mask.')
        Image.fromarray(rgba).save(directory/'closed-composition.png')
        result={'ready':True,'version':VERSION,'crop':{'x':x,'y':y,'width':x1-x,'height':y1-y},
            'curves':curves.tolist(),'registration':registration,'observations':observations,
            'raw_sha256':record['raw_sha256'],'source_stage':folder.name,'manual_coordinates':False,
            'outside_eye_mask_changed_pixels':0,'iris_policy':'original eye interior remains at original coordinates until occluded'}
        write_report(directory/'eyes.json',result);write_report(folder/'validation.json',{'valid':True,'observations':observations})
        write_report(assets/'geometry.json',{'curves':curves.tolist(),'crop':result['crop']})
    except PreparationError as error:
        write_report(folder/'validation.json',{'valid':False,'issues':[str(error)]});raise


def package(config,item,original,image,face,directory):
    oral_path=item['oral_path'];oral=json.loads((oral_path/'package.json').read_text())
    eyes=json.loads((directory/'eyes.json').read_text())
    if not eyes['ready'] or not (directory/'depth.json').exists() or not (directory/'segmentation.json').exists():
        raise PreparationError('Motion resources are incomplete.')
    forehead=face.points[10,:2];chin=face.points[152,:2]
    face_width=float(np.linalg.norm(face.points[234,:2]-face.points[454,:2]));face_height=float(chin[1]-forehead[1])
    result={'schema':'syntelle-motion-package/1','original_sha256':digest(original),'oral_build_id':oral['build_id'],
        'canvas':{'width':image.width,'height':image.height},'foreground':'foreground.webp','depth':'depth.webp',
        'eyes':{**{k:eyes[k] for k in ('crop','curves')},'open':'eyes/open.webp','closed':'eyes/closed.webp','mask':'eyes/mask.png'},
        'anatomy':{'head_pivot':[float(chin[0]),float(chin[1])],'chin_y':float(chin[1]),'face_width':face_width,'face_height':face_height},
        'blink':config['blink'],'motion':config['motion'],'manual_masks':False,'visual_acceptance':False,
        'segmentation':json.loads((directory/'segmentation.json').read_text()),'depth_record':json.loads((directory/'depth.json').read_text())}
    result['geometry_validation']=validate_motion(Image.open(directory/'depth.webp'),result['anatomy'],image.size,config['motion'])
    result['algorithm_files']={path.name:digest(path) for path in [Path(__file__),PREPARATION_DIR/'eyes.py',PREPARATION_DIR/'motion_assets.py',PREPARATION_DIR/'motion_geometry.py']}
    result['build_id']=sha256(encoded(result)).hexdigest();write_report(directory/'package.json',result)
    return result


def install(directory,name):
    manifest=json.loads((directory/'package.json').read_text());destination=ROOT/'apps/web/src/experiments/motion/assets'/name
    destination.mkdir(parents=True,exist_ok=True)
    for file in ('package.json','foreground.webp','depth.webp','eyes/open.webp','eyes/closed.webp','eyes/mask.png'):
        target=destination/file;target.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(directory/file,target)


def run(args):
    config=json.loads(args.config.read_text());db=Archive();args.output.mkdir(parents=True,exist_ok=True)
    db.budget(config['budget_id'],digest(args.config),config['limits'])
    provider=RecordedProvider(db,config['budget_id'],args.output,'image');models=ROOT/'.tools/models'
    if args.phase=='backgrounds':
        background_assets(config,args.output);db.archive_tree(config['budget_id']+'/backgrounds',args.output/'backgrounds')
        db.archive_tree(config['budget_id']+'/background-runtime',ROOT/'apps/web/src/experiments/motion/backgrounds');return
    for name in args.cases:
        if name not in config['cases']:raise PreparationError('Character is outside the authorized motion batch.')
        if args.phase in ('generate','correct') and name!='syntelle':
            review=args.output/'syntelle/blink-browser-review.json'
            if not review.exists() or not json.loads(review.read_text()).get('visually_accepted'):
                raise PreparationError('Inspect the complete Syntelle blink before generating other characters.')
            if json.loads(review.read_text())['eyes_sha256']!=digest(args.output/'syntelle/eyes.json'):
                raise PreparationError('Syntelle eye resources changed since their browser review.')
        item=config['cases'][name];original=(args.config.parent/item['original']).resolve()
        oral_path=(args.config.parent/item['oral']).resolve();oral=json.loads((oral_path/'package.json').read_text())
        if digest(original)!=oral['source_sha256']:raise PreparationError('Motion original does not match the oral package.')
        item={**item,'oral_path':oral_path};image=ImageOps.exif_transpose(Image.open(original)).convert('RGB')
        directory=args.output/name;directory.mkdir(parents=True,exist_ok=True);print(f'{name}: {args.phase}',flush=True)
        with PreparationDetector(models/'face_landmarker.task') as detector:face=detector.detect(image)
        write_report(directory/'base-landmarks.json',face.as_dict())
        try:
            if args.phase in ('generate','correct'):generation(original,image,face.points,directory,provider,config,args.request,args.phase=='correct')
            elif args.phase=='eyes':eye_assets(image,face,directory,models)
            elif args.phase=='segment':subject(image,face.points,models,directory)
            elif args.phase=='depth':depth(image,models,directory)
            elif args.phase=='package':package(config,item,original,image,face,directory)
            elif args.phase=='install':install(directory,name)
        finally:db.archive_tree(config['budget_id']+'/'+name,directory)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config',type=Path,required=True);parser.add_argument('--output',type=Path,required=True)
    parser.add_argument('--cases',nargs='+',default=['syntelle'])
    parser.add_argument('--phase',choices=['backgrounds','generate','correct','eyes','segment','depth','package','install'],required=True)
    parser.add_argument('--request',action='store_true');args=parser.parse_args();args.config=args.config.resolve();args.output=args.output.resolve()
    try:run(args)
    except PreparationError as error:raise SystemExit(str(error)) from None

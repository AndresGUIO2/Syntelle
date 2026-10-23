"""Original EI -> enlarged/grid/lip views -> proposal -> one visual correction."""
import argparse
import base64
from hashlib import sha256
from importlib.metadata import version
from io import BytesIO
import json
import os
from pathlib import Path

from dotenv import dotenv_values
import numpy as np
from PIL import Image, ImageDraw

from app.workflows.contours.gemini_contour_trial import Contours, build_request, original_crop, post_once, render_contours, response_text, stamp
from app.preparation.faces import PreparationError
from app.preparation.mesh import INNER, OUTER
from app.preparation.oral import polygon_mask
from app.workflows.visemes.viseme_trial import ROOT, write_report

SETTINGS=ROOT/'docs/characters/syntelle/gemini-contour-loop.json'
PROPOSAL_PROMPT=ROOT/'docs/prompts/dental-contours-grid-v1.txt'
CORRECTION_PROMPT=ROOT/'docs/prompts/dental-contours-correction-v1.txt'


def digest(data):return sha256(data).hexdigest()


def png(image):
    stream=BytesIO();image.save(stream,format='PNG');return stream.getvalue()


def input_views(source, native, evidence, settings):
    original=Image.open(BytesIO(native)).convert('RGB');factor=settings['input']['display_scale']
    if not isinstance(factor,int) or not 1<=factor<=8:raise PreparationError('Invalid display scale.')
    clean=original.resize((original.width*factor,original.height*factor),Image.Resampling.LANCZOS)
    width,height=clean.size
    grid=Image.new('RGBA',clean.size);draw=ImageDraw.Draw(grid)
    alpha=round(255*settings['grid']['line_opacity'])
    for value in range(0,1001,settings['grid']['normalized_step']):
        x=min(width-1,round(value*width/1000));y=min(height-1,round(value*height/1000))
        draw.line((x,0,x,height-1),fill=(100,220,235,alpha));draw.line((0,y,width-1,y),fill=(100,220,235,alpha))
        if value not in (0,1000):
            draw.text((x+2,2),str(value),fill=(255,255,255,255),font_size=settings['grid']['font_size'],stroke_width=1,stroke_fill=(0,0,0,220))
            draw.text((2,y+2),str(value),fill=(255,255,255,255),font_size=settings['grid']['font_size'],stroke_width=1,stroke_fill=(0,0,0,220))
    gridded=Image.alpha_composite(clean.convert('RGBA'),grid).convert('RGB')
    run=json.loads((source/'run.json').read_text(encoding='utf-8'))
    landmarks=np.asarray(json.loads((source/'variant-landmarks.json').read_text(encoding='utf-8'))['landmarks_px'])[:,:2]
    landmarks*=np.asarray(evidence['original_size'])/[run['canvas']['width'],run['canvas']['height']]
    landmarks-=[evidence['crop']['x'],evidence['crop']['y']]
    lips=clean.copy();draw=ImageDraw.Draw(lips);cfg=settings['mediapipe']
    for ids,color in [(OUTER,cfg['outer_color']),(INNER,cfg['inner_color'])]:
        path=[tuple(p*factor) for p in landmarks[ids]]
        draw.line(path+[path[0]],fill=tuple(color),width=cfg['line_width'])
    lip_evidence={'inner_landmark_ids':INNER,'outer_landmark_ids':OUTER,
                  'inner_input_pixels':landmarks[INNER].tolist(),'outer_input_pixels':landmarks[OUTER].tolist(),
                  'input_to_display':[[factor,0,0],[0,factor,0],[0,0,1]],
                  'display_to_original':(np.asarray(evidence['input_to_original'])@np.diag([1/factor,1/factor,1])).tolist(),
                  'display_size':[width,height],'coordinate_space':'normalized-full-input-image-xy-0-1000',
                  'manual_mask_inputs':False,'note':'Lip landmarks only; no dental/tongue ground truth.'}
    return {'clean.png':png(clean),'grid.png':png(gridded),'mediapipe-lips.png':png(lips)},lip_evidence


def image_input(data, settings):
    return {'type':'image','mime_type':'image/png','data':base64.b64encode(data).decode('ascii'),
            'resolution':settings['input']['media_resolution']}


def round_request(prompt, views, settings, previous=None):
    payload=build_request(views['clean.png'],prompt,settings)
    payload['input']=[{'type':'text','text':prompt}]+[image_input(views[name],settings) for name in ('clean.png','grid.png','mediapipe-lips.png')]
    if previous is not None:payload['input'].append(image_input(previous,settings))
    return payload


def checked_files(directory, hashes):
    for name,value in hashes.items():
        path=directory/name
        if not path.is_file() or digest(path.read_bytes())!=value:raise PreparationError('Cached experiment evidence changed; no provider call.')


def capture_round(directory, payload, settings, request, *, provider=None):
    """Exclusive request ledger: failed/started work can never be retried."""
    binding=digest(json.dumps(payload,sort_keys=True).encode());ledger=directory/'run.json'
    if ledger.exists():
        record=json.loads(ledger.read_text(encoding='utf-8'))
        if record['request_binding']!=binding:raise PreparationError('Cached round inputs changed; no request.')
        if record['status']!='received':raise PreparationError('Previous round failed or is incomplete; no retry.')
        checked_files(directory,record['cached_hashes'])
        return (directory/'output.txt').read_text(encoding='utf-8'),record
    if not request:raise PreparationError('Round is not cached. --request permits the two explicitly authorized stages.')
    key=os.environ.get('GEMINI_API_KEY') or dotenv_values(ROOT/'.env').get('GEMINI_API_KEY') or ''
    if not key.strip():raise PreparationError('Configure GEMINI_API_KEY in local secret configuration.')
    directory.mkdir(exist_ok=True)
    record={'schema':'syntelle-contour-loop-round/1','request_binding':binding,'status':'request_started',
            'attempts':1,'started_at':stamp(),'model':settings['model']}
    with ledger.open('x',encoding='utf-8') as handle:json.dump(record,handle,indent=2)
    write_report(directory/'request.json',payload)
    (directory/'prompt.txt').write_text(payload['input'][0]['text'],encoding='utf-8')
    try:
        raw,status,duration=(provider(payload,settings,key,directory) if provider else post_once(payload,settings,key));(directory/'response.json').write_bytes(raw)
        record.update({'http_status':status,'duration_seconds':duration,'response_sha256':digest(raw)})
        if not 200<=status<300:raise PreparationError(f'Gemini rejected the round (HTTP {status}); no retries or fallback.')
        body=json.loads(raw);text=response_text(body);(directory/'output.txt').write_text(text,encoding='utf-8')
        record.update({'status':'received','usage':body.get('usage',{}),'provider_model':body.get('model'),
                       'provider_status':body.get('status'),
                       'cached_hashes':{name:digest((directory/name).read_bytes()) for name in ('request.json','prompt.txt','response.json','output.txt')}})
        if body.get('status') not in (None,'completed'):
            raise PreparationError('Provider response is incomplete; raw output and usage retained, without retry.')
    except (ValueError,PreparationError) as exc:
        record.update({'status':'failed','error':str(exc).replace(key,'[redacted]')[:500]})
        raise PreparationError(record['error']) from None
    finally:
        record['finished_at']=stamp();write_report(ledger,record)
    return text,record


def evaluate_round(native, text, evidence, lip_evidence, settings, directory):
    contours=Contours.model_validate_json(text);write_report(directory/'contours.json',contours.model_dump())
    # Diagnostic rendering never repairs vertices or promotes a mask.
    quality=render_contours(native,contours,evidence,settings,directory,inspection_only=True)
    aperture=np.asarray(polygon_mask((evidence['crop']['width'],evidence['crop']['height']),[lip_evidence['inner_input_pixels']]))>0
    quality['outside_inner_lip_pixels']={}
    for region in contours.regions:
        mask=np.asarray(Image.open(directory/f'{region.label}-mask.png'))>=128
        quality['outside_inner_lip_pixels'][region.label]=int((mask&~aperture).sum())
    quality['lip_boundary_check']='diagnostic only; mask not clipped or corrected'
    write_report(directory/'validation.json',quality)
    return contours,quality


def comparison(output, native, qualities):
    crop=Image.open(BytesIO(native)).convert('RGB');scale=3;w,h=crop.width*scale,crop.height*scale
    sheet=Image.new('RGB',(w*3,h+135),'#17161B');draw=ImageDraw.Draw(sheet)
    frames=[crop,Image.open(output/'proposal/mask-overlay.png'),Image.open(output/'correction/mask-overlay.png')]
    titles=['Original EI','Gemini: enlarged + grid','Gemini: one visual correction']
    for index,(frame,title) in enumerate(zip(frames,titles)):
        x=index*w;draw.text((x+12,12),title,fill='white',font_size=20)
        sheet.paste(frame.resize((w,h),Image.Resampling.LANCZOS),(x,45))
        if index:
            quality=qualities[index-1]
            draw.text((x+12,h+62),quality['status'],fill='#FFB4AB' if quality['issues'] else '#CFB9FF',font_size=18)
    draw.text((12,h+96),'Blue: upper teeth   Pink: lower teeth   Orange: tongue   No manual mask input or contour repair.',fill='white',font_size=17)
    sheet.save(output/'comparison.png')


def prepare(source, output, request=False, settings_path=SETTINGS, proposal_path=PROPOSAL_PROMPT, correction_path=CORRECTION_PROMPT):
    settings=json.loads(settings_path.read_text(encoding='utf-8'))
    if (settings['maximum_requests']!=2 or settings['correction_rounds']!=1 or settings['retries']!=0
            or settings['follow_redirects'] or settings['store']
            or settings['endpoint']!='https://generativelanguage.googleapis.com/v1beta/interactions'
            or settings['model']!='gemini-3.8-flash' or settings['generation_config']['thinking_level'] not in {'low','medium','high'}):
        raise PreparationError('This experiment permits exactly the configured text-vision proposal and one correction, without retries.')
    native,evidence=original_crop(source,settings)
    proposal=proposal_path.read_text(encoding='utf-8');correction_template=correction_path.read_text(encoding='utf-8')
    binding={'input':evidence,'settings_sha256':digest(settings_path.read_bytes()),
             'proposal_prompt_sha256':digest(proposal_path.read_bytes()),'correction_template_sha256':digest(correction_path.read_bytes())}
    output.mkdir(parents=True,exist_ok=True);ledger=output/'run.json'
    if ledger.exists():
        record=json.loads(ledger.read_text(encoding='utf-8'))
        if record['binding']!=binding:raise PreparationError('Loop source/prompts/settings changed; inspect existing work.')
        if record['status'].startswith('stopped'):raise PreparationError('Previous loop stopped; completed work retained and no retry allowed.')
        checked_files(output,record['input_hashes'])
        if record['status']=='completed':
            checked_files(output,record['artifact_hashes']);print('Reusing completed two-stage experiment. No provider request.');return record
    else:
        views,lip_evidence=input_views(source,native,evidence,settings)
        record={'schema':'syntelle-gemini-contour-loop/1','version':settings['version'],'binding':binding,
                'status':'prepared','started_at':stamp(),'maximum_requests':2,'manual_mask_inputs':False,'rounds':{},
                'dependencies':{name:version(name) for name in ('Pillow','numpy','pydantic','httpx')}}
        with ledger.open('x',encoding='utf-8') as handle:json.dump(record,handle,indent=2)
        (output/'input-native.png').write_bytes(native)
        for name,data in views.items():(output/name).write_bytes(data)
        (output/'settings.json').write_bytes(settings_path.read_bytes())
        (output/'proposal-template.txt').write_bytes(proposal_path.read_bytes());(output/'correction-template.txt').write_bytes(correction_path.read_bytes())
        (output/'loop-source.py').write_bytes(Path(__file__).read_bytes())
        (output/'shared-contour-source.py').write_bytes((ROOT/'services/processor/app/workflows/contours/gemini_contour_trial.py').read_bytes())
        write_report(output/'mediapipe.json',lip_evidence)
        record['input_hashes']={p.name:digest(p.read_bytes()) for p in output.iterdir() if p.is_file() and p.name!='run.json'}
        write_report(ledger,record)
    views={name:(output/name).read_bytes() for name in ('clean.png','grid.png','mediapipe-lips.png')}
    lip_evidence=json.loads((output/'mediapipe.json').read_text(encoding='utf-8'));qualities=[];first=None
    try:
        for stage in ('proposal','correction'):
            prompt=proposal if first is None else correction_template.replace('{{PREVIOUS_CONTOURS}}',first).replace('{{VALIDATION_REPORT}}',json.dumps(qualities[0],indent=2))
            previous=None if first is None else png(Image.open(output/'proposal/mask-overlay.png').resize(Image.open(BytesIO(views['clean.png'])).size,Image.Resampling.LANCZOS))
            if previous is not None:
                feedback=output/'correction-feedback.png'
                if feedback.exists() and feedback.read_bytes()!=previous:raise PreparationError('Correction feedback changed; no provider call.')
                feedback.write_bytes(previous)
            payload=round_request(prompt,views,settings,previous)
            text,round_record=capture_round(output/stage,payload,settings,request)
            record['rounds'][stage]={'http_status':round_record['http_status'],'duration_seconds':round_record['duration_seconds'],
                                     'usage':round_record['usage'],'response_sha256':round_record['response_sha256']}
            _,quality=evaluate_round(native,text,evidence,lip_evidence,settings,output/stage)
            record['rounds'][stage]['validation']=quality;qualities.append(quality)
            write_report(ledger,record)
            if first is None:first=text
    except (PreparationError,ValueError) as exc:
        record.update({'status':'stopped','error':str(exc)[:800],'finished_at':stamp()});write_report(ledger,record)
        raise PreparationError(str(exc)) from None
    comparison(output,native,qualities)
    record.update({'status':'completed','finished_at':stamp(),'provider_requests':2,'new_viseme_generations':0,
                   'active_mask_installation':False,'manual_contour_adjustments':False,
                   'artifact_hashes':{str(p.relative_to(output)):digest(p.read_bytes()) for p in output.rglob('*') if p.is_file() and p!=ledger}})
    write_report(ledger,record)
    print('Proposal and one visual correction saved. No viseme generation or active mask installation.')
    return record


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',type=Path,default=ROOT/'.tools/visemes-01/ei')
    parser.add_argument('--output',type=Path,default=ROOT/'.tools/gemini-contour-loop-ei-01')
    parser.add_argument('--request',action='store_true')
    args=parser.parse_args()
    try:prepare(args.source,args.output,args.request)
    except PreparationError as exc:raise SystemExit(str(exc)) from None

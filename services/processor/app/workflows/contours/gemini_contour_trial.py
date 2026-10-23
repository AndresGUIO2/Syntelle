"""One cached Gemini vision request on original EI pixels; never generates images."""
import argparse
import base64
from datetime import datetime, timezone
from hashlib import sha256
from importlib.metadata import version
from io import BytesIO
import json
import os
from pathlib import Path
from time import perf_counter
from typing import Annotated, Literal

import cv2
from dotenv import dotenv_values
import httpx
import numpy as np
from PIL import Image, ImageDraw, ImageOps
from pydantic import BaseModel, ConfigDict, Field, model_validator

from app.preparation.faces import PreparationError
from app.preparation.mesh import OUTER
from app.workflows.visemes.viseme_trial import ROOT, write_report

SETTINGS = ROOT/'docs/characters/syntelle/gemini-contours-trial.json'
PROMPT = ROOT/'docs/prompts/dental-contours-v1.txt'
Number = Annotated[float, Field(ge=0, le=1000, allow_inf_nan=False)]
Polygon = Annotated[list[tuple[Number, Number]], Field(min_length=3, max_length=96)]


class Region(BaseModel):
    model_config = ConfigDict(extra='forbid')
    label: Literal['upper_teeth', 'lower_teeth', 'tongue']
    visible: bool
    polygons: Annotated[list[Polygon], Field(max_length=32)]
    notes: str

    @model_validator(mode='after')
    def check_visibility(self):
        if self.visible != bool(self.polygons):
            raise ValueError('Visible regions require contours; absent regions must have none.')
        return self


class Contours(BaseModel):
    model_config = ConfigDict(extra='forbid')
    coordinate_space: Literal['normalized-full-input-image-xy-0-1000']
    regions: Annotated[list[Region], Field(min_length=3, max_length=3)]

    @model_validator(mode='after')
    def check_labels(self):
        if {r.label for r in self.regions} != {'upper_teeth', 'lower_teeth', 'tongue'}:
            raise ValueError('Each requested region must occur exactly once.')
        return self


def stamp():
    return datetime.now(timezone.utc).isoformat()


def original_crop(folder, settings):
    run_bytes=(folder/'run.json').read_bytes()
    record=json.loads(run_bytes)
    raw=(folder/record['generation']['file']).read_bytes()
    if sha256(raw).hexdigest()!=record['generation']['raw_sha256'] or record.get('viseme')!='EI':
        raise PreparationError('Original EI cache is missing, changed or belongs to another pose.')
    landmark_bytes=(folder/'variant-landmarks.json').read_bytes()
    points=np.array(json.loads(landmark_bytes)['landmarks_px'],dtype=float)[:,:2]
    with Image.open(BytesIO(raw)) as image:
        image.load(); original=ImageOps.exif_transpose(image).convert('RGB')
    scale=np.array([original.width/record['canvas']['width'],original.height/record['canvas']['height']])
    native=points*scale
    padding=float(np.linalg.norm(native[61]-native[291]))*settings['input']['crop_padding_mouth_width_ratio']
    bounds=(*np.floor(native[OUTER].min(axis=0)-padding).astype(int),*np.ceil(native[OUTER].max(axis=0)+padding).astype(int))
    left,top,right,bottom=(max(0,int(bounds[0])),max(0,int(bounds[1])),min(original.width,int(bounds[2])),min(original.height,int(bounds[3])))
    if right<=left or bottom<=top: raise PreparationError('Invalid landmark-derived mouth crop.')
    crop=original.crop((left,top,right,bottom)); encoded=BytesIO();crop.save(encoded,format='PNG')
    to_original=np.array([[1,0,left],[0,1,top],[0,0,1]],dtype=float)
    to_canvas=np.diag([1/scale[0],1/scale[1],1])@to_original
    alignment=np.vstack([record['metrics']['transform'],[0,0,1]])
    evidence={'pose':'EI','source_sha256':record['source_sha256'],'raw_sha256':sha256(raw).hexdigest(),
              'run_sha256':sha256(run_bytes).hexdigest(),'landmarks_sha256':sha256(landmark_bytes).hexdigest(),
              'original_size':list(original.size),'crop':{'x':left,'y':top,'width':crop.width,'height':crop.height},
              'input_sha256':sha256(encoded.getvalue()).hexdigest(),'mime_type':'image/png','resized':False,
              'input_to_original':to_original.tolist(),'input_to_aligned_character':(alignment@to_canvas).tolist()}
    return encoded.getvalue(),evidence


def build_request(image, prompt, settings):
    # Keep the provider schema inline and simple. Bounds, tuple lengths,
    # visibility and topology are still validated strictly by Contours locally.
    region_schema={'type':'object','properties':{
        'label':{'type':'string','enum':['upper_teeth','lower_teeth','tongue']},
        'visible':{'type':'boolean'},
        'polygons':{'type':'array','items':{'type':'array','items':{'type':'array','items':{'type':'number'}}}},
        'notes':{'type':'string'}},'required':['label','visible','polygons','notes']}
    schema={'type':'object','properties':{'coordinate_space':{'type':'string'},
            'regions':{'type':'array','items':region_schema}},'required':['coordinate_space','regions']}
    payload={'model':settings['model'],'input':[{'type':'text','text':prompt},
             {'type':'image','mime_type':'image/png','data':base64.b64encode(image).decode('ascii')}],
             'response_format':{'type':'text','mime_type':'application/json','schema':schema},
             'generation_config':settings['generation_config'],'store':False}
    return payload


def post_once(payload, settings, key):
    started=perf_counter()
    try:
        with httpx.Client(timeout=settings['timeout_seconds'],transport=httpx.HTTPTransport(retries=0),follow_redirects=False) as client:
            response=client.post(settings['endpoint'],headers={'x-goog-api-key':key},json=payload)
    except httpx.RequestError:
        raise PreparationError('Gemini connection failed or timed out; no retry was made.') from None
    # A provider error may echo a credential. Never save it in evidence.
    body=response.content.replace(key.encode(),b'[redacted]')
    return body,response.status_code,round(perf_counter()-started,3)


def response_text(body):
    if isinstance(body.get('output_text'),str):return body['output_text']
    parts=[item['text'] for step in body.get('steps',[]) if step.get('type')=='model_output'
           for item in step.get('content',[]) if item.get('type')=='text' and item.get('text')]
    if not parts:
        parts=[item['text'] for item in body.get('outputs',[]) if item.get('type')=='text' and item.get('text')]
    if not parts: raise PreparationError('Gemini returned no text contours; raw response retained, no retry.')
    return ''.join(parts)


def polygon_is_simple(points):
    def cross(a,b,c):
        v,w=b-a,c-a
        return float(v[0]*w[1]-v[1]*w[0])
    n=len(points)
    for i in range(n):
        a,b=points[i],points[(i+1)%n]
        if np.allclose(a,b):return False
        for j in range(i+1,n):
            if j==i+1 or (i==0 and j==n-1):continue
            c,d=points[j],points[(j+1)%n]
            if cross(a,b,c)*cross(a,b,d)<0 and cross(c,d,a)*cross(c,d,b)<0:return False
    return abs(cv2.contourArea(points.astype(np.float32)))>0


def render_contours(image, contours, evidence, settings, output, *, inspection_only=False):
    crop=Image.open(BytesIO(image)).convert('RGB');width,height=crop.size
    scale=settings['render']['supersampling']; masks={};coordinates={};issues=[]
    for region in contours.regions:
        mask=Image.new('L',(width*scale,height*scale));draw=ImageDraw.Draw(mask)
        coords={'normalized':region.polygons,'input_pixels':[],'original_pixels':[],'aligned_character_pixels':[]}
        for polygon in region.polygons:
            normalized=np.array(polygon,dtype=float)
            if not polygon_is_simple(normalized):
                issue=f'{region.label}: degenerate or crossing contour; no local repair or retry.'
                if not inspection_only:raise PreparationError(issue)
                issues.append(issue)
            pixels=normalized*[width/1000,height/1000]
            draw.polygon([tuple(p) for p in pixels*scale],fill=255)
            coords['input_pixels'].append(pixels.tolist())
            homogeneous=np.column_stack([pixels,np.ones(len(pixels))])
            for target,matrix in [('original_pixels','input_to_original'),('aligned_character_pixels','input_to_aligned_character')]:
                coords[target].append((homogeneous@np.array(evidence[matrix]).T)[:,:2].tolist())
        masks[region.label]=mask.resize((width,height),Image.Resampling.LANCZOS)
        coordinates[region.label]=coords
    names=list(masks)
    for i,a in enumerate(names):
        for b in names[i+1:]:
            if ((np.asarray(masks[a])>=128)&(np.asarray(masks[b])>=128)).any():
                issue=f'{a}/{b}: predicted regions overlap; no local repair or retry.'
                if not inspection_only:raise PreparationError(issue)
                issues.append(issue)
    original=np.asarray(crop);overlay=original.astype(float);union=np.zeros((height,width),dtype=bool)
    for label,mask in masks.items():
        mask.save(output/f'{label}-mask.png')
        alpha=np.asarray(mask,dtype=float)[...,None]/255*settings['render']['overlay_opacity']
        union|=np.asarray(mask)>0
        overlay=overlay*(1-alpha)+np.array(settings['render']['colors'][label])*alpha
    overlay=Image.fromarray(np.rint(overlay).astype(np.uint8));overlay.save(output/'mask-overlay.png')
    assert np.array_equal(np.asarray(overlay)[~union],original[~union])
    zoom=settings['render']['preview_scale'];w,h=width*zoom,height*zoom
    sheet=Image.new('RGB',(w*2,h+140),'#17161B');draw=ImageDraw.Draw(sheet)
    draw.text((16,12),'Original EI crop - unchanged pixels',fill='white',font_size=20)
    title='Gemini proposal - REJECTED geometry' if issues else 'Gemini contour proposal - no manual adjustment'
    draw.text((w+16,12),title,fill='#FFB4AB' if issues else 'white',font_size=20)
    sheet.paste(crop.resize((w,h),Image.Resampling.LANCZOS),(0,45))
    sheet.paste(overlay.resize((w,h),Image.Resampling.LANCZOS),(w,45))
    for i,region in enumerate(contours.regions):
        draw.text((16+i*(w*2//3),h+60),f'{region.label}: {"visible" if region.visible else "not visible"}',fill=tuple(settings['render']['colors'][region.label]),font_size=18)
    if issues:draw.text((16,h+100),'Crossing/overlapping contours: diagnostic fill only; not a usable dental mask.',fill='#FFB4AB',font_size=18)
    sheet.save(output/'review.png')
    write_report(output/'coordinates.json',{'space':contours.coordinate_space,'transforms':evidence,'regions':coordinates})
    return {'mask_pixels':{label:int((np.asarray(mask)>=128).sum()) for label,mask in masks.items()},
            'changed_pixels_outside_masks':0,'status':'rejected_geometry' if issues else 'unreviewed_proposal',
            'issues':issues,'manual_adjustments':False}


def inspect_cached(output):
    """Visualize rejected coordinates verbatim, offline; never bypass acceptance."""
    record=json.loads((output/'run.json').read_text(encoding='utf-8'))
    if record['status']!='invalid_contours':raise PreparationError('Offline rejected inspection requires a cached invalid contour response.')
    for name,digest in record['cached_hashes'].items():
        if sha256((output/name).read_bytes()).hexdigest()!=digest:
            raise PreparationError('Cached evidence changed; inspection refused.')
    settings=json.loads((output/'settings.json').read_text(encoding='utf-8'))
    contours=Contours.model_validate_json((output/'output.txt').read_text(encoding='utf-8'))
    target=output/'inspection';target.mkdir(exist_ok=True)
    quality=render_contours((output/'input.png').read_bytes(),contours,record['binding']['input'],settings,target,inspection_only=True)
    (target/'render-source.py').write_bytes(Path(__file__).read_bytes())
    write_report(target/'report.json',{'schema':'syntelle-rejected-contour-inspection/1',
        'created_at':stamp(),'request_status':record['status'],'binding':record['binding'],
        'response_sha256':record['response_sha256'],'quality':quality,'new_provider_requests':0,
        'artifact_hashes':{path.name:sha256(path.read_bytes()).hexdigest() for path in target.iterdir() if path.is_file() and path.name!='report.json'}})
    print('Rejected contour diagnostic rendered verbatim. No request, geometry repair or active mask installation.')
    return quality


def prepare(folder, output, request=False, settings_path=SETTINGS, prompt_path=PROMPT):
    settings_bytes=settings_path.read_bytes();settings=json.loads(settings_bytes)
    if settings['maximum_requests']!=1 or settings['retries']!=0 or settings['follow_redirects'] or settings['store']:
        raise PreparationError('This trial permits one non-stored vision request with no retries or redirects.')
    if settings['endpoint']!='https://generativelanguage.googleapis.com/v1beta/interactions':
        raise PreparationError('Unexpected provider destination.')
    if 'image' in settings['model'] or 'banana' in settings['model']:
        raise PreparationError('Use a vision/text model, not an image-generation model.')
    if settings['model']=='gemini-3.8-flash' and settings['generation_config'].get('thinking_level') not in {'low','medium','high'}:
        raise PreparationError('Gemini 3.8 Flash requires low, medium or high thinking; minimal is unsupported.')
    image,evidence=original_crop(folder,settings)
    prompt_bytes=prompt_path.read_bytes();prompt=prompt_bytes.decode('utf-8')
    binding={'input':evidence,'prompt_sha256':sha256(prompt_bytes).hexdigest(),'settings_sha256':sha256(settings_bytes).hexdigest()}
    output.mkdir(parents=True,exist_ok=True);ledger=output/'run.json'
    if ledger.exists():
        record=json.loads(ledger.read_text(encoding='utf-8'))
        if record['binding']!=binding:raise PreparationError('Cached contour inputs/settings changed; no request or overwrite.')
        if record['status'] not in {'received','ready_for_visual_review'}:raise PreparationError('Previous request incomplete or failed; no retry is allowed here.')
        for name,digest in record['cached_hashes'].items():
            if not (output/name).is_file() or sha256((output/name).read_bytes()).hexdigest()!=digest:
                raise PreparationError('Cached contour evidence changed or is missing; no request.')
        text=(output/'output.txt').read_text(encoding='utf-8')
        print('Reusing cached Gemini contours. No provider request.')
    else:
        if not request:raise PreparationError('No cached contours. --request makes one explicitly authorized vision request.')
        key=os.environ.get('GEMINI_API_KEY') or dotenv_values(ROOT/'.env').get('GEMINI_API_KEY') or ''
        if not key.strip():raise PreparationError('Configure GEMINI_API_KEY in local secret configuration.')
        payload=build_request(image,prompt,settings)
        record={'schema':'syntelle-gemini-contour-trial/1','version':settings['version'],'binding':binding,
                'status':'request_started','attempts':1,'started_at':stamp(),'model':settings['model'],
                'source_code_sha256':sha256(Path(__file__).read_bytes()).hexdigest(),
                'dependencies':{name:version(name) for name in ('Pillow','numpy','pydantic','httpx')}}
        # Exclusive creation bounds paid work even if this command is launched twice.
        with ledger.open('x',encoding='utf-8') as handle:json.dump(record,handle,indent=2)
        (output/'input.png').write_bytes(image);(output/'prompt.txt').write_bytes(prompt_bytes)
        (output/'settings.json').write_bytes(settings_bytes);write_report(output/'request.json',payload)
        (output/'preparation-source.py').write_bytes(Path(__file__).read_bytes())
        try:
            raw,status,duration=post_once(payload,settings,key)
            (output/'response.json').write_bytes(raw)
            record.update({'http_status':status,'duration_seconds':duration,'response_sha256':sha256(raw).hexdigest()})
            if status<200 or status>=300:raise PreparationError(f'Gemini rejected contour request (HTTP {status}); response retained, no retry.')
            body=json.loads(raw);text=response_text(body)
            (output/'output.txt').write_text(text,encoding='utf-8')
            record.update({'status':'received','usage':body.get('usage',{}),'interaction_id':body.get('id'),
                           'cached_hashes':{name:sha256((output/name).read_bytes()).hexdigest() for name in ('input.png','prompt.txt','settings.json','request.json','response.json','output.txt','preparation-source.py')}})
        except (PreparationError,ValueError) as exc:
            record.update({'status':'failed','error':str(exc).replace(key,'[redacted]')[:500]})
            raise PreparationError(record['error']) from None
        finally:
            record['finished_at']=stamp();write_report(ledger,record)
    try:
        contours=Contours.model_validate_json(text)
        write_report(output/'contours.json',contours.model_dump())
        quality=render_contours(image,contours,evidence,settings,output)
    except (ValueError,PreparationError) as exc:
        record.update({'status':'invalid_contours','validation_error':str(exc)[:800]});write_report(ledger,record)
        raise PreparationError('Contour validation failed; paid response retained without repair or retry.') from None
    record.update({'status':'ready_for_visual_review','quality':quality,'rendered_at':stamp(),
                   'render_source_sha256':sha256(Path(__file__).read_bytes()).hexdigest(),
                   'artifact_hashes':{str(path.relative_to(output)):sha256(path.read_bytes()).hexdigest() for path in output.iterdir() if path.is_file() and path.name!='run.json'}})
    write_report(ledger,record)
    print('EI contour overlay prepared. One vision request maximum; visemes and active trial unchanged.')
    return record


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',type=Path,default=ROOT/'.tools/visemes-01/ei')
    parser.add_argument('--output',type=Path,default=ROOT/'.tools/gemini-contours-ei-03')
    parser.add_argument('--request',action='store_true')
    parser.add_argument('--inspect-invalid',action='store_true',help='Offline visualization only; invalid masks stay rejected.')
    args=parser.parse_args()
    if args.request and args.inspect_invalid:parser.error('Rejected inspection cannot request provider work.')
    try:
        if args.inspect_invalid:inspect_cached(args.output)
        else:prepare(args.source,args.output,args.request)
    except PreparationError as exc:raise SystemExit(str(exc)) from None

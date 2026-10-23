"""Two explicitly authorized image repairs, with immutable evidence and no retries."""
import argparse
import base64
from hashlib import sha256
from io import BytesIO
import json
from pathlib import Path
import shutil
from time import perf_counter

import cv2
import httpx
import numpy as np
from dotenv import dotenv_values
from PIL import Image

from app.workflows.dentition.complete_dentition import chroma_matte, warp_rgba
from app.workflows.contours.gemini_contour_trial import stamp
from app.preparation.faces import FaceDetector, PreparationError
from app.preparation.gemini import build_prompt
from app.preparation.mouth import align_variant, export_lossless_pair
from app.workflows.visemes.viseme_trial import ROOT, read_generated, save_provider_image, write_report
from app.workflows.oral.oral_semantics import load_inputs

VERSION = 'oral-repair/1'


def digest(data): return sha256(data).hexdigest()


def prepare(manifest, fragments, output):
    spec, _, images, _, _ = load_inputs(manifest)
    output.mkdir(parents=True, exist_ok=True)
    binding = {'manifest': digest(manifest.read_bytes()), 'fragments': digest(fragments.read_bytes())}
    ledger = output/'inputs.json'
    if ledger.exists():
        saved=json.loads(ledger.read_text())
        if saved['binding'] != binding: raise PreparationError('Repair inputs changed; use preserved evidence.')
        for name, expected in saved['files'].items():
            if digest((output/name).read_bytes()) != expected: raise PreparationError('Repair evidence changed.')
        return saved
    original=(manifest.parent/spec['original']).resolve()
    shutil.copyfile(original,output/'portrait.jpg')
    images['EI'].save(output/'context.png')
    shutil.copyfile(fragments,output/'fragments.png')
    rgba=np.asarray(Image.open(fragments).convert('RGBA')); ys,xs=np.nonzero(rgba[...,3]>=128)
    scale=min(4.,760/(xs.max()-xs.min()+1))
    matrix=np.array([[scale,0,512-scale*(xs.min()+xs.max())/2],[0,scale,390-scale*ys.min()]],float)
    placed=warp_rgba(rgba,matrix,(1024,1024))
    canvas=Image.new('RGB',(1024,1024),(0,255,0)); canvas.paste(Image.fromarray(placed),(0,0),Image.fromarray(placed[...,3]))
    canvas.save(output/'canvas.png'); Image.fromarray(placed[...,3]).save(output/'protected.png')
    lower=(ROOT/'docs/prompts/lower-dentition-continuity-v1.txt').read_text()
    lower+='\nAnatomical orientation: this is the LOWER arch viewed from the front. Its incisal biting edge is at the TOP of each crown, with crowns extending DOWN toward the roots and gums. Do not flip upper teeth upside down or paint teeth pointing down as a second layer. Extend each existing tooth continuously, without a horizontal ledge. Use exactly the supplied camera scale and registered location.\n'
    (output/'lower-prompt.txt').write_text(lower)
    fv=(ROOT/'docs/prompts/viseme-fv-contact-v2.txt').read_text().replace('{{ORIGINAL_FV_PROMPT}}',build_prompt(ROOT/'docs/prompts/nano-banana-source.txt','FV'))
    (output/'fv-prompt.txt').write_text(fv)
    saved={'version':VERSION,'binding':binding,'maximum_requests':2,'original_sha256':spec['original_sha256'],
           'transform':matrix.tolist(),'fragment_size':[rgba.shape[1],rgba.shape[0]],
           'files':{p.name:digest(p.read_bytes()) for p in output.iterdir() if p.is_file()}}
    write_report(ledger,saved); return saved


def request(output, kind):
    if kind not in ('lower','fv'): raise PreparationError('Unknown repair identifier.')
    stage=output/kind; ledger=stage/'run.json'
    if ledger.exists():
        run=json.loads(ledger.read_text())
        if run['status']=='received':
            if digest((stage/run['file']).read_bytes())!=run['raw_sha256']:
                raise PreparationError('Cached repair output changed.')
            return run
        raise PreparationError('A prior request was attempted; automatic retry is prohibited.')
    if any(json.loads(p.read_text()).get('status')!='received' for p in output.glob('*/run.json')):
        raise PreparationError('An earlier provider attempt failed or was interrupted; the repair batch is stopped.')
    if len(list(output.glob('*/run.json')))>=2: raise PreparationError('Repair budget exhausted.')
    key=dotenv_values(ROOT/'.env').get('GEMINI_API_KEY','')
    if not key: raise PreparationError('Configure the local Gemini key.')
    names=['canvas.png','context.png','protected.png','portrait.jpg'] if kind=='lower' else ['portrait.jpg','context.png']
    prompt=(output/f'{kind}-prompt.txt').read_text()
    payload={'model':'gemini-nano-banana-2.1','input':[{'type':'text','text':prompt}],
             'response_format':{'type':'image','mime_type':'image/jpeg','aspect_ratio':'1:1' if kind=='lower' else '3:4','image_size':'1K'},'store':False}
    for name in names:
        payload['input'].append({'type':'image','mime_type':'image/jpeg' if name.endswith('.jpg') else 'image/png',
                                 'data':base64.b64encode((output/name).read_bytes()).decode()})
    stage.mkdir(); shutil.copyfile(Path(__file__),stage/'source.py')
    (stage/'prompt.txt').write_text(prompt); write_report(stage/'request.json',payload)
    run={'status':'request_started','version':VERSION,'started_at':stamp(),'attempts':1,'retries':0,
         'settings':{k:v for k,v in payload.items() if k!='input'},'original_sha256':digest((output/'portrait.jpg').read_bytes()),
         'inputs':{name:digest((output/name).read_bytes()) for name in names},'prompt_sha256':digest(prompt.encode())}
    write_report(ledger,run); started=perf_counter()
    try:
        with httpx.Client(timeout=240,transport=httpx.HTTPTransport(retries=0),follow_redirects=False) as client:
            response=client.post('https://generativelanguage.googleapis.com/v1beta/interactions',headers={'x-goog-api-key':key},json=payload)
        raw=response.content.replace(key.encode(),b'[redacted]'); (stage/'response.json').write_bytes(raw)
        run['http_status']=response.status_code
        if not response.is_success: raise PreparationError(f'Provider HTTP {response.status_code}; no retry.')
        body=json.loads(raw); run['usage']=body.get('usage',{})
        parts=[body['output_image']] if body.get('output_image') else [p for s in body.get('steps',[]) for p in s.get('content',[]) if p.get('type')=='image' and p.get('data')]
        if len(parts)!=1: raise PreparationError('Expected one image; evidence retained.')
        data=base64.b64decode(parts[0]['data'],validate=True)
        file=save_provider_image(data,stage,'FV' if kind=='fv' else 'lower')
        run.update(status='received',file=file.name,raw_sha256=digest(data))
    except (httpx.RequestError,ValueError,PreparationError) as error:
        run.update(status='failed',error=str(error).replace(key,'[redacted]')[:500]); raise PreparationError(run['error']) from None
    finally:
        run.update(finished_at=stamp(),duration_seconds=round(perf_counter()-started,3)); write_report(ledger,run)
    return run


def align_fv(output):
    stage=output/'fv'; run=json.loads((stage/'run.json').read_text()); base=Image.open(output/'portrait.jpg').convert('RGB')
    raw=(stage/run['file']).read_bytes(); generated=read_generated(raw,base.size)
    with FaceDetector(ROOT/'.tools/models/face_landmarker.task') as detector:
        a,b=detector.detect(base),detector.detect(generated)
    _,_,metrics=align_variant(base,generated,a,b)
    write_report(stage/'base-landmarks.json',a.as_dict());write_report(stage/'variant-landmarks.json',b.as_dict())
    run.update(viseme='FV',source_sha256=digest((output/'portrait.jpg').read_bytes()),canvas={'width':base.width,'height':base.height},metrics=metrics,
               generation={'file':run['file'],'raw_sha256':digest(raw),'request_settings':{'input_sha256':digest((output/'portrait.jpg').read_bytes())}})
    write_report(stage/'run.json',run)


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--manifest',type=Path,required=True);parser.add_argument('--fragments',type=Path,required=True)
    parser.add_argument('--output',type=Path,required=True);parser.add_argument('--request',choices=['lower','fv'])
    args=parser.parse_args(); prepare(args.manifest.resolve(),args.fragments.resolve(),args.output.resolve())
    if args.request:
        print(json.dumps(request(args.output,args.request)))
        if args.request=='fv': align_fv(args.output)

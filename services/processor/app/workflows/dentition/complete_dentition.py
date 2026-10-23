"""Complete the actual selected EI fragments; two recorded image requests maximum."""
import argparse
import base64
from hashlib import sha256
from io import BytesIO
import json
import os
from pathlib import Path
from time import perf_counter

import cv2
import httpx
import numpy as np
from PIL import Image,ImageDraw
from dotenv import dotenv_values

from app.workflows.visemes.viseme_trial import ROOT,write_report
from app.workflows.contours.gemini_contour_trial import stamp
from app.preparation.faces import PreparationError
from app.preparation.mouth import export_lossless_pair

VERSION='selected-dentition-completion/2'
PROMPT=ROOT/'docs/prompts/dentition-completion-v2.txt'
SOURCE=ROOT/'.tools/dental-spacing-03/package'
OUTPUT=ROOT/'.tools/dentition-completion-04'


def digest(data):return sha256(data).hexdigest()


def png(image):
    f=BytesIO();image.save(f,format='PNG');return f.getvalue()


def prepare_canvas(source,output):
    output.mkdir(parents=True,exist_ok=True)
    if (output/'inputs.json').exists():
        record=json.loads((output/'inputs.json').read_text())
        for name,h in record['files'].items():
            if digest((output/name).read_bytes())!=h:raise PreparationError('Completion evidence changed.')
        for name,h in record['sources'].items():
            if digest(Path(name).read_bytes())!=h:raise PreparationError('Selected dental source changed.')
        return record
    canvas=Image.new('RGB',(1024,1024),(0,255,0));guide=Image.new('L',canvas.size)
    transforms={};sources={}
    for row,edge in [('upper',350),('lower',680)]:
        path=source/f'{row}-teeth.png';image=Image.open(path).convert('RGBA');sources[str(path.resolve())]=digest(path.read_bytes())
        alpha=np.asarray(image)[...,3];ys,xs=np.nonzero(alpha>=128)
        factor=4.;tx=512-factor*(xs.min()+xs.max())/2
        ty=edge-factor*(ys.max() if row=='upper' else ys.min())
        matrix=np.array([[factor,0,tx],[0,factor,ty]],dtype=float)
        rgba=cv2.warpAffine(np.asarray(image),matrix,(1024,1024),flags=cv2.INTER_LINEAR)
        placed=Image.fromarray(rgba);canvas.paste(placed,(0,0),placed.getchannel('A'))
        guide=Image.fromarray(np.maximum(np.asarray(guide),rgba[...,3]))
        transforms[row]={'character_to_atlas':matrix.tolist(),'atlas_to_character':cv2.invertAffineTransform(matrix).tolist(),
                         'source_bounds':[int(xs.min()),int(ys.min()),int(xs.max()+1),int(ys.max()+1)]}
    (output/'canvas.png').write_bytes(png(canvas));(output/'protected.png').write_bytes(png(guide))
    for src,name in [(source/'dental-source.png','context.png'),(ROOT.parent/'Colors/Syntelle.jpg','portrait.jpg')]:
        (output/name).write_bytes(src.read_bytes());sources[str(src.resolve())]=digest(src.read_bytes())
    (output/'prompt.txt').write_bytes(PROMPT.read_bytes())
    record={'version':VERSION,'source_package':str(source.resolve()),'size':[1024,1024],
            'transforms':transforms,'sources':sources,'files':{name:digest((output/name).read_bytes()) for name in ('canvas.png','protected.png','context.png','portrait.jpg','prompt.txt')}}
    write_report(output/'inputs.json',record);return record


def request(output,correction=False):
    record=json.loads((output/'inputs.json').read_text());stage=output/('correction' if correction else 'initial')
    if (stage/'run.json').exists():raise PreparationError('This request was already attempted. Replay locally; no retry.')
    if correction and not (output/'initial/response.json').exists():raise PreparationError('No initial generation to correct.')
    if len(list(output.glob('*/run.json')))>=2:raise PreparationError('The two-request budget is exhausted.')
    key=os.environ.get('GEMINI_API_KEY') or dotenv_values(ROOT/'.env').get('GEMINI_API_KEY') or ''
    if not key:raise PreparationError('Configure the existing local Gemini key.')
    prompt=(output/'prompt.txt').read_text()
    if correction:
        feedback=(output/'correction-feedback.txt').read_text(encoding='utf-8')
        prompt+='\nVisual correction of the previous result, using the ORIGINAL fragments as authority:\n'+feedback
    images=[('canvas.png','image/png'),('context.png','image/png'),('protected.png','image/png'),('portrait.jpg','image/jpeg')]
    prior=json.loads((output/'initial/run.json').read_text()) if correction else None
    if prior and prior.get('image_file'):images.append(('initial/'+prior['image_file'],prior['provider_mime_type']))
    mime='image/jpeg' if prior and prior.get('http_status')==400 else 'image/png'
    payload={'model':'gemini-nano-banana-2.1','input':[{'type':'text','text':prompt}]+[
        {'type':'image','mime_type':mime,'data':base64.b64encode((output/name).read_bytes()).decode()} for name,mime in images],
        'response_format':{'type':'image','mime_type':mime,'aspect_ratio':'1:1','image_size':'1K'},'store':False}
    stage.mkdir();run={'version':VERSION,'status':'request_started','attempts':1,'started_at':stamp(),
                       'settings':{k:v for k,v in payload.items() if k!='input'},'input_binding':digest(json.dumps(record,sort_keys=True).encode()),
                       'prompt_sha256':digest(prompt.encode()),'retries':0,'timeout_seconds':180}
    write_report(stage/'run.json',run);write_report(stage/'request.json',payload)
    (stage/'prompt.txt').write_text(prompt,encoding='utf-8');(stage/'source.py').write_bytes(Path(__file__).read_bytes())
    started=perf_counter()
    try:
        with httpx.Client(timeout=180,follow_redirects=False,transport=httpx.HTTPTransport(retries=0)) as client:
            response=client.post('https://generativelanguage.googleapis.com/v1beta/interactions',headers={'x-goog-api-key':key},json=payload)
        raw=response.content.replace(key.encode(),b'[redacted]');(stage/'response.json').write_bytes(raw)
        run.update({'http_status':response.status_code,'response_sha256':digest(raw),'duration_seconds':round(perf_counter()-started,3)})
        if not response.is_success:raise PreparationError(f'Provider error HTTP {response.status_code}; no retry.')
        body=json.loads(raw);run['usage']=body.get('usage',{});run['provider_status']=body.get('status')
        if body.get('status') not in (None,'completed'):raise PreparationError('Provider result incomplete; evidence retained.')
        image=body.get('output_image')
        if not image:
            found=[part for step in body.get('steps',[]) for part in step.get('content',[]) if part.get('type')=='image' and part.get('data')]
            if len(found)!=1:raise PreparationError('Expected one completed image.')
            image=found[0]
        data=base64.b64decode(image['data'],validate=True)
        with Image.open(BytesIO(data)) as im:
            if im.size!=(1024,1024):raise PreparationError('Provider changed completion canvas dimensions.')
            run['actual_format']=im.format
        filename='generated.png' if run['actual_format']=='PNG' else 'generated.jpg'
        (stage/filename).write_bytes(data)
        run.update({'status':'received','image_sha256':digest(data),'provider_mime_type':image.get('mime_type'),
                    'image_file':filename,'format_deviation':('Provider rejected PNG; JPEG original retained, working exports lossless.' if mime=='image/jpeg' else None)})
    except (httpx.RequestError,ValueError,PreparationError) as exc:
        run.update({'status':'failed','error':str(exc).replace(key,'[redacted]')[:500]});raise PreparationError(run['error']) from None
    finally:
        run['finished_at']=stamp();write_report(stage/'run.json',run)
    return run


def chroma_matte(rgb):
    """Key connected background/apertures; never infer opacity from enamel brightness."""
    a=np.asarray(rgb.convert('RGB')).astype(float);r,g,b=a.transpose(2,0,1)
    chroma=(g>70)&(g>r*1.45)&(g>b*1.45)
    # Include enclosed key-colored apertures between teeth, not only frame-connected backdrop.
    background=chroma;foreground=(~background).astype(np.uint8)
    n,l,stats,_=cv2.connectedComponentsWithStats(foreground,8)
    keep=[i for i in range(1,n) if stats[i,cv2.CC_STAT_AREA]>500]
    foreground=np.isin(l,keep).astype(np.uint8)
    distance=cv2.distanceTransform(foreground,cv2.DIST_L2,cv2.DIST_MASK_PRECISE)
    alpha=np.clip(distance,0,1)
    # Remove chroma spill on the edge; never use intensity to set opacity.
    edge=(distance<=2)&(foreground>0)
    a[...,1][edge]=np.minimum(a[...,1][edge],np.maximum(a[...,0],a[...,2])[edge]*1.1)
    a[foreground==0]=0
    return np.dstack((np.clip(a,0,255),alpha*255)).astype(np.uint8)


def warp_rgba(raw, matrix, size):
    """Filter premultiplied material to prevent matte RGB leaking into fringes."""
    pixels=raw.astype(np.float32)/255;pixels[...,:3]*=pixels[...,3,None]
    warped=cv2.warpAffine(pixels,matrix,size,flags=cv2.INTER_LINEAR)
    warped[...,:3]/=np.maximum(warped[...,3,None],1e-6)
    return np.rint(np.clip(warped,0,1)*255).astype(np.uint8)


def replay(output,stage='initial'):
    record=json.loads((output/'inputs.json').read_text());folder=output/stage
    run=json.loads((folder/'run.json').read_text())
    raw=(folder/run['image_file']).read_bytes()
    if run['status']!='received' or digest(raw)!=run['image_sha256']:raise PreparationError('Completion output is missing or changed.')
    image=Image.open(BytesIO(raw)).convert('RGB');matte=chroma_matte(image)
    target=folder/'replay';target.mkdir(exist_ok=True);report={'version':VERSION,'stage':stage,'rows':{}}
    source=Path(record['source_package']);preview=Image.new('RGB',(1024,1024),'#242127')
    for row,y0,y1 in [('upper',0,512),('lower',512,1024)]:
        isolated=matte.copy();isolated[:y0]=0;isolated[y1:]=0
        original=Image.open(source/f'{row}-teeth.png').convert('RGBA');a=np.asarray(original);h,w=a.shape[:2]
        transform=np.asarray(record['transforms'][row]['character_to_atlas']);inv=np.asarray(record['transforms'][row]['atlas_to_character'])
        # Match incisal silhouettes, not brightness (the model changes illumination).
        template=cv2.warpAffine(a,transform,(1024,1024),flags=cv2.INTER_LINEAR)
        support=(template[...,3]>=240).astype(np.uint8)
        cols=np.flatnonzero(support.any(0));central=cols[(cols>np.quantile(cols,.22))&(cols<np.quantile(cols,.78))]
        def edge_profile(mask,x):
            indices=np.flatnonzero(mask[:,x]);return (indices.max() if row=='upper' else indices.min()) if len(indices) else np.nan
        target_edges=np.array([edge_profile(support,x) for x in central])
        candidates=[]
        for shift in range(-32,33):
            generated_edges=np.array([edge_profile(isolated[...,3]>=128,x+shift) for x in central])
            delta=generated_edges-target_edges;dy=int(round(np.nanmedian(delta)))
            error=float(np.nanmedian(np.abs(delta-dy)))+abs(shift)*0.005
            candidates.append((error,shift,dy))
        minimum,dx,dy=min(candidates)
        if abs(dy)>180 or minimum>10:raise PreparationError('Completion cannot be registered to supplied incisal edges.')
        shifted=warp_rgba(isolated,np.array([[1,0,-dx],[0,1,-dy]],float),(1024,1024))
        pixels=warp_rgba(shifted,inv,(w,h))
        protected=a[...,3]>=128
        overlap=protected&(pixels[...,3]>=128)
        if overlap.sum()<protected.sum()*.8:raise PreparationError('Completion lost the original tooth anchors.')
        rgb=a[...,:3].astype(float)
        enamel=overlap&(rgb[...,1]/(rgb[...,0]+1)>.70)&(rgb[...,2]/(rgb[...,0]+1)>.60)&(rgb[...,1]>60)
        if enamel.sum()<30:raise PreparationError('Insufficient original enamel for completion illumination matching.')
        ratios=np.clip((rgb+1)/(pixels[...,:3].astype(float)+1),.5,2)
        gain=np.median(ratios[enamel],axis=0)
        distance,nearest=cv2.distanceTransformWithLabels((~enamel).astype(np.uint8),cv2.DIST_L2,5,labelType=cv2.DIST_LABEL_PIXEL)
        lookup=np.vstack((gain,ratios[enamel]));field=lookup[np.minimum(nearest,len(lookup)-1)]
        field=cv2.GaussianBlur(field,(0,0),2)
        blend=np.clip(distance/24,0,1)[...,None];field=field*(1-blend)+gain*blend
        pixels[...,:3]=np.rint(np.clip(pixels[...,:3]*field,0,255)).astype(np.uint8)
        # Complete behind the incisal edges, not through/below their silhouette.
        yy,xx=np.nonzero(protected);columns=np.flatnonzero(protected.any(0))
        edges=[np.flatnonzero(protected[:,col]).max() if row=='upper' else np.flatnonzero(protected[:,col]).min() for col in columns]
        edge=np.interp(np.arange(w),columns,edges)
        if row=='upper':pixels[...,3][np.arange(h)[:,None]>edge[None,:]+1]=0
        else:pixels[...,3][np.arange(h)[:,None]<edge[None,:]-1]=0
        pixels[protected,:3]=a[protected,:3];pixels[protected,3]=255
        if (pixels[...,3]>0).sum()<protected.sum()*1.5:raise PreparationError('Generated completion has insufficient hidden coverage.')
        export_lossless_pair(Image.fromarray(pixels),target,row)
        Image.fromarray((protected*255).astype(np.uint8)).save(target/f'{row}-protected.png')
        completed=(pixels[...,3]>0)&~protected;Image.fromarray((completed*255).astype(np.uint8)).save(target/f'{row}-completed.png')
        report['rows'][row]={'registration_method':'incisal-edge-profile/2','translation_atlas_px':[dx,dy],'registration_error':minimum,
                             'completion_color_gain':gain.tolist(),'protected_pixels':int(protected.sum()),
                             'completed_pixels':int(completed.sum()),'known_RGB_restored':bool(np.array_equal(pixels[protected,:3],a[protected,:3]))}
        mapped=cv2.warpAffine(pixels,transform,(1024,1024),flags=cv2.INTER_LINEAR)
        preview.paste(Image.fromarray(mapped),(0,0),Image.fromarray(mapped[...,3]))
    preview.save(target/'completed-atlas.png');write_report(target/'replay.json',report)
    print(json.dumps(report,indent=2));return report


if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',type=Path,default=SOURCE);parser.add_argument('--output',type=Path,default=OUTPUT)
    parser.add_argument('--request',action='store_true');parser.add_argument('--correct',action='store_true');parser.add_argument('--replay',action='store_true')
    args=parser.parse_args();prepare_canvas(args.source,args.output)
    if args.request:print(json.dumps(request(args.output,args.correct),indent=2))
    if args.replay:replay(args.output,'correction' if args.correct else 'initial')
